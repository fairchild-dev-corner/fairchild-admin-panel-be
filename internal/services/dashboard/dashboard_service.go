package services

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	interfaces "fairchild-admin-panel-be/internal/interfaces"
	models "fairchild-admin-panel-be/internal/models/dashboard"
	"fairchild-admin-panel-be/internal/utils"
)

const (
	// The ledger aggregate scans every branch's sldtl - far too heavy to run
	// on every dashboard load, and loan balances only move when branches
	// post, so a few minutes of staleness is fine.
	loanTotalsTTL     = 10 * time.Minute
	loanTotalsTimeout = 30 * time.Second
)

type DashboardService struct {
	repo interfaces.DashboardRepositoryInterface

	mu          sync.Mutex
	loanTotals  *models.LoanTotals
	loanTotalAt time.Time
}

func NewDashboardService(repo interfaces.DashboardRepositoryInterface) *DashboardService {
	return &DashboardService{repo: repo}
}

func (s *DashboardService) GetSummary(ctx context.Context) (*models.DashboardSummary, error) {
	now := time.Now().In(utils.ManilaTZ)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, utils.ManilaTZ)

	central, err := s.repo.CountCentral(ctx, monthStart)
	if err != nil {
		return nil, err
	}

	loans, err := s.activeLoanTotals(ctx)
	if err != nil {
		return nil, err
	}

	return &models.DashboardSummary{
		TotalMembers:                  central.TotalMembers,
		NewMembersThisMonth:           central.NewMembersThisMonth,
		PendingLoanApplications:       central.PendingLoanApplications,
		PendingMembershipApplications: central.PendingMembershipApplications,
		ActiveLoans:                   loans.Count,
		ActiveLoanBalance:             math.Round(loans.Balance*100) / 100,
		MaintenanceMode:               central.MaintenanceMode,
	}, nil
}

// activeLoanTotals serves the cached ledger aggregate, refreshing it once
// it's older than loanTotalsTTL. The mutex also collapses concurrent
// refreshes into one. If a refresh fails but an older value exists, the
// stale value is served (and the error logged) rather than failing the whole
// dashboard - the health page shows which ledger DB is down.
func (s *DashboardService) activeLoanTotals(ctx context.Context) (*models.LoanTotals, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loanTotals != nil && time.Since(s.loanTotalAt) < loanTotalsTTL {
		return s.loanTotals, nil
	}

	ctx, cancel := context.WithTimeout(ctx, loanTotalsTimeout)
	defer cancel()

	totals, err := s.repo.SumActiveLoans(ctx)
	if err != nil {
		if s.loanTotals != nil {
			slog.Error("refreshing active loan totals failed, serving stale value", "age", time.Since(s.loanTotalAt), "error", err)
			return s.loanTotals, nil
		}
		return nil, err
	}

	s.loanTotals, s.loanTotalAt = totals, time.Now()
	return totals, nil
}
