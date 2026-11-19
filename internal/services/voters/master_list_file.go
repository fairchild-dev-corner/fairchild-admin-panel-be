package services

import (
	"bytes"
	"encoding/csv"
	"fmt"
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
	memberIDHeader = regexp.MustCompile(`(?i)^\s*(member\s*(id|no\.?|number|#)|id\s*(no\.?|number)?|membership\s*(no\.?|number|id)|badge\s*(id|no\.?|number)?|reference\s*(id|no\.?|number))\s*#?\.?\s*$`)
	nameHeader     = regexp.MustCompile(`(?i)^\s*(full\s*name|name|member\s*name|member'?s\s*name|name\s*of\s*(the\s*)?(client|member)|client\s*name)\s*$`)
	categoryHeader = regexp.MustCompile(`(?i)^\s*(category|voter\s*(category|type|status)|voting\s*(category|status))\s*$`)
	// A yes/no column (any value but blank, no, 0 or false means delinquent).
	// "Deliquent" is a common misspelling.
	delinquentHeader = regexp.MustCompile(`(?i)^\s*deli(n)?quen(t|cy)(\s*(status|flag|member))?\s*$`)
	// An ineligible member's reason; one naming delinquency marks them delinquent.
	remarksHeader    = regexp.MustCompile(`(?i)^\s*(remarks?|reasons?)\s*$`)
	delinquentRemark = regexp.MustCompile(`(?i)delinquen`)
	// Yes/no columns naming a requirement an ineligible member didn't meet
	// (filled in, usually with the member's ID, when they didn't).
	lessThan4kHeader = regexp.MustCompile(`(?i)^\s*less\s*than\s*(php|₱)?\s*4\s*,?\s*(k|000)\s*$`)
	unpaidFeeHeader  = regexp.MustCompile(`(?i)^\s*unpaid\s*membership\s*fees?\s*$`)
	noTINHeader      = regexp.MustCompile(`(?i)^\s*no\s*tin\s*$`)
	spaces           = regexp.MustCompile(`\s+`)
	ineligibleSheet  = regexp.MustCompile(`(?i)ineligible`)
	votingSheet      = regexp.MustCompile(`(?i)^\s*voting\s*list`)
	branchListSheet  = regexp.MustCompile(`(?i)\blist\s*$`)
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

// sheet is one worksheet's cells as text, as Excel displays them (so a
// member ID formatted 000696 keeps its zeros). A CSV is one unnamed sheet.
type sheet struct {
	name string
	rows [][]string
}

// readWorkbook returns every worksheet of an Excel file, or a CSV's rows.
func readWorkbook(fileName string, data []byte) ([]sheet, error) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".xlsx", ".xlsm":
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			return nil, invalid("Couldn't read %s as an Excel file", fileName)
		}
		defer f.Close()
		names := f.GetSheetList()
		if len(names) == 0 {
			return nil, invalid("%s has no worksheets", fileName)
		}
		sheets := make([]sheet, 0, len(names))
		for _, name := range names {
			rows, err := f.GetRows(name)
			if err != nil {
				return nil, invalid("Couldn't read the %q worksheet of %s", name, fileName)
			}
			sheets = append(sheets, sheet{name: name, rows: rows})
		}
		return sheets, nil
	case ".csv":
		r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
		r.FieldsPerRecord, r.LazyQuotes = -1, true
		rows, err := r.ReadAll()
		if err != nil {
			return nil, invalid("Couldn't read %s as a CSV file", fileName)
		}
		return []sheet{{rows: rows}}, nil
	default:
		return nil, invalid("Upload an Excel (.xlsx) or CSV file")
	}
}

// sheetRole says how a worksheet of a multi-tab master list is read: an
// "All Ineligible" tab makes everyone on it ineligible, a "Voting List
// (Only)" tab makes everyone voting, and a branch tab ("Main List",
// "Knowles List", ...) uses each row's Category. Other tabs ("All Listing",
// "Summary") are skipped.
func sheetRole(name string) string {
	switch {
	case ineligibleSheet.MatchString(name):
		return models.SheetIneligible
	case votingSheet.MatchString(name):
		return models.SheetVoting
	case branchListSheet.MatchString(name):
		return models.SheetBranch
	}
	return ""
}

