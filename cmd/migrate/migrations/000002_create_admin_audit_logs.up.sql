-- One row per admin mutation (application reviews, maintenance toggles, ...).
-- Written in the same transaction as the mutation itself. actor_name is a
-- snapshot so the log still reads correctly after a panel user is renamed or
-- deleted.
CREATE TABLE admin_audit_logs (
	id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	actor_name    VARCHAR(200)    NOT NULL,
	action        VARCHAR(150)    NOT NULL,
	target        VARCHAR(100)    NOT NULL,
	ip_address    VARCHAR(45)     DEFAULT NULL,
	created_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_admin_audit_logs_created_at (created_at),
	KEY idx_admin_audit_logs_user (user_panel_id),
	CONSTRAINT fk_admin_audit_logs_user FOREIGN KEY (user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
