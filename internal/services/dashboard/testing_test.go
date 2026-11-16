package services

import (
	"context"
	"errors"
	"fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/dashboard"
	"fairchild-admin-panel-be/internal/utils"
	"testing"
	"time"
)

type summaryRepo struct {
	interfaces.DashboardRepositoryInterface
	err   error
	start time.Time
}

func (r *summaryRepo) CountCentral(_ context.Context, start time.Time) (*models.CentralCounts, error) {
	r.start = start
	return &models.CentralCounts{TotalMembers: 100, PendingAccountClaims: 3}, r.err
}
func (r *summaryRepo) SumActiveLoans(context.Context) (*models.LoanTotals, error) {
	return &models.LoanTotals{Count: 4, Balance: 123.456}, r.err
}
func readyDashboard(r *summaryRepo) *DashboardService {
	ready := make(chan struct{})
	close(ready)
	return &DashboardService{repo: r, loanReady: ready, loanTotals: &models.LoanTotals{Count: 4, Balance: 123.456}}
}
func TestDashboardSummary(t *testing.T) {
	r := &summaryRepo{}
	s := readyDashboard(r)
	got, err := s.GetSummary(context.Background())
	if err != nil || got.TotalMembers != 100 || got.PendingAccountClaims != 3 || got.ActiveLoans != 4 || got.ActiveLoanBalance != 123.46 {
		t.Fatalf("%+v %v", got, err)
	}
	if r.start.In(utils.ManilaTZ).Day() != 1 || r.start.In(utils.ManilaTZ).Hour() != 0 {
		t.Fatal("wrong month boundary")
	}
	failure := errors.New("db unavailable")
	r.err = failure
	if _, err := s.GetSummary(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func TestDashboardKeepsStaleTotalsAndHonorsCancellation(t *testing.T) {
	r := &summaryRepo{err: errors.New("ledger unavailable")}
	s := readyDashboard(r)
	if s.refreshLoanTotals() {
		t.Fatal("failed refresh reported success")
	}
	got, err := s.activeLoanTotals(context.Background())
	if err != nil || got.Count != 4 {
		t.Fatal("stale totals lost")
	}
	s.loanReady = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.activeLoanTotals(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func BenchmarkDashboard(b *testing.B) {
	s := readyDashboard(&summaryRepo{})
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.GetSummary(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}
