package services

import (
	"testing"

	models "fairchild-admin-panel-be/internal/models/voters"
)

func TestParseMasterListCategory(t *testing.T) {
	csv := "BADGE,NAME,DATA-TIME,CATEGORY,VOTED\n" +
		"MER061,\"SUGAROL, JANNICE E.\",Mar 07,voting,YES\n" +
		"K001198,\"NUNEZ, ARLENE A.\",Mar 07,Non-Voting,NO\n" +
		"003879,\"BELLEZA, ARLENE\",Mar 07,,YES\n" +
		"010118,\"ARIAS, JOSEPH\",Mar 07,maybe,YES\n"

	rows, bad, blank, err := parseMasterList("list.csv", []byte(csv))
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

	rows, _, _, err := parseMasterList("list.csv", []byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	want := []models.Tokens{{Attendance: 100, Survey: 500}, {Attendance: 1000, Survey: 500, LuckyPick: 100}}
	for i, w := range want {
		if rows[i].Tokens != w {
			t.Errorf("row %d tokens = %+v, want %+v", i, rows[i].Tokens, w)
		}
	}

	if _, _, _, err := parseMasterList("list.csv", []byte("BADGE,ATTENDANCE\nMER061,YES\n")); err == nil {
		t.Error("want an error for a non-numeric attendance amount")
	}
}

func TestParseMasterListWithoutCategory(t *testing.T) {
	rows, bad, _, err := parseMasterList("list.csv", []byte("Member ID,Name\n000123,Juan\n"))
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
