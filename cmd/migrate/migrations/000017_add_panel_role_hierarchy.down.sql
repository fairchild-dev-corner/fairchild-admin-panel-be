-- Back to admin + staff with one role-wide staff setting. Per-account
-- capabilities are lost; staff fall back to the defaults.
CREATE TABLE panel_staff_capabilities (
	capability VARCHAR(64)     NOT NULL,
	allowed    BOOLEAN         NOT NULL,
	updated_by BIGINT UNSIGNED DEFAULT NULL,
	updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (capability),
	CONSTRAINT fk_panel_staff_capabilities_user FOREIGN KEY (updated_by) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

DROP TABLE IF EXISTS panel_user_capabilities;

UPDATE users_panel SET role = 'admin' WHERE role = 'super_admin';
UPDATE users_panel SET role = 'staff' WHERE role IN ('manager', 'supervisor');

ALTER TABLE users_panel
	MODIFY role ENUM('admin','staff') NOT NULL DEFAULT 'staff';
