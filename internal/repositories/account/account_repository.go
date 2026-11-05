package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/account"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/internal/utils"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/go-sql-driver/mysql"
)

/**
* AccountRepository
* @Description: One panel user's own data: their
* users_panel row, live users_panel_sessions, users_panel_login_audit
* attempts and the admin_audit_logs rows they wrote. The only writes are
* the user changing their own name/email and password, each audited in the
* same transaction.
**/
type AccountRepository struct {
	DB *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{DB: db}
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// FindAccount returns nil, nil when no non-deleted user has that id.
func (r *AccountRepository) FindAccount(ctx context.Context, id int64) (*models.Account, error) {
	var a models.Account
	var email sql.NullString
	var created, updated time.Time
	var lastLogin sql.NullTime
	err := r.DB.QueryRowContext(ctx,
		`SELECT uuid, username, first_name, last_name, email, role, status, created_at, updated_at, last_login_at
		 FROM users_panel WHERE id = ? AND status <> 'deleted' AND deleted_at IS NULL`,
		id,
	).Scan(&a.ID, &a.Username, &a.FirstName, &a.LastName, &email, &a.Role, &a.Status, &created, &updated, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Email = email.String
	a.CreatedAt, a.UpdatedAt = rfc3339(created), rfc3339(updated)
	if lastLogin.Valid {
		a.LastLoginAt = rfc3339(lastLogin.Time)
	}
	return &a, nil
}

// ListActiveSessions returns the user's refresh sessions that are neither
// revoked nor expired, newest first.
func (r *AccountRepository) ListActiveSessions(ctx context.Context, id int64) ([]models.AccountSession, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT COALESCE(ip_address, ''), COALESCE(user_agent, ''), created_at, expires_at
		 FROM users_panel_sessions
		 WHERE user_panel_id = ? AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP()
		 ORDER BY created_at DESC, id DESC
		 LIMIT 20`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []models.AccountSession{}
	for rows.Next() {
		var s models.AccountSession
		var created, expires time.Time
		if err := rows.Scan(&s.IPAddress, &s.UserAgent, &created, &expires); err != nil {
			return nil, err
		}
		s.CreatedAt, s.ExpiresAt = rfc3339(created), rfc3339(expires)
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// ListSignIns returns the latest sign-in attempts on this account: rows
// linked to the user, plus failed ones on their username (those aren't
// always linked).
func (r *AccountRepository) ListSignIns(ctx context.Context, id int64, username string, limit int) ([]models.AccountSignIn, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT status = 'success', COALESCE(reason, ''), COALESCE(ip_address, ''), COALESCE(user_agent, ''), created_at
		 FROM users_panel_login_audit
		 WHERE user_panel_id = ? OR username_attempted = ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`,
		id, username, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	signIns := []models.AccountSignIn{}
	for rows.Next() {
		var s models.AccountSignIn
		var at time.Time
		if err := rows.Scan(&s.Success, &s.Reason, &s.IPAddress, &s.UserAgent, &at); err != nil {
			return nil, err
		}
		s.At = rfc3339(at)
		signIns = append(signIns, s)
	}
	return signIns, rows.Err()
}

// ListActivity returns the latest audit entries this user wrote and how many
// there are in all.
func (r *AccountRepository) ListActivity(ctx context.Context, id int64, limit int) ([]models.AccountActivity, int, error) {
	var total int
	if err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_audit_logs WHERE user_panel_id = ?`, id,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, action, target, created_at
		 FROM admin_audit_logs
		 WHERE user_panel_id = ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`,
		id, limit,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	activity := []models.AccountActivity{}
	for rows.Next() {
		var a models.AccountActivity
		var rowID int64
		var at time.Time
		if err := rows.Scan(&rowID, &a.Action, &a.Target, &at); err != nil {
			return nil, 0, err
		}
		a.ID, a.At = strconv.FormatInt(rowID, 10), rfc3339(at)
		activity = append(activity, a)
	}
	return activity, total, rows.Err()
}

// UpdateProfile saves the user's own name and email (empty = none) with its
// audit entry. cc.ErrEmailAlreadyExists if another panel account uses it.
func (r *AccountRepository) UpdateProfile(ctx context.Context, id int64, firstName, lastName, email string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE users_panel SET first_name = ?, last_name = ?, email = ? WHERE id = ? AND deleted_at IS NULL`,
			firstName, lastName, utils.NullIfEmpty(email), id); err != nil {
			var myErr *mysql.MySQLError
			if errors.As(err, &myErr) && myErr.Number == 1062 {
				return cc.ErrEmailAlreadyExists
			}
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}

// ChangePassword stores the new hash and signs the user out everywhere
// (every refresh session revoked), with its audit entry.
func (r *AccountRepository) ChangePassword(ctx context.Context, id int64, passwordHash string, audit *audit_models.AuditLog) error {
	return database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE users_panel SET password_hash = ? WHERE id = ? AND deleted_at IS NULL`, passwordHash, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users_panel_sessions SET revoked_at = UTC_TIMESTAMP() WHERE user_panel_id = ? AND revoked_at IS NULL`,
			id); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
}
