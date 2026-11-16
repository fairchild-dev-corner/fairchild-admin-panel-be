package services

import (
	"bytes"
	"encoding/csv"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	models "fairchild-admin-panel-be/internal/models/voters"

	"github.com/xuri/excelize/v2"
)

// headerScanRows is how far down a sheet the header row may be (a title or
// note above it is common).
const headerScanRows = 10

var (
	memberIDHeader = regexp.MustCompile(`(?i)^\s*(member\s*(id|no\.?|number|#)|id\s*(no\.?|number)?|membership\s*(no\.?|number|id)|badge\s*(id|no\.?|number)?)\s*#?\.?\s*$`)
	nameHeader     = regexp.MustCompile(`(?i)^\s*(full\s*name|name|member\s*name|member'?s\s*name)\s*$`)
	categoryHeader = regexp.MustCompile(`(?i)^\s*(category|voter\s*(category|type|status)|voting\s*(category|status))\s*$`)
	// Separators dropped before reading a category: "Non-Voting", "non voting".
	categorySeparators = regexp.MustCompile(`[\s_-]+`)
	// Election token amounts. "SURVEY" and "LUCKY PICK" on their own are
	// yes/no columns, so only the amount headers match.
	attendanceHeader = regexp.MustCompile(`(?i)^\s*attendance(\s*(token|amount|reward))?\s*$`)
	surveyHeader     = regexp.MustCompile(`(?i)^\s*completed?\s*survey(\s*(token|amount|reward))?\s*$`)
	luckyPickHeader  = regexp.MustCompile(`(?i)^\s*lucky\s*pick\s*prize(\s*(token|amount))?\s*$`)
	// Dropped before reading an amount: "₱1,000.00", "PHP 500".
	amountNoise = regexp.MustCompile(`(?i)[\s,₱]|php`)
	// "696.0" - a number Excel stored as a float.
	floatTail = regexp.MustCompile(`^(\d+)\.0+$`)
)

// readSheet returns the first worksheet's cells as text, as Excel displays
// them (so a member ID formatted 000696 keeps its zeros), or a CSV's rows.
func readSheet(fileName string, data []byte) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".xlsx", ".xlsm":
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			return nil, invalid("Couldn't read %s as an Excel file", fileName)
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, invalid("%s has no worksheets", fileName)
		}
		rows, err := f.GetRows(sheets[0])
		if err != nil {
			return nil, invalid("Couldn't read the first worksheet of %s", fileName)
		}
		return rows, nil
	case ".csv":
		r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
		r.FieldsPerRecord, r.LazyQuotes = -1, true
		rows, err := r.ReadAll()
		if err != nil {
			return nil, invalid("Couldn't read %s as a CSV file", fileName)
		}
		return rows, nil
	default:
		return nil, invalid("Upload an Excel (.xlsx) or CSV file")
	}
}

// parseCategory reads a Category cell as CategoryVoting or
// CategoryNonVoting; ok is false for a blank or unknown one.
func parseCategory(s string) (category string, ok bool) {
	switch categorySeparators.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "") {
	case "voting", "voter":
		return models.CategoryVoting, true
	case "nonvoting", "nonvoter":
		return models.CategoryNonVoting, true
	}
	return "", false
}

