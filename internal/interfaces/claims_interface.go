package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/claims"
)

// ClaimsRepositoryInterface covers fairchild_be's `account_claim_requests`.
// The only writes are a review - approve (which also updates the matching
// portal account's email/mobile) or reject - and deleting a reviewed claim.
type ClaimsRepositoryInterface interface {
	// Claims per status matching q's search and date range (q.Status is
	// ignored; every status present, zero or not).
	CountByStatus(ctx context.Context, q models.ClaimListQuery) (map[string]int, error)
	List(ctx context.Context, q models.ClaimListQuery) ([]models.ClaimRequest, error)
	// Returns nil, nil for an unknown id.
	FindByID(ctx context.Context, id int64) (*models.ClaimRequest, error)
	Approve(ctx context.Context, id int64, email, mobile string, audit *audit_models.AuditLog) error
	Reject(ctx context.Context, id int64, reason string, audit *audit_models.AuditLog) error
	// cc.ErrClaimStillPending for a pending claim.
	Delete(ctx context.Context, id int64, audit *audit_models.AuditLog) error
}
