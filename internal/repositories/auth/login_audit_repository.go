package repository

import (
	"context"
	"time"

	models "fairchild-admin-panel-be/internal/models/auth"
	"fairchild-admin-panel-be/internal/utils"
)

func (r *AuthRepository) RecordLoginAttempt(ctx context.Context, a *models.LoginAttempt) error {
	status := "failed"
	if a.Success {
		status = "success"
	}

	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO users_panel_login_audit (user_panel_id, username_attempted, ip_address, user_agent, status, reason)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.UserPanelID, utils.Truncate(a.UsernameAttempted, 50),
		utils.NullIfEmpty(a.IPAddress), utils.NullIfEmpty(utils.Truncate(a.UserAgent, 255)),
		status, utils.NullIfEmpty(a.Reason),
	)
	return err
}

// CountRecentFailedLogins counts failed attempts in the rolling window for
// the username and, separately, for the source IP. The window is computed in
// SQL against the DB clock so app/DB clock skew can't widen or shrink it.
func (r *AuthRepository) CountRecentFailedLogins(ctx context.Context, username, ip string, window time.Duration) (int, int, error) {
	seconds := int64(window.Seconds())

	var byUsername, byIP int
	err := r.DB.QueryRowContext(ctx,
		`SELECT
			(SELECT COUNT(*) FROM users_panel_login_audit
			 WHERE username_attempted = ? AND status = 'failed'
			   AND created_at > NOW() - INTERVAL ? SECOND),
			(SELECT COUNT(*) FROM users_panel_login_audit
			 WHERE ip_address = ? AND status = 'failed'
			   AND created_at > NOW() - INTERVAL ? SECOND)`,
		username, seconds, ip, seconds,
	).Scan(&byUsername, &byIP)

	return byUsername, byIP, err
}