// parseCategory reads a Category cell as CategoryVoting, CategoryNonVoting
// or CategoryIneligible; "Delinquent" is ineligible and sets delinquent. ok
// is false for a blank or unknown one.
func parseCategory(s string) (category string, delinquent, ok bool) {
	switch categorySeparators.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "") {
	case "voting", "voter":
		return models.CategoryVoting, false, true
	case "nonvoting", "nonvoter":
		return models.CategoryNonVoting, false, true
	case "ineligible", "noteligible", "inelligible":
		return models.CategoryIneligible, false, true
	case "delinquent":
		return models.CategoryIneligible, true, true
	}
	return "", false, false
}

// The requirement each yes/no column stands for, worded like the Remarks
// column, for members whose file gives no remark.
const (
	reasonDelinquent = "Non-delinquent on coop payables"
	reasonLessThan4k = "25% of the total subscribed share capital must be paid up, and this paid-up amount cannot be less than the required minimum of Php4,000"
	reasonUnpaidFee  = "Fully paid membership fee"
	reasonNoTIN      = "Have submitted the TIN"
)

// addReasons appends the reasons not already in list (ignoring case and
// spacing), each tidied and capped at MaxReasonLength, up to MaxReasons.
func addReasons(list []string, reasons ...string) []string {
	for _, r := range reasons {
		r = strings.TrimSpace(spaces.ReplaceAllString(r, " "))
		if r == "" || len(list) >= models.MaxReasons {
			continue
		}
		if runes := []rune(r); len(runes) > models.MaxReasonLength {
			r = string(runes[:models.MaxReasonLength])
		}
		if !slices.ContainsFunc(list, func(s string) bool { return strings.EqualFold(s, r) }) {
			list = append(list, r)
		}
	}
	return list
}

// isFlagged reads a yes/no column such as Delinquent: blank, "no", "0",
// "false", "-" and "n/a" are no; anything else (often the member's ID) is yes.
func isFlagged(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "no", "n", "0", "false", "-", "n/a", "na", "none":
		return false
	}
	return true
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

// parsedList is what an uploaded master list holds, one row per member ID.
type parsedList struct {
	rows        []models.MasterListRow
	badCategory []models.MasterListRow
	// Repeats of a member ID within one tab, and members whose tabs disagree
	// on the category (the row that lost; see mergeRows).
	duplicates, conflicts []models.MasterListRow
	// Rows with a name but no member ID.
	blank  int
	sheets []models.MasterListSheet
}

// parseMasterList reads an uploaded master list. A workbook with "All
// Ineligible", "Voting List" or branch "... List" tabs is read from those
// tabs together (see sheetRole and mergeRows); anything else from its first
// worksheet.
func parseMasterList(fileName string, data []byte) (parsedList, error) {
	sheets, err := readWorkbook(fileName, data)
	if err != nil {
		return parsedList{}, err
	}

	var out parsedList
	var tabs []parsedTab
	for _, s := range sheets {
		role := sheetRole(s.name)
		if role == "" {
			continue
		}
		rows, bad, blank, err := parseSheet(s, role)
		if err != nil {
			return parsedList{}, err
		}
		tabs = append(tabs, parsedTab{role: role, rows: rows, bad: bad})
		out.blank += blank
		out.sheets = append(out.sheets, models.MasterListSheet{Name: s.name, Role: role, Rows: len(rows) + len(bad)})
	}

	if len(tabs) == 0 {
		rows, bad, blank, err := parseSheet(sheets[0], "")
		if err != nil {
			return parsedList{}, err
		}
		out.rows, out.badCategory, out.blank = rows, bad, blank
	} else {
		// Ineligible first, then voting, then the branch tabs, so a member's
		// first row comes from the tab that decides their category.
		slices.SortStableFunc(tabs, func(a, b parsedTab) int { return roleOrder[a.role] - roleOrder[b.role] })
		out.rows, out.badCategory, out.duplicates, out.conflicts = mergeRows(tabs)
	}

	if len(out.rows) == 0 && len(out.badCategory) == 0 {
		return parsedList{}, invalid("%s has no member IDs under the \"Member ID\" or \"Reference ID\" column", fileName)
	}
	if len(out.rows)+len(out.badCategory) > models.MaxMasterListRows {
		return parsedList{}, invalid("A master list can have up to %d members", models.MaxMasterListRows)
	}
	return out, nil
}