// parseToken reads an election token amount in pesos; blank is 0.
func parseToken(s string) (float64, bool) {
	s = amountNoise.ReplaceAllString(s, "")
	if s == "" || s == "-" {
		return 0, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > models.MaxToken || math.IsNaN(v) {
		return 0, false
	}
	return math.Round(v*100) / 100, true
}

// validTokens checks amounts sent back in a save request.
func validTokens(t models.Tokens) bool {
	for _, v := range []float64{t.Attendance, t.Survey, t.LuckyPick} {
		if v < 0 || v > models.MaxToken || math.IsNaN(v) {
			return false
		}
	}
	return true
}

// parseMasterList finds the header row (a "Member ID" or "Badge" column, and
// "Name", "Category" and token amount columns if there are) and reads the
// members below it. Without a Category column everyone is voting; with one,
// rows whose category is blank or unknown go to badCategory instead of rows.
// An unreadable token amount rejects the file. blank counts rows with a name
// but no member ID.
func parseMasterList(fileName string, data []byte) (rows, badCategory []models.MasterListRow, blank int, err error) {
	sheet, err := readSheet(fileName, data)
	if err != nil {
		return nil, nil, 0, err
	}

	header, idCol, nameCol, categoryCol := -1, -1, -1, -1
	attendanceCol, surveyCol, luckyPickCol := -1, -1, -1
	for i := 0; i < min(len(sheet), headerScanRows) && header < 0; i++ {
		for j, cell := range sheet[i] {
			if idCol < 0 && memberIDHeader.MatchString(cell) {
				header, idCol = i, j
			}
		}
		if header >= 0 {
			for j, cell := range sheet[i] {
				switch {
				case j == idCol:
				case nameCol < 0 && nameHeader.MatchString(cell):
					nameCol = j
				case categoryCol < 0 && categoryHeader.MatchString(cell):
					categoryCol = j
				case attendanceCol < 0 && attendanceHeader.MatchString(cell):
					attendanceCol = j
				case surveyCol < 0 && surveyHeader.MatchString(cell):
					surveyCol = j
				case luckyPickCol < 0 && luckyPickHeader.MatchString(cell):
					luckyPickCol = j
				}
			}
		}
	}
	if header < 0 {
		return nil, nil, 0, invalid("Couldn't find a \"Member ID\" or \"Badge\" column in the first %d rows of %s", headerScanRows, fileName)
	}

	cell := func(row []string, col int) string {
		if col < 0 || col >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[col])
	}
	for i := header + 1; i < len(sheet); i++ {
		memberNo, name := cell(sheet[i], idCol), cell(sheet[i], nameCol)
		if m := floatTail.FindStringSubmatch(memberNo); m != nil {
			memberNo = m[1]
		}
		switch {
		case memberNo == "" && name == "":
			continue
		case memberNo == "":
			blank++
			continue
		}
		row := models.MasterListRow{Row: i + 1, MemberNo: memberNo, Name: name, Category: models.CategoryVoting}
		for _, t := range []struct {
			col    int
			header string
			dst    *float64
		}{
			{attendanceCol, "Attendance", &row.Attendance},
			{surveyCol, "Complete Survey", &row.Survey},
			{luckyPickCol, "Lucky Pick Prize", &row.LuckyPick},
		} {
			v, ok := parseToken(cell(sheet[i], t.col))
			if !ok {
				return nil, nil, 0, invalid("Row %d: %q in the %s column isn't an amount (0 to %d)",
					i+1, cell(sheet[i], t.col), t.header, models.MaxToken)
			}
			*t.dst = v
		}
		if categoryCol >= 0 {
			raw := cell(sheet[i], categoryCol)
			category, ok := parseCategory(raw)
			if !ok {
				row.Category = raw
				badCategory = append(badCategory, row)
				continue
			}
			row.Category = category
		}
		rows = append(rows, row)
		if len(rows)+len(badCategory) > models.MaxMasterListRows {
			return nil, nil, 0, invalid("A master list can have up to %d members", models.MaxMasterListRows)
		}
	}
	if len(rows) == 0 && len(badCategory) == 0 {
		return nil, nil, 0, invalid("%s has no member IDs under the \"Member ID\" column", fileName)
	}
	return rows, badCategory, blank, nil
}

// accountIndex matches member IDs to portal accounts: exactly (ignoring
// case), or - when Excel dropped leading zeros ("000696" -> "696") - by the
// number without them, if that's unambiguous.
type accountIndex struct {
	exact    map[string]models.AccountRef
	unpadded map[string][]models.AccountRef
}

func newAccountIndex(accounts []models.AccountRef) accountIndex {
	idx := accountIndex{exact: map[string]models.AccountRef{}, unpadded: map[string][]models.AccountRef{}}
	for _, a := range accounts {
		key := strings.ToUpper(a.MemberNo)
		idx.exact[key] = a
		idx.unpadded[strings.TrimLeft(key, "0")] = append(idx.unpadded[strings.TrimLeft(key, "0")], a)
	}
	return idx
}

func (idx accountIndex) find(memberNo string) (models.AccountRef, bool) {
	key := strings.ToUpper(strings.TrimSpace(memberNo))
	if a, ok := idx.exact[key]; ok {
		return a, true
	}
	if candidates := idx.unpadded[strings.TrimLeft(key, "0")]; len(candidates) == 1 {
		return candidates[0], true
	}
	return models.AccountRef{}, false
}

// matchRows splits rows into members with a portal account (one each),
// rows with none, and repeats. A repeat with a different category than the
// member's first row is a conflict; the first row is kept.
func matchRows(idx accountIndex, rows []models.MasterListRow) (members []models.MasterListMember, notFound, duplicates, conflicts []models.MasterListRow) {
	members, notFound, duplicates, conflicts = []models.MasterListMember{}, []models.MasterListRow{}, []models.MasterListRow{}, []models.MasterListRow{}
	seen := map[int64]string{}
	var missing []string
	for _, r := range rows {
		a, ok := idx.find(r.MemberNo)
		if !ok {
			key := strings.ToUpper(r.MemberNo)
			if slices.Contains(missing, key) {
				duplicates = append(duplicates, r)
			} else {
				missing = append(missing, key)
				notFound = append(notFound, r)
			}
			continue
		}
		category := r.Category
		if category == "" {
			category = models.CategoryVoting
		}
		if first, ok := seen[a.UserID]; ok {
			if first != category {
				conflicts = append(conflicts, r)
			} else {
				duplicates = append(duplicates, r)
			}
			continue
		}
		seen[a.UserID] = category
		members = append(members, models.MasterListMember{
			MemberNo: a.MemberNo, ListedName: r.Name, AccountName: a.Name, Status: a.Status,
			Voting: category == models.CategoryVoting, Tokens: r.Tokens, UserID: a.UserID,
		})
	}
	return members, notFound, duplicates, conflicts
}
