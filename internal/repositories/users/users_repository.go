package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	audit_models "fairchild-admin-panel-be/internal/models/audit"
	auth_models "fairchild-admin-panel-be/internal/models/auth"
	models "fairchild-admin-panel-be/internal/models/users"
	audit_repo "fairchild-admin-panel-be/internal/repositories/audit"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/go-sql-driver/mysql"
)

/**
* UsersRepository
* @Description: Every `users_panel` account, for the Users page. Writes:
* adding an account (with its first `panel_user_capabilities`), changing
* its role, and suspending or reactivating it (suspending revokes its
* `users_panel_sessions`). Each is audited in the same transaction.
**/
type UsersRepository struct {
	DB *sql.DB
}

func NewUsersRepository(db *sql.DB) *UsersRepository {
	return &UsersRepository{DB: db}
}

const userSelect = `
SELECT id, uuid, username, first_name, last_name, COALESCE(email, ''), role, status, last_login_at, created_at
FROM users_panel`

const notDeleted = `deleted_at IS NULL AND status <> 'deleted'`

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func scanUser(row interface{ Scan(...any) error }) (*models.PanelUser, error) {
	var u models.PanelUser
	var lastLogin sql.NullTime
	var created time.Time
	if err := row.Scan(&u.InternalID, &u.ID, &u.Username, &u.FirstName, &u.LastName, &u.Email, &u.Role, &u.Status, &lastLogin, &created); err != nil {
		return nil, err
	}
	u.CreatedAt = rfc3339(created)
	if lastLogin.Valid {
		u.LastLoginAt = rfc3339(lastLogin.Time)
	}
	return &u, nil
}

// ListUsers returns the highest roles first, each by name.
func (r *UsersRepository) ListUsers(ctx context.Context) ([]models.PanelUser, error) {
	rows, err := r.DB.QueryContext(ctx, userSelect+`
		WHERE `+notDeleted+`
		ORDER BY FIELD(role, 'super_admin', 'admin', 'manager', 'supervisor', 'staff'), first_name, last_name, username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.PanelUser{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (r *UsersRepository) FindUser(ctx context.Context, id string) (*models.PanelUser, error) {
	u, err := scanUser(r.DB.QueryRowContext(ctx, userSelect+` WHERE uuid = ? AND `+notDeleted, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, cc.ErrPanelUserNotFound
	}
	return u, err
}

func findByIDTx(ctx context.Context, tx *sql.Tx, id int64) (*models.PanelUser, error) {
	return scanUser(tx.QueryRowContext(ctx, userSelect+` WHERE id = ?`, id))
}

func (r *UsersRepository) CreateUser(ctx context.Context, u *auth_models.UserPanel, capabilities []string, audit *audit_models.AuditLog) (*models.PanelUser, error) {
	var created *models.PanelUser
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO users_panel (uuid, username, password_hash, first_name, last_name, email, role, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			u.UUID, u.Username, u.PasswordHash, u.FirstName, u.LastName, u.Email, u.Role, u.Status,
		)
		if err != nil {
			var myErr *mysql.MySQLError
			if errors.As(err, &myErr) && myErr.Number == 1062 {
				if strings.Contains(myErr.Message, "uq_users_panel_email") {
					return cc.ErrEmailAlreadyExists
				}
				return cc.ErrUsernameAlreadyExists
			}
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, key := range capabilities {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO panel_user_capabilities (user_panel_id, capability, granted_by) VALUES (?, ?, ?)`,
				id, key, audit.UserPanelID); err != nil {
				return err
			}
		}
		if created, err = findByIDTx(ctx, tx, id); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return created, err
}

func (r *UsersRepository) UpdateRole(ctx context.Context, userPanelID int64, role string, audit *audit_models.AuditLog) (*models.PanelUser, error) {
	var updated *models.PanelUser
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users_panel SET role = ? WHERE id = ?`, role, userPanelID); err != nil {
			return err
		}
		var err error
		if updated, err = findByIDTx(ctx, tx, userPanelID); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return updated, err
}

func (r *UsersRepository) UpdateStatus(ctx context.Context, userPanelID int64, status string, audit *audit_models.AuditLog) (*models.PanelUser, error) {
	var updated *models.PanelUser
	err := database.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users_panel SET status = ? WHERE id = ?`, status, userPanelID); err != nil {
			return err
		}
		if status == cc.USER_STATUS_SUSPENDED {
			if _, err := tx.ExecContext(ctx,
				`UPDATE users_panel_sessions SET revoked_at = UTC_TIMESTAMP() WHERE user_panel_id = ? AND revoked_at IS NULL`,
				userPanelID); err != nil {
				return err
			}
		}
		var err error
		if updated, err = findByIDTx(ctx, tx, userPanelID); err != nil {
			return err
		}
		return audit_repo.InsertAuditLogTx(ctx, tx, audit)
	})
	return updated, err
}
