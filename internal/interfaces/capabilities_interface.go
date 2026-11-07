package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
)

// CapabilitiesRepositoryInterface covers `panel_user_capabilities`: what each
// manager, supervisor and staff account may do.
type CapabilitiesRepositoryInterface interface {
	// The capabilities userPanelID has, in no particular order.
	FindUserCapabilities(ctx context.Context, userPanelID int64) ([]string, error)
	// Grants add and revokes remove for userPanelID, with one audit entry.
	SaveUserCapabilities(ctx context.Context, userPanelID int64, add, remove []string, grantedBy int64, audit *audit_models.AuditLog) error
}
