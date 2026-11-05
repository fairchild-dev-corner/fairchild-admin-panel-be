package interfaces

import (
	"context"

	models "fairchild-admin-panel-be/internal/models/account"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
)

// AccountRepositoryInterface covers one panel user's own account data. The
// only writes are their own profile and password.
type AccountRepositoryInterface interface {
	// Returns nil, nil for an unknown or deleted user.
	FindAccount(ctx context.Context, id int64) (*models.Account, error)
	ListActiveSessions(ctx context.Context, id int64) ([]models.AccountSession, error)
	ListSignIns(ctx context.Context, id int64, username string, limit int) ([]models.AccountSignIn, error)
	ListActivity(ctx context.Context, id int64, limit int) ([]models.AccountActivity, int, error)
	// cc.ErrEmailAlreadyExists if another panel account uses the email.
	UpdateProfile(ctx context.Context, id int64, firstName, lastName, email string, audit *audit_models.AuditLog) error
	// Also revokes every refresh session.
	ChangePassword(ctx context.Context, id int64, passwordHash string, audit *audit_models.AuditLog) error
}
