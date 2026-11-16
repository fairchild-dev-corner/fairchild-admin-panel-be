package services

import (
	"context"
	"errors"
	"fairchild-admin-panel-be/internal/interfaces"
	apps "fairchild-admin-panel-be/internal/models/applications"
	models "fairchild-admin-panel-be/internal/models/metrics"
	"fairchild-admin-panel-be/internal/utils"
	"testing"
	"time"
)

type chartRepo struct {
	interfaces.MetricsRepositoryInterface
	err error
}

func (r chartRepo) LoanApplicationsPerMonth(context.Context, time.Time) ([]models.MonthCount, error) {
	return []models.MonthCount{{Month: "2025-12", Count: 3}}, r.err
}
func (r chartRepo) MembershipApplicationsPerMonth(context.Context, time.Time) ([]models.MonthCount, error) {
	return []models.MonthCount{{Month: "2026-01", Count: 2}}, r.err
}
func (r chartRepo) ApprovedLoansByProduct(context.Context) ([]models.LoansByProduct, error) {
	return []models.LoansByProduct{{Product: "Unexpected", Count: 1}}, r.err
}
func TestMetricsZeroFillAndYearBoundary(t *testing.T) {
	s := NewMetricsService(chartRepo{})
	got, err := s.applicationsPerMonth(context.Background(), time.Date(2026, 1, 15, 0, 0, 0, 0, utils.ManilaTZ))
	if err != nil || len(got) != 6 {
		t.Fatalf("%v %v", got, err)
	}
	if got[0].Label != "Aug" || got[0].Loans != 0 || got[4].Loans != 3 || got[5].Memberships != 2 {
		t.Fatalf("%+v", got)
	}
	products, err := s.loansByProduct(context.Background())
	if err != nil || len(products) != len(apps.LoanProducts)+1 || products[len(products)-1].Product != "Unexpected" {
		t.Fatalf("%+v %v", products, err)
	}
	failure := errors.New("db unavailable")
	if _, err := NewMetricsService(chartRepo{err: failure}).GetMetrics(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func BenchmarkMetrics(b *testing.B) {
	s := NewMetricsService(chartRepo{})
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.GetMetrics(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}
