package services

import (
	"context"
	"slices"
	"testing"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/audit"
)

// fakeAuditRepo records the query it was given.
type fakeAuditRepo struct {
	got   []models.AuditLogQuery
	total int
}

func (f *fakeAuditRepo) ListRecentAuditLogs(context.Context, int) ([]models.AuditLogEntry, error) {
	return nil, nil
}

func (f *fakeAuditRepo) ListAuditLogs(_ context.Context, q models.AuditLogQuery) ([]models.AuditLogEntry, int, error) {
	f.got = append(f.got, q)
	return []models.AuditLogEntry{}, f.total, nil
}

func TestElectionAuditLogOnlyElectionActions(t *testing.T) {
	repo := &fakeAuditRepo{}
	s := NewAuditService(repo)

	if _, err := s.ListElectionAuditLog(context.Background(), cc.AUDIT_TURNED_ELECTION_ON, "  Juan ", 1, 20); err != nil {
		t.Fatal(err)
	}
	q := repo.got[0]
	if !slices.Equal(q.Actions, cc.ElectionAuditActions) || q.Action != cc.AUDIT_TURNED_ELECTION_ON || q.Search != "Juan" {
		t.Errorf("query = %+v", q)
	}

	// A non-election action can't be used to read the rest of the log.
	if _, err := s.ListElectionAuditLog(context.Background(), cc.AUDIT_APPROVED_LOAN_APPLICATION, "", 1, 20); err != nil {
		t.Fatal(err)
	}
	if len(repo.got[1].Actions) != 0 {
		t.Errorf("non-election action queried %v", repo.got[1].Actions)
	}
}

func TestElectionAuditLogClampsPage(t *testing.T) {
	repo := &fakeAuditRepo{total: 25}
	page, err := NewAuditService(repo).ListElectionAuditLog(context.Background(), "", "", 9, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 2 || repo.got[len(repo.got)-1].Page != 2 {
		t.Errorf("page = %d, want the last page 2", page.Page)
	}
}
