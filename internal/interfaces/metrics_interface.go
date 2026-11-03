package interfaces

import (
	"context"
	"time"

	models "fairchild-admin-panel-be/internal/models/metrics"
)

type MetricsRepositoryInterface interface {
	// Monthly counts since `since`, bucketed in the cooperative's time zone.
	LoanApplicationsPerMonth(ctx context.Context, since time.Time) ([]models.MonthCount, error)
	MembershipApplicationsPerMonth(ctx context.Context, since time.Time) ([]models.MonthCount, error)
	ApprovedLoansByProduct(ctx context.Context) ([]models.LoansByProduct, error)
}
