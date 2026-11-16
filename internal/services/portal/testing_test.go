package services

import (
	"context"
	"errors"
	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/portal"
	"testing"
)

type lookupLedger struct {
	interfaces.LedgerLookupInterface
	calls int
	err   error
}

func (l *lookupLedger) Branches() []string { return []string{"main", "tiic"} }
func (l *lookupLedger) LoadMemberNos(_ context.Context, branch string) ([]models.LedgerLink, error) {
	l.calls++
	if l.err != nil {
		return nil, l.err
	}
	return []models.LedgerLink{{MemberNo: "00042", ClientID: 42, Branch: branch}}, nil
}
func TestPortalLedgerResolutionAndCache(t *testing.T) {
	ctx := context.Background()
	l := &lookupLedger{}
	s := NewPortalService(nil, l)
	resolver, err := s.BranchResolver(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if branch, ok := resolver(" 00042 ", "42"); !ok || branch != "main" {
		t.Fatalf("%s %v", branch, ok)
	}
	if _, ok := resolver("missing", ""); ok {
		t.Fatal("missing member matched")
	}
	if _, err := s.BranchResolver(ctx); err != nil || l.calls != 2 {
		t.Fatalf("cache missed: %d %v", l.calls, err)
	}
	index := map[string][]models.LedgerLink{"42": {{ClientID: 1, Branch: "main"}, {ClientID: 2, Branch: "tiic"}}}
	if link, ok := resolveLedger(index, "42", "2"); !ok || link.Branch != "tiic" {
		t.Fatal("client ID did not disambiguate")
	}
}
func TestPortalLedgerFailure(t *testing.T) {
	failure := errors.New("ledger unavailable")
	l := &lookupLedger{err: failure}
	if _, err := NewPortalService(nil, l).BranchResolver(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func BenchmarkPortal(b *testing.B) {
	index := map[string][]models.LedgerLink{"42": {{ClientID: 1, Branch: "main"}, {ClientID: 2, Branch: "tiic"}}}
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := resolveLedger(index, "42", "2"); !ok {
			b.Fatal("not found")
		}
	}
}
