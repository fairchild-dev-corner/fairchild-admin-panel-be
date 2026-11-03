package interfaces

import (
	"context"
	"time"

	models "fairchild-admin-panel-be/internal/models/dashboard"
)

type DashboardRepositoryInterface interface {
	CountCentral(ctx context.Context, monthStart time.Time) (*models.CentralCounts, error)
	// SumActiveLoans aggregates across every branch ledger DB.
	SumActiveLoans(ctx context.Context) (*models.LoanTotals, error)
}
