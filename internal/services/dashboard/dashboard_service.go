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
	// The ledger aggregate scans every branch's sldtl (~5s for the largest
	// branch) - far too heavy to run on a dashboard load, and loan balances
	// only move when branches post, so a few minutes of staleness is fine.
	loanTotalsTTL     = 10 * time.Minute
	loanTotalsRetry   = time.Minute
	loanTotalsTimeout = time.Minute
)

type DashboardService struct {
	repo interfaces.DashboardRepositoryInterface

	mu          sync.RWMutex
	loanTotals  *models.LoanTotals
	loanTotalAt time.Time
	loanErr     error
	// loanReady is closed once the first refresh has finished.
	loanReady chan struct{}
}

// NewDashboardService starts refreshing the active-loan totals in the
// background right away, so they're ready before the first dashboard load
// and no request ever waits on the ledger scan.
func NewDashboardService(repo interfaces.DashboardRepositoryInterface) *DashboardService {
	s := &DashboardService{repo: repo, loanReady: make(chan struct{})}
	go s.refreshLoanTotalsLoop()
	return s
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
		PendingAccountClaims:          central.PendingAccountClaims,
		ActiveLoans:                   loans.Count,
		ActiveLoanBalance:             math.Round(loans.Balance*100) / 100,
		MaintenanceMode:               central.MaintenanceMode,
	}, nil
}

// activeLoanTotals returns the latest background-computed totals. Only a
// request that arrives before the first refresh after startup waits, and
// only for that refresh.
func (s *DashboardService) activeLoanTotals(ctx context.Context) (*models.LoanTotals, error) {
	select {
	case <-s.loanReady:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.loanTotals != nil {
		return s.loanTotals, nil
	}
	return nil, s.loanErr
}

// refreshLoanTotalsLoop recomputes the totals every loanTotalsTTL, or after
// loanTotalsRetry when a refresh fails. A failed refresh keeps serving the
// previous value (the health page shows which ledger DB is down).
func (s *DashboardService) refreshLoanTotalsLoop() {
	for first := true; ; first = false {
		ok := s.refreshLoanTotals()
		if first {
			close(s.loanReady)
		}
		if ok {
			time.Sleep(loanTotalsTTL)
		} else {
			time.Sleep(loanTotalsRetry)
		}
	}
}

func (s *DashboardService) refreshLoanTotals() bool {
	ctx, cancel := context.WithTimeout(context.Background(), loanTotalsTimeout)
	defer cancel()

	started := time.Now()
	totals, err := s.repo.SumActiveLoans(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.loanErr = err
		if s.loanTotals != nil {
			slog.Error("refreshing active loan totals failed, serving stale value", "age", time.Since(s.loanTotalAt), "error", err)
		} else {
			slog.Error("computing active loan totals failed", "error", err)
		}
		return false
	}
	s.loanTotals, s.loanTotalAt, s.loanErr = totals, time.Now(), nil
	slog.Info("active loan totals refreshed", "took", time.Since(started).Round(time.Millisecond))
	return true
}
