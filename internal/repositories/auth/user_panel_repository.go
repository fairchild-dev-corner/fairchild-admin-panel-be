package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/auth"

	"github.com/go-sql-driver/mysql"
)

const userPanelColumns = `id, uuid, username, password_hash, first_name, last_name, email,
	role, status, last_login_at, created_at, updated_at`

func scanUserPanel(row interface{ Scan(...any) error }) (*models.UserPanel, error) {
	var u models.UserPanel
	err := row.Scan(
		&u.ID, &u.UUID, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Email,
		&u.Role, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindUserByUsername returns nil, nil when no non-deleted account matches.
// The column's utf8mb4_general_ci collation makes the match case-insensitive.
func (r *AuthRepository) FindUserByUsername(ctx context.Context, username string) (*models.UserPanel, error) {
	return scanUserPanel(r.DB.QueryRowContext(ctx,
		`SELECT `+userPanelColumns+` FROM users_panel WHERE username = ? AND deleted_at IS NULL`,
		username,
	))
}

// FindUserByID returns nil, nil when no non-deleted account matches.
func (r *AuthRepository) FindUserByID(ctx context.Context, id int64) (*models.UserPanel, error) {
	return scanUserPanel(r.DB.QueryRowContext(ctx,
		`SELECT `+userPanelColumns+` FROM users_panel WHERE id = ? AND deleted_at IS NULL`,
		id,
	))
}

// CreateUser inserts a users_panel row (used by cmd/create_panel_user),
// mapping unique-key violations onto the matching sentinel error.
func (r *AuthRepository) CreateUser(ctx context.Context, u *models.UserPanel) (int64, error) {
	res, err := r.DB.ExecContext(ctx,
		`INSERT INTO users_panel (uuid, username, password_hash, first_name, last_name, email, role, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.UUID, u.Username, u.PasswordHash, u.FirstName, u.LastName, u.Email, u.Role, u.Status,
	)
	if err != nil {
		var myErr *mysql.MySQLError
		if errors.As(err, &myErr) && myErr.Number == 1062 {
			if strings.Contains(myErr.Message, "uq_users_panel_email") {
				return 0, cc.ErrEmailAlreadyExists
			}
			return 0, cc.ErrUsernameAlreadyExists
		}
		return 0, err
	}
	return res.LastInsertId()
}

func (r *AuthRepository) TouchLastLogin(ctx context.Context, id int64) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE users_panel SET last_login_at = NOW() WHERE id = ?`, id)
	return err
}
