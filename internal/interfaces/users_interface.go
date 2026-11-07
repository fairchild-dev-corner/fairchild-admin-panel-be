package interfaces

import (
	"context"

	audit_models "fairchild-admin-panel-be/internal/models/audit"
	auth_models "fairchild-admin-panel-be/internal/models/auth"
	models "fairchild-admin-panel-be/internal/models/users"
)

// UsersRepositoryInterface covers every `users_panel` account, for admins
// managing who can sign in to the panel and at what role.
type UsersRepositoryInterface interface {
	// Every non-deleted account.
	ListUsers(ctx context.Context) ([]models.PanelUser, error)
	// The non-deleted account with this uuid; cc.ErrPanelUserNotFound if none.
	FindUser(ctx context.Context, id string) (*models.PanelUser, error)
	// Adds the account with its first capabilities. cc.ErrUsernameAlreadyExists
	// / cc.ErrEmailAlreadyExists on a clash.
	CreateUser(ctx context.Context, u *auth_models.UserPanel, capabilities []string, audit *audit_models.AuditLog) (*models.PanelUser, error)
	UpdateRole(ctx context.Context, userPanelID int64, role string, audit *audit_models.AuditLog) (*models.PanelUser, error)
	// Suspending also revokes the account's sessions.
	UpdateStatus(ctx context.Context, userPanelID int64, status string, audit *audit_models.AuditLog) (*models.PanelUser, error)
}
