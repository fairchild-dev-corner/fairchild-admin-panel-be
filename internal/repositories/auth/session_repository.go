package repository

import (
	"context"
	"database/sql"
	"errors"

	models "fairchild-admin-panel-be/internal/models/auth"
	"fairchild-admin-panel-be/internal/utils"
)

// CreateSession persists a new refresh session. Only the SHA-256 of the
// refresh token is stored - never the raw value.
func (r *AuthRepository) CreateSession(ctx context.Context, s *models.AuthSession) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO users_panel_sessions (user_panel_id, refresh_token_hash, user_agent, ip_address, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		s.UserPanelID, s.RefreshTokenHash,
		utils.NullIfEmpty(utils.Truncate(s.UserAgent, 255)), utils.NullIfEmpty(s.IPAddress),
		s.ExpiresAt.UTC(),
	)
	return err
}

// FindActiveSessionByTokenHash returns nil, nil for an unknown, revoked or
// expired session.
func (r *AuthRepository) FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (*models.AuthSession, error) {
	var s models.AuthSession
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, user_panel_id, refresh_token_hash, expires_at
		 FROM users_panel_sessions
		 WHERE refresh_token_hash = ? AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP()`,
		tokenHash,
	).Scan(&s.ID, &s.UserPanelID, &s.RefreshTokenHash, &s.ExpiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// RevokeSession marks the session revoked and reports whether this call was
// the one that revoked it. Refresh relies on that: two concurrent refreshes
// with the same token can both find the session active, but only one wins
// the revoke, so a refresh token can never be redeemed twice.
func (r *AuthRepository) RevokeSession(ctx context.Context, tokenHash string) (bool, error) {
	res, err := r.DB.ExecContext(ctx,
		`UPDATE users_panel_sessions SET revoked_at = UTC_TIMESTAMP()
		 WHERE refresh_token_hash = ? AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
