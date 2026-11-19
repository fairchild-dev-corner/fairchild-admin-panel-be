package services

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/locks"

	"github.com/xuri/excelize/v2"
)

type listRepo struct {
	interfaces.LocksRepositoryInterface
	locks []models.LockedAccount
	got   models.LockedAccountListQuery
}

func (r *listRepo) List(_ context.Context, q models.LockedAccountListQuery) ([]models.LockedAccount, error) {
	r.got = q
	return r.locks, nil
}

func TestExportBuildsWorkbook(t *testing.T) {
	repo := &listRepo{locks: []models.LockedAccount{
		{
			ID: "LK-00007", MemberNo: "003763", Status: "unlocked", IPAddress: "203.0.113.9", AttemptCount: 5,
			Reason: "Too many failed attempts", LockedAt: "2026-11-03T04:26:00Z", ExpiresAt: "2026-11-04T04:26:00Z",
			UnlockedAt: "2026-11-03T06:00:00Z", UnlockedVia: models.UnlockedViaPanel, UnlockedBy: "Justin Dequillo",
			Account: &models.LockedAccountUser{FirstName: "EDGAR", LastName: "ANCAJAS", Email: "e@example.com", Status: "active"},
		},
		{ID: "LK-00008", MemberNo: "000696", Status: "locked", AttemptCount: 3, LockedAt: "2026-11-19T01:00:00Z"},
	}}

	file, name, err := NewLocksService(repo, nil, nil).Export(context.Background(),
		models.LockedAccountListQuery{Status: "all", Search: " edgar ", Page: 3, PageSize: 15})
	if err != nil {
		t.Fatal(err)
	}
	if repo.got.Page != 1 || repo.got.PageSize != MaxExportRows || repo.got.Search != "edgar" || repo.got.Status != "all" {
		t.Fatalf("export should list every match, got query %+v", repo.got)
	}
	if !strings.HasPrefix(name, "locked-accounts-all-") || !strings.HasSuffix(name, ".xlsx") {
		t.Fatalf("file name %q", name)
	}

	f, err := excelize.OpenReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Locked accounts")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0][0] != "Lock ID" || rows[0][14] != "Unlocked by" {
		t.Fatalf("rows %v", rows)
	}
	first := rows[1]
	// Leading zeros survive; times are Asia/Manila; the unlock is described.
	if first[1] != "003763" || first[2] != "EDGAR ANCAJAS" || first[9] != "Unlocked" || first[10] != "2026-11-03 12:26" ||
		first[12] != "2026-11-03 14:00" || first[13] != "Admin unlocked" || first[14] != "Justin Dequillo" {
		t.Fatalf("first row %v", first)
	}
	// A lock with no portal account and no unlock still exports.
	if second := rows[2]; second[1] != "000696" || second[2] != "" || second[9] != "Locked" || len(second) > 12 && second[12] != "" {
		t.Fatalf("second row %v", second)
	}
}
