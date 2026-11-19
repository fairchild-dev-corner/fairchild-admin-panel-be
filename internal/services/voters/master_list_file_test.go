package services

import (
	"fmt"
	"strings"
	"testing"

	models "fairchild-admin-panel-be/internal/models/voters"

	"github.com/xuri/excelize/v2"
)

func TestParseMasterListCategory(t *testing.T) {
	csv := "BADGE,NAME,DATA-TIME,CATEGORY,VOTED\n" +
		"MER061,\"SUGAROL, JANNICE E.\",Mar 07,voting,YES\n" +
		"K001198,\"NUNEZ, ARLENE A.\",Mar 07,Non-Voting,NO\n" +
		"003879,\"BELLEZA, ARLENE\",Mar 07,,YES\n" +
		"010118,\"ARIAS, JOSEPH\",Mar 07,maybe,YES\n"

	rows, bad, blank, err := parseRows("list.csv", []byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if blank != 0 {
		t.Errorf("blank = %d, want 0", blank)
	}
	if len(rows) != 2 || rows[0].Category != models.CategoryVoting || rows[1].Category != models.CategoryNonVoting {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].MemberNo != "MER061" || rows[0].Name != "SUGAROL, JANNICE E." {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if len(bad) != 2 || bad[0].MemberNo != "003879" || bad[1].Category != "maybe" {
		t.Errorf("bad = %+v", bad)
	}
}

func TestParseMasterListTokens(t *testing.T) {
	csv := "BADGE,NAME,CATEGORY,VOTED,SURVEY,LUCKY PICK,ATTENDANCE,COMPLETE SURVEY,LUCKY PICK PRIZE\n" +
		"MER061,\"SUGAROL, JANNICE E.\",voting,YES,YES,Not_Pick,100,500,\n" +
		"K001198,\"NUNEZ, ARLENE A.\",voting,YES,YES,Lucky_Winner,\"₱1,000.00\",500,100\n"

	rows, _, _, err := parseRows("list.csv", []byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	want := []models.Tokens{{Attendance: 100, Survey: 500}, {Attendance: 1000, Survey: 500, LuckyPick: 100}}
	for i, w := range want {
		if rows[i].Tokens != w {
			t.Errorf("row %d tokens = %+v, want %+v", i, rows[i].Tokens, w)
		}
	}

	if _, _, _, err := parseRows("list.csv", []byte("BADGE,ATTENDANCE\nMER061,YES\n")); err == nil {
		t.Error("want an error for a non-numeric attendance amount")
	}
}

func TestParseMasterListWithoutCategory(t *testing.T) {
	rows, bad, _, err := parseRows("list.csv", []byte("Member ID,Name\n000123,Juan\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 0 || len(rows) != 1 || rows[0].Category != models.CategoryVoting {
		t.Fatalf("rows = %+v, bad = %+v", rows, bad)
	}
}

func TestMatchRowsCategory(t *testing.T) {
	idx := newAccountIndex([]models.AccountRef{
		{UserID: 1, MemberNo: "MER061", Status: "active"},
		{UserID: 2, MemberNo: "K001198", Status: "active"},
	})
	members, notFound, duplicates, conflicts := matchRows(idx, []models.MasterListRow{
		{Row: 2, MemberNo: "MER061", Category: models.CategoryVoting},
		{Row: 3, MemberNo: "k001198", Category: models.CategoryNonVoting},
		{Row: 4, MemberNo: "MER061", Category: models.CategoryVoting},
		{Row: 5, MemberNo: "MER061", Category: models.CategoryNonVoting},
		{Row: 6, MemberNo: "ZZZ999"},
	})
	if len(members) != 2 || !members[0].Voting || members[1].Voting {
		t.Fatalf("members = %+v", members)
	}
	if len(duplicates) != 1 || duplicates[0].Row != 4 {
		t.Errorf("duplicates = %+v", duplicates)
	}
	if len(conflicts) != 1 || conflicts[0].Row != 5 {
		t.Errorf("conflicts = %+v", conflicts)
	}
	if len(notFound) != 1 || notFound[0].MemberNo != "ZZZ999" {
		t.Errorf("notFound = %+v", notFound)
	}
}

func TestParseMasterListIneligibleAndDelinquent(t *testing.T) {
	csv := "BADGE,NAME,CATEGORY,DELINQUENT\n" +
		"A1,\"ONE\",Ineligible,A1\n" +
		"A2,\"TWO\",Ineligible,\n" +
		"A3,\"THREE\",Delinquent,\n" +
		"A4,\"FOUR\",Voting,no\n"

	rows, bad, _, err := parseRows("list.csv", []byte(csv))
	if err != nil || len(bad) != 0 || len(rows) != 4 {
		t.Fatalf("rows=%+v bad=%+v err=%v", rows, bad, err)
	}
	want := []struct {
		category   string
		delinquent bool
	}{
		{models.CategoryIneligible, true},
		{models.CategoryIneligible, false},
		{models.CategoryIneligible, true},
		{models.CategoryVoting, false},
	}
	for i, w := range want {
		if rows[i].Category != w.category || rows[i].Delinquent != w.delinquent {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}

	members, _, _, _ := matchRows(newAccountIndex([]models.AccountRef{{UserID: 1, MemberNo: "A1"}}), rows[:1])
	if len(members) != 1 || members[0].Voting || members[0].Category != models.CategoryIneligible || !members[0].Delinquent {
		t.Errorf("members = %+v", members)
	}
}

// parseRows is parseMasterList for single-sheet tests.
func parseRows(fileName string, data []byte) (rows, bad []models.MasterListRow, blank int, err error) {
	list, err := parseMasterList(fileName, data)
	return list.rows, list.badCategory, list.blank, err
}

func TestParseMasterListWorkbookTabs(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	write := func(sheet string, rows [][]any) {
		if _, err := f.NewSheet(sheet); err != nil {
			t.Fatal(err)
		}
		for i, r := range rows {
			if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", i+1), &r); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Combined tab: skipped (its member ID header is blank).
	write("All Listing", [][]any{{"No.", "", "Name of Client", "Category"}, {1, "Z9", "Skipped", "Voting"}})
	write("All Ineligible", [][]any{
		{"No.", "Reference ID", "Name of Client", "Category", "Branch", "Remarks"},
		{1, "K1", "Ineligible One", "Ineligible", "Knowles", "Non delinquent of coop payables as of FEB 28, 2026"},
		{2, "K1", "Ineligible One", "Ineligible", "Knowles", "25% of the total subscribed share capital"},
		{3, "M3", "Also Voting", "Ineligible", "Main", "Have submitted the TIN"},
	})
	write("Voting List Only", [][]any{
		{"No.", "Reference ID", "Name of Client", "Category", "VOTED"},
		{1, "M1", "Voter", "Voting", "YES"},
		{2, "M3", "Also Voting", "Voting", "#N/A"},
	})
	write("Main List", [][]any{
		{"No.", "Reference ID", "Name of Client", "Category", "Branch", "Deliquent", "Less than 4k"},
		{1, "M1", "Voter", "Non Voting", "Main", "", ""},
		{2, "M2", "Non Voter", "Non Voting", "Main", "", ""},
		{3, "M4", "Branch Ineligible", "Ineligible", "Main", "M4", ""},
		{4, "M5", "Bad", "??", "Main", "", ""},
	})
	write("Summary", [][]any{{"Category", "Main"}, {"Voting", 1}})
	f.DeleteSheet("Sheet1")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}

	list, err := parseMasterList("GA.xlsx", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]models.MasterListRow{}
	for _, r := range list.rows {
		got[r.MemberNo] = r
	}
	want := map[string]struct {
		category   string
		delinquent bool
	}{
		"K1": {models.CategoryIneligible, true},
		"M3": {models.CategoryIneligible, false}, // ineligible beats the voting list
		"M1": {models.CategoryVoting, false},     // voting list beats the branch tab
		"M2": {models.CategoryNonVoting, false},
		"M4": {models.CategoryIneligible, true},
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %+v", list.rows)
	}
	for id, w := range want {
		if r := got[id]; r.Category != w.category || r.Delinquent != w.delinquent {
			t.Errorf("%s = %+v, want %+v", id, r, w)
		}
	}
	if len(list.duplicates) != 1 || list.duplicates[0].MemberNo != "K1" || list.duplicates[0].Sheet != "All Ineligible" {
		t.Errorf("duplicates = %+v", list.duplicates)
	}
	if len(list.conflicts) != 2 {
		t.Errorf("conflicts = %+v", list.conflicts)
	}
	if len(list.badCategory) != 1 || list.badCategory[0].MemberNo != "M5" {
		t.Errorf("bad = %+v", list.badCategory)
	}
	if len(list.sheets) != 3 {
		t.Errorf("sheets = %+v", list.sheets)
	}

	// Both of K1's remarks (one row each), and M4's from the Deliquent column.
	if r := got["K1"].Reasons; len(r) != 2 || r[0] != "Non delinquent of coop payables as of FEB 28, 2026" {
		t.Errorf("K1 reasons = %q", r)
	}
	if r := got["M4"].Reasons; len(r) != 1 || r[0] != reasonDelinquent {
		t.Errorf("M4 reasons = %q", r)
	}
	members, _, _, _ := matchRows(newAccountIndex([]models.AccountRef{{UserID: 1, MemberNo: "K1"}, {UserID: 2, MemberNo: "M1"}}),
		[]models.MasterListRow{got["K1"], {MemberNo: "M1", Category: models.CategoryVoting, Reasons: []string{"x"}}})
	if len(members) != 2 || len(members[0].Reasons) != 2 || members[1].Reasons != nil {
		t.Errorf("members = %+v", members)
	}
}

func TestAddReasons(t *testing.T) {
	got := addReasons(nil, "  Fully paid\n membership fee ", "fully PAID membership fee", "", strings.Repeat("a", 400))
	if len(got) != 2 || got[0] != "Fully paid membership fee" || len([]rune(got[1])) != models.MaxReasonLength {
		t.Errorf("got %q", got)
	}
}
