package interfaces

import (
	"context"

	models "fairchild-admin-panel-be/internal/models/applications"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
)

type ApplicationsRepositoryInterface interface {
	// status "" means all statuses.
	ListLoanApplications(ctx context.Context, status string) ([]models.LoanApplication, error)
	FindLoanApplicationByID(ctx context.Context, id int64) (*models.LoanApplication, error)
	ReviewLoanApplication(ctx context.Context, id int64, decision *models.ReviewDecisionRequest, reviewerID int64, audit *audit_models.AuditLog) error

	// search "" means no search; otherwise it matches name, email, mobile or id.
	ListMembershipApplications(ctx context.Context, status, search string) ([]models.MembershipApplication, error)
	FindMembershipApplicationByID(ctx context.Context, id int64) (*models.MembershipApplication, error)
	// Returns the new row id; sets audit.Target once it's known.
	CreateMembershipApplication(ctx context.Context, req *models.NewMembershipApplicationRequest, audit *audit_models.AuditLog) (int64, error)
	ReviewMembershipApplication(ctx context.Context, id int64, decision *models.ReviewDecisionRequest, reviewerID int64, audit *audit_models.AuditLog) error
}
