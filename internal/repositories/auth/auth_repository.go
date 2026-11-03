package repository

import (
	"database/sql"
)

/**
* AuthRepository
* @Description: Wraps the central Auth DB pool for panel accounts
* (users_panel), their refresh sessions (users_panel_sessions) and login
* attempts (users_panel_login_audit).
**/
type AuthRepository struct {
	DB *sql.DB
}

func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{DB: db}
}
