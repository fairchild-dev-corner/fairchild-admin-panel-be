-- What staff accounts may see and do in the admin panel, set by an admin on
-- System > Capabilities. One row per capability an admin has saved; a
-- capability with no row uses its default from the catalog in
-- internal/models/capabilities. Admins always have every capability.
CREATE TABLE panel_staff_capabilities (
	capability VARCHAR(64)     NOT NULL,
	allowed    BOOLEAN         NOT NULL,
	updated_by BIGINT UNSIGNED DEFAULT NULL,
	updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (capability),
	CONSTRAINT fk_panel_staff_capabilities_user FOREIGN KEY (updated_by) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