type parsedTab struct {
	role      string
	rows, bad []models.MasterListRow
}

var roleOrder = map[string]int{models.SheetIneligible: 0, models.SheetVoting: 1, models.SheetBranch: 2}

// Which category wins when tabs disagree: ineligible over voting over
// non-voting, so a member is never given a vote another tab took away.
var categoryRank = map[string]int{models.CategoryNonVoting: 1, models.CategoryVoting: 2, models.CategoryIneligible: 3}

// mergeRows combines the tabs into one row per member ID. Tabs disagreeing
// on the category go to conflicts (the losing row), and the higher-ranked
// category is kept; a member ID repeated within one tab goes to duplicates.
// Delinquent is set if any tab says so, the name is the first one given and
// each token amount the largest. A row with an unreadable category is only
// reported when no other tab places that member.
func mergeRows(tabs []parsedTab) (rows, bad, duplicates, conflicts []models.MasterListRow) {
	type tabKey struct {
		tab int
		key string
	}
	index, seen := map[string]int{}, map[tabKey]bool{}
	for t, tab := range tabs {
		for _, r := range tab.rows {
			key := strings.ToUpper(r.MemberNo)
			if seen[tabKey{t, key}] {
				// A repeat on the same tab can add a reason (one row per
				// requirement not met).
				m := &rows[index[key]]
				m.Reasons = addReasons(m.Reasons, r.Reasons...)
				duplicates = append(duplicates, r)
				continue
			}
			seen[tabKey{t, key}] = true

			i, ok := index[key]
			if !ok {
				index[key] = len(rows)
				rows = append(rows, r)
				continue
			}
			m := &rows[i]
			if r.Category != m.Category {
				if categoryRank[r.Category] > categoryRank[m.Category] {
					conflicts = append(conflicts, *m)
					m.Category = r.Category
				} else {
					conflicts = append(conflicts, r)
				}
			}
			m.Delinquent = m.Delinquent || r.Delinquent
			if m.Name == "" {
				m.Name = r.Name
			}
			// The ineligible tab's remarks come first; a branch tab's flag
			// columns only fill in for members without any.
			if len(m.Reasons) == 0 {
				m.Reasons = r.Reasons
			}
			m.Attendance = max(m.Attendance, r.Attendance)
			m.Survey = max(m.Survey, r.Survey)
			m.LuckyPick = max(m.LuckyPick, r.LuckyPick)
		}
	}
	for _, tab := range tabs {
		for _, r := range tab.bad {
			if _, ok := index[strings.ToUpper(r.MemberNo)]; !ok {
				bad = append(bad, r)
			}
		}
	}
	return rows, bad, duplicates, conflicts
}

