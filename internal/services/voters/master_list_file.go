package services

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	models "fairchild-admin-panel-be/internal/models/voters"

	"github.com/xuri/excelize/v2"
)

// headerScanRows is how far down a sheet the header row may be (a title or
// note above it is common).
const headerScanRows = 10

var (
	memberIDHeader = regexp.MustCompile(`(?i)^\s*(member\s*(id|no\.?|number|#)|id\s*(no\.?|number)?|membership\s*(no\.?|number|id))\s*#?\.?\s*$`)
	nameHeader     = regexp.MustCompile(`(?i)^\s*(full\s*name|name|member\s*name|member'?s\s*name)\s*$`)
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

// parseMasterList finds the header row (a "Member ID" column, and a "Name"
// column if there is one) and reads the members below it. blank counts rows
// with a name but no member ID.
func parseMasterList(fileName string, data []byte) (rows []models.MasterListRow, blank int, err error) {
	sheet, err := readSheet(fileName, data)
	if err != nil {
		return nil, 0, err
	}

	header, idCol, nameCol := -1, -1, -1
	for i := 0; i < min(len(sheet), headerScanRows) && header < 0; i++ {
		for j, cell := range sheet[i] {
			if idCol < 0 && memberIDHeader.MatchString(cell) {
				header, idCol = i, j
			}
		}
		if header >= 0 {
			for j, cell := range sheet[i] {
				if j != idCol && nameHeader.MatchString(cell) {
					nameCol = j
					break
				}
			}
		}
	}
	if header < 0 {
		return nil, 0, invalid("Couldn't find a \"Member ID\" column in the first %d rows of %s", headerScanRows, fileName)
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
		rows = append(rows, models.MasterListRow{Row: i + 1, MemberNo: memberNo, Name: name})
		if len(rows) > models.MaxMasterListRows {
			return nil, 0, invalid("A master list can have up to %d members", models.MaxMasterListRows)
		}
	}
	if len(rows) == 0 {
		return nil, 0, invalid("%s has no member IDs under the \"Member ID\" column", fileName)
	}
	return rows, blank, nil
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
// rows with none, and repeats.
func matchRows(idx accountIndex, rows []models.MasterListRow) (members []models.MasterListMember, notFound, duplicates []models.MasterListRow) {
	members, notFound, duplicates = []models.MasterListMember{}, []models.MasterListRow{}, []models.MasterListRow{}
	seen := map[int64]bool{}
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
		if seen[a.UserID] {
			duplicates = append(duplicates, r)
			continue
		}
		seen[a.UserID] = true
		members = append(members, models.MasterListMember{
			MemberNo: a.MemberNo, ListedName: r.Name, AccountName: a.Name, Status: a.Status, UserID: a.UserID,
		})
	}
	return members, notFound, duplicates
}
