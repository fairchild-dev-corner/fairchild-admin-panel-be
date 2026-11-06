package interfaces

import (
	"context"
	"time"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/site"
)

type SiteRepositoryInterface interface {
	GetSiteStatus(ctx context.Context) (*models.SiteStatus, error)
	// UpdateSiteStatus writes the new state; audit is called with the state
	// being replaced (read under a row lock) to build the audit entry.
	// startsAt (nil = now) and durationMinutes follow
	// UpdateSiteStatusRequest. When the save puts maintenance in force,
	// every member session (fairchild_be's auth_sessions) is revoked in the
	// same transaction and the count is returned; otherwise it's 0.
	UpdateSiteStatus(ctx context.Context, maintenanceMode bool, message string, startsAt *time.Time, durationMinutes *int, updatedBy int64, audit func(previous *models.SiteStatus) *audit_models.AuditLog) (int64, error)
	// SignOutIfDue revokes every member session once a scheduled start has
	// passed - once per maintenance window, however many instances call it.
	// Returns how many were signed out.
	SignOutIfDue(ctx context.Context) (int64, error)
}