// parseSheet finds a worksheet's header row (a "Member ID", "Reference ID"
// or "Badge" column, and "Name", "Category", "Delinquent", "Remarks" and
// token amount columns if there are) and reads the members below it. role
// is the tab's sheetRole, or "" for a single-sheet upload. Without a
// Category column everyone is voting (or the tab's own category); with one,
// rows whose category is blank or unknown go to badCategory instead of rows.
// An unreadable token amount rejects the file. blank counts rows with a name
// but no member ID.
func parseSheet(s sheet, role string) (rows, badCategory []models.MasterListRow, blank int, err error) {
	where := ""
	if s.name != "" {
		where = fmt.Sprintf(" of the %q tab", s.name)
	}
	header, idCol, nameCol, categoryCol, delinquentCol, remarksCol := -1, -1, -1, -1, -1, -1
	attendanceCol, surveyCol, luckyPickCol, lessThan4kCol, unpaidFeeCol, noTINCol := -1, -1, -1, -1, -1, -1
	for i := 0; i < min(len(s.rows), headerScanRows) && header < 0; i++ {
		for j, cell := range s.rows[i] {
			if idCol < 0 && memberIDHeader.MatchString(cell) {
				header, idCol = i, j
			}
		}
		if header >= 0 {
			for j, cell := range s.rows[i] {
				switch {
				case j == idCol:
				case nameCol < 0 && nameHeader.MatchString(cell):
					nameCol = j
				case categoryCol < 0 && categoryHeader.MatchString(cell):
					categoryCol = j
				case delinquentCol < 0 && delinquentHeader.MatchString(cell):
					delinquentCol = j
				case remarksCol < 0 && remarksHeader.MatchString(cell):
					remarksCol = j
				case lessThan4kCol < 0 && lessThan4kHeader.MatchString(cell):
					lessThan4kCol = j
				case unpaidFeeCol < 0 && unpaidFeeHeader.MatchString(cell):
					unpaidFeeCol = j
				case noTINCol < 0 && noTINHeader.MatchString(cell):
					noTINCol = j
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
		return nil, nil, 0, invalid("Couldn't find a \"Member ID\", \"Reference ID\" or \"Badge\" column in the first %d rows%s",
			headerScanRows, where)
	}

	// A tab's own category overrides the Category column.
	forced := map[string]string{models.SheetIneligible: models.CategoryIneligible, models.SheetVoting: models.CategoryVoting}[role]

	cell := func(row []string, col int) string {
		if col < 0 || col >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[col])
	}
	for i := header + 1; i < len(s.rows); i++ {
		line := s.rows[i]
		memberNo, name := cell(line, idCol), cell(line, nameCol)
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
		row := models.MasterListRow{Row: i + 1, Sheet: s.name, MemberNo: memberNo, Name: name, Category: models.CategoryVoting}
		remark := cell(line, remarksCol)
		row.Delinquent = isFlagged(cell(line, delinquentCol)) || delinquentRemark.MatchString(remark)
		// The Remarks cell, or else the requirement columns filled in.
		if row.Reasons = addReasons(nil, remark); len(row.Reasons) == 0 {
			for _, f := range []struct {
				col    int
				reason string
			}{
				{delinquentCol, reasonDelinquent},
				{lessThan4kCol, reasonLessThan4k},
				{unpaidFeeCol, reasonUnpaidFee},
				{noTINCol, reasonNoTIN},
			} {
				if f.col >= 0 && isFlagged(cell(line, f.col)) {
					row.Reasons = addReasons(row.Reasons, f.reason)
				}
			}
		}
		for _, t := range []struct {
			col    int
			header string
			dst    *float64
		}{
			{attendanceCol, "Attendance", &row.Attendance},
			{surveyCol, "Complete Survey", &row.Survey},
			{luckyPickCol, "Lucky Pick Prize", &row.LuckyPick},
		} {
			v, ok := parseToken(cell(line, t.col))
			if !ok {
				return nil, nil, 0, invalid("Row %d%s: %q in the %s column isn't an amount (0 to %d)",
					i+1, where, cell(line, t.col), t.header, models.MaxToken)
			}
			*t.dst = v
		}
		switch {
		case forced != "":
			row.Category = forced
		case categoryCol >= 0:
			raw := cell(line, categoryCol)
			category, delinquent, ok := parseCategory(raw)
			if !ok {
				row.Category = raw
				badCategory = append(badCategory, row)
				continue
			}
			row.Category = category
			row.Delinquent = row.Delinquent || delinquent
		}
		rows = append(rows, row)
		if len(rows)+len(badCategory) > models.MaxMasterListRows {
			return nil, nil, 0, invalid("A master list can have up to %d members", models.MaxMasterListRows)
		}
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
		member := models.MasterListMember{
			MemberNo: a.MemberNo, ListedName: r.Name, AccountName: a.Name, Status: a.Status,
			Voting: category == models.CategoryVoting, Category: category, Delinquent: r.Delinquent,
			Tokens: r.Tokens, UserID: a.UserID,
		}
		// Reasons only explain why someone can't vote. Tidied again here
		// since a save request sends them back.
		if !member.Voting {
			member.Reasons = addReasons(nil, r.Reasons...)
		}
		members = append(members, member)
	}
	return members, notFound, duplicates, conflicts
}
