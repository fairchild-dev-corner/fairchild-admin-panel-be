package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/site"
)

type SiteRepositoryInterface interface {
	GetSiteStatus(ctx context.Context) (*models.SiteStatus, error)
	// UpdateSiteStatus writes the new state; audit is called with the state
	// being replaced (read under a row lock) to build the audit entry.
	UpdateSiteStatus(ctx context.Context, maintenanceMode bool, message string, updatedBy int64, audit func(previous *models.SiteStatus) *audit_models.AuditLog) error
}
