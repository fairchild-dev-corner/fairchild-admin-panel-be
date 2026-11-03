package interfaces

import (
	"context"
	"time"

	models "fairchild-admin-panel-be/internal/models/auth"
)

type AuthRepositoryInterface interface {
	// users_panel
	FindUserByUsername(ctx context.Context, username string) (*models.UserPanel, error)
	FindUserByID(ctx context.Context, id int64) (*models.UserPanel, error)
	CreateUser(ctx context.Context, u *models.UserPanel) (int64, error)
	TouchLastLogin(ctx context.Context, id int64) error

	// users_panel_sessions
	CreateSession(ctx context.Context, s *models.AuthSession) error
	FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (*models.AuthSession, error)
	RevokeSession(ctx context.Context, tokenHash string) (bool, error)

	// users_panel_login_audit
	RecordLoginAttempt(ctx context.Context, a *models.LoginAttempt) error
	CountRecentFailedLogins(ctx context.Context, username, ip string, window time.Duration) (byUsername, byIP int, err error)
}
