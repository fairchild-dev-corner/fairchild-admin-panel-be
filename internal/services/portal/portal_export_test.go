package services

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/portal"

	"github.com/xuri/excelize/v2"
)

type exportRepo struct {
	interfaces.PortalRepositoryInterface
	keys    []models.PortalKey
	members map[string]models.PortalMember
	lookups [][]int64
}

func (r *exportRepo) ListKeys(context.Context, string) ([]models.PortalKey, error) {
	return r.keys, nil
}
func (r *exportRepo) FindByIDs(_ context.Context, ids []int64) ([]models.PortalMember, error) {
	r.lookups = append(r.lookups, ids)
	var out []models.PortalMember
	for _, m := range r.members {
		out = append(out, m)
	}
	return out, nil
}

// Branch ledgers: 00042 is on main, 00043 on tiic.
type exportLedger struct{ interfaces.LedgerLookupInterface }

func (exportLedger) Branches() []string { return []string{"main", "tiic"} }
func (exportLedger) LoadMemberNos(_ context.Context, branch string) ([]models.LedgerLink, error) {
	if branch == "main" {
		return []models.LedgerLink{{MemberNo: "00042", Branch: "main", Category: "regular"}}, nil
	}
	return []models.LedgerLink{{MemberNo: "00043", Branch: "tiic"}}, nil
}

func exportFixture() *exportRepo {
	return &exportRepo{
		keys: []models.PortalKey{
			{ID: 3, MemberNo: "00043", Status: "suspended"},
			{ID: 2, MemberNo: "00042", Status: "pending_review"},
			{ID: 1, MemberNo: "ZZ9", Status: "pending_review"}, // not on a ledger
		},
		members: map[string]models.PortalMember{
			"PU-00003": {ID: "PU-00003", MemberNo: "00043", FirstName: "Ana", LastName: "Cruz", Status: "suspended"},
			"PU-00002": {
				ID: "PU-00002", MemberNo: "00042", FirstName: "Ben", LastName: "Reyes", MobileNumber: "09171234567",
				Email: "ben@example.com", EmailVerified: true, Status: "pending_review",
				RegisteredAt: "2026-11-03T04:26:00Z",
			},
			"PU-00001": {ID: "PU-00001", MemberNo: "ZZ9", FirstName: "Cy", LastName: "Lim", Status: "pending_review"},
		},
	}
}

func readSheet(t *testing.T, file []byte) [][]string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Portal accounts")
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestExportFiltersByStatusAndBranch(t *testing.T) {
	ctx := context.Background()
	s := NewPortalService(exportFixture(), exportLedger{})

	// A status exports just that tab, newest first, with accounts on no ledger.
	file, name, err := s.Export(ctx, models.PortalListQuery{Status: "pending_review"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "portal-accounts-pending_review-") || !strings.HasSuffix(name, ".xlsx") {
		t.Fatalf("file name %q", name)
	}
	rows := readSheet(t, file)
	if len(rows) != 3 || rows[0][0] != "Account ID" {
		t.Fatalf("rows %v", rows)
	}
	ben := rows[1]
	if ben[0] != "PU-00002" || ben[1] != "00042" || ben[6] != "ben@example.com" || ben[7] != "Yes" ||
		ben[8] != "09171234567" || ben[9] != "Main" || ben[12] != "Pending review" || ben[13] != "2026-11-03 12:26" {
		t.Fatalf("ben %v", ben)
	}
	if cy := rows[2]; cy[0] != "PU-00001" || cy[9] != "" {
		t.Fatalf("an account on no ledger has no branch: %v", cy)
	}

	// All takes every tab; a branch filter keeps only accounts on that ledger.
	file, _, err = s.Export(ctx, models.PortalListQuery{Status: "all"})
	if err != nil || len(readSheet(t, file)) != 4 {
		t.Fatalf("all: %v", err)
	}
	file, _, err = s.Export(ctx, models.PortalListQuery{Status: "all", Branches: []string{"tiic"}})
	if err != nil {
		t.Fatal(err)
	}
	if rows := readSheet(t, file); len(rows) != 2 || rows[1][0] != "PU-00003" || rows[1][9] != "TIIC" || rows[1][12] != "Suspended" {
		t.Fatalf("tiic rows %v", rows)
	}
}

func TestExportLooksUpInChunks(t *testing.T) {
	repo := &exportRepo{members: map[string]models.PortalMember{}}
	for i := int64(1); i <= 2500; i++ {
		repo.keys = append(repo.keys, models.PortalKey{ID: i, Status: "active"})
	}
	if _, _, err := NewPortalService(repo, exportLedger{}).Export(context.Background(), models.PortalListQuery{Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if len(repo.lookups) != 3 || len(repo.lookups[0]) != exportChunk || len(repo.lookups[2]) != 500 {
		t.Fatalf("want 3 lookups of 1000, 1000 and 500, got %d", len(repo.lookups))
	}
}
