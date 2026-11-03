-- Single-row table holding the public website's maintenance flag. Read by
-- the public GET /api/v1/site/status (fairchild-fe's maintenance banner).
CREATE TABLE site_status (
	id                    TINYINT UNSIGNED NOT NULL DEFAULT 1,
	maintenance_mode      BOOLEAN          NOT NULL DEFAULT FALSE,
	message               VARCHAR(500)     NOT NULL DEFAULT '',
	updated_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	updated_at            TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	CONSTRAINT chk_site_status_singleton CHECK (id = 1),
	CONSTRAINT fk_site_status_user FOREIGN KEY (updated_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO site_status (id, maintenance_mode, message)
VALUES (1, FALSE, 'We''re performing scheduled maintenance. Please check back shortly.');
