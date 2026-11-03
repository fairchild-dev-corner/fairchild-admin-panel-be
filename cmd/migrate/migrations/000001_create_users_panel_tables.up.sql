-- Admin panel accounts (admins and staff). Deliberately separate from the
-- member `users` table owned by fairchild_be: panel users sign in with a
-- username + password, never a badge number, and a member account can never
-- be promoted into a panel account by editing a column.
CREATE TABLE users_panel (
	id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	uuid          CHAR(36)        NOT NULL,
	username      VARCHAR(50)     NOT NULL,
	password_hash VARCHAR(255)    NOT NULL,
	first_name    VARCHAR(100)    NOT NULL,
	last_name     VARCHAR(100)    NOT NULL,
	email         VARCHAR(255)    DEFAULT NULL,
	role          ENUM('admin','staff') NOT NULL DEFAULT 'staff',
	status        ENUM('active','suspended','deleted') NOT NULL DEFAULT 'active',
	last_login_at TIMESTAMP       NULL DEFAULT NULL,
	created_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	deleted_at    TIMESTAMP       NULL DEFAULT NULL,
	PRIMARY KEY (id),
	UNIQUE KEY uq_users_panel_uuid (uuid),
	UNIQUE KEY uq_users_panel_username (username),
	UNIQUE KEY uq_users_panel_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Refresh-token sessions for panel users. Same rotate-on-use design as
-- fairchild_be's auth_sessions: only the SHA-256 of the opaque token is stored.
CREATE TABLE users_panel_sessions (
	id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	user_panel_id      BIGINT UNSIGNED NOT NULL,
	refresh_token_hash CHAR(64)        NOT NULL,
	user_agent         VARCHAR(255)    DEFAULT NULL,
	ip_address         VARCHAR(45)     DEFAULT NULL,
	expires_at         TIMESTAMP       NOT NULL,
	revoked_at         TIMESTAMP       NULL DEFAULT NULL,
	created_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE KEY uq_users_panel_sessions_token_hash (refresh_token_hash),
	KEY idx_users_panel_sessions_user (user_panel_id),
	CONSTRAINT fk_users_panel_sessions_user FOREIGN KEY (user_panel_id) REFERENCES users_panel(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Every login attempt, successful or not. Failed rows drive the lockout
-- (per username and per IP over a rolling window - see AuthService.Login).
CREATE TABLE users_panel_login_audit (
	id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	user_panel_id      BIGINT UNSIGNED DEFAULT NULL,
	username_attempted VARCHAR(50)     NOT NULL,
	ip_address         VARCHAR(45)     DEFAULT NULL,
	user_agent         VARCHAR(255)    DEFAULT NULL,
	status             ENUM('success','failed') NOT NULL,
	reason             VARCHAR(255)    DEFAULT NULL,
	created_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_users_panel_login_audit_username (username_attempted, status, created_at),
	KEY idx_users_panel_login_audit_ip (ip_address, status, created_at),
	CONSTRAINT fk_users_panel_login_audit_user FOREIGN KEY (user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
