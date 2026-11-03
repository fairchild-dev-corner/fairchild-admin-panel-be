package models

import (
	"database/sql"
	"strings"
	"time"
)

// UserPanel mirrors the `users_panel` table - an admin or staff account for
// the admin panel. Entirely separate from fairchild_be's member `users`.
type UserPanel struct {
	ID           int64
	UUID         string
	Username     string
	PasswordHash string
	FirstName    string
	LastName     string
	Email        sql.NullString
	Role         string
	Status       string
	LastLoginAt  sql.NullTime
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DisplayName is what the panel shows as the actor on reviews and audit
// entries.
func (u *UserPanel) DisplayName() string {
	if name := strings.TrimSpace(u.FirstName + " " + u.LastName); name != "" {
		return name
	}
	return u.Username
}

// AuthSession mirrors the `users_panel_sessions` table.
type AuthSession struct {
	ID               int64
	UserPanelID      int64
	RefreshTokenHash string
	UserAgent        string
	IPAddress        string
	ExpiresAt        time.Time
}

// LoginAttempt is one `users_panel_login_audit` row.
type LoginAttempt struct {
	UserPanelID       *int64
	UsernameAttempted string
	IPAddress         string
	UserAgent         string
	Success           bool
	Reason            string
}
