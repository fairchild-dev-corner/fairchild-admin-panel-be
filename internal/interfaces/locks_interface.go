package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/locks"
)

// LocksRepositoryInterface covers fairchild_be's `locked_member_accounts`.
// The only write is an unlock.
type LocksRepositoryInterface interface {
	// Locks per status matching search and the date range (both statuses
	// present, zero or not).
	CountByStatus(ctx context.Context, q models.LockedAccountListQuery) (map[string]int, error)
	List(ctx context.Context, q models.LockedAccountListQuery) ([]models.LockedAccount, error)
	// Returns nil, nil for an unknown id.
	FindByID(ctx context.Context, id int64) (*models.LockedAccount, error)
	Unlock(ctx context.Context, id int64, audit *audit_models.AuditLog) error
}
