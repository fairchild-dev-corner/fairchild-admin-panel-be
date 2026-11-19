package services

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/claims"

	"github.com/xuri/excelize/v2"
)

type listRepo struct {
	interfaces.ClaimsRepositoryInterface
	claims []models.ClaimRequest
	got    models.ClaimListQuery
}

func (r *listRepo) List(_ context.Context, q models.ClaimListQuery) ([]models.ClaimRequest, error) {
	r.got = q
	return r.claims, nil
}

func TestExportBuildsWorkbook(t *testing.T) {
	repo := &listRepo{claims: []models.ClaimRequest{
		{
			ID: "CR-00015", MemberNo: "003763", FullName: "Edgar Ancajas", ProposedEmail: "e@example.com",
			ProposedMobileNumber: "09171234502", Status: "rejected", RejectionReason: "Couldn't verify",
			SubmittedAt: "2026-11-03T04:26:00Z", ReviewedAt: "2026-11-04T01:00:00Z", ReviewedBy: "Justin Dequillo",
			Account: &models.ClaimAccount{FirstName: "EDGAR", LastName: "ANCAJAS"},
		},
		{ID: "CR-00012", MemberNo: "000696", Status: "approved", Message: "=1+1"},
	}}

	file, name, err := NewClaimsService(repo, nil, nil).Export(context.Background(),
		models.ClaimListQuery{Status: "rejected", Search: "  edgar ", Page: 4, PageSize: 15})
	if err != nil {
		t.Fatal(err)
	}
	if repo.got.Page != 1 || repo.got.PageSize != MaxExportRows || repo.got.Search != "edgar" || repo.got.Status != "rejected" {
		t.Fatalf("export should list every match, got query %+v", repo.got)
	}
	if !strings.HasPrefix(name, "account-claims-rejected-") || !strings.HasSuffix(name, ".xlsx") {
		t.Fatalf("file name %q", name)
	}

	f, err := excelize.OpenReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Claims")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want a header and 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "Claim ID" || rows[0][11] != "Reviewed by" {
		t.Fatalf("header %v", rows[0])
	}
	first := rows[1]
	// Leading zeros survive, names are joined, times are Asia/Manila.
	if first[1] != "003763" || first[3] != "EDGAR ANCAJAS" || first[7] != "Rejected" ||
		first[9] != "2026-11-03 12:26" || first[10] != "2026-11-04 09:00" {
		t.Fatalf("first row %v", first)
	}
	// Text that looks like a formula is stored as text.
	if rows[2][6] != "=1+1" {
		t.Fatalf("message %q", rows[2][6])
	}
	if cellType, _ := f.GetCellType("Claims", "G3"); cellType != excelize.CellTypeSharedString && cellType != excelize.CellTypeInlineString {
		t.Fatalf("message cell type %v, want text", cellType)
	}
	if formula, _ := f.GetCellFormula("Claims", "G3"); formula != "" {
		t.Fatalf("message became a formula: %q", formula)
	}
}

func TestExportFileNameAll(t *testing.T) {
	for _, status := range []string{"", "all"} {
		if n := exportFileName(status); !strings.HasPrefix(n, "account-claims-all-") {
			t.Fatalf("%q -> %q", status, n)
		}
	}
}
