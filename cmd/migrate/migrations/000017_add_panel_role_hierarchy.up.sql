-- Panel roles become a hierarchy: super_admin > admin > manager > supervisor
-- > staff. Super admins and admins can do everything; managers, supervisors
-- and staff get only the capabilities an admin assigns to their account, so
-- two staff can be allowed different things.
ALTER TABLE users_panel
	MODIFY role ENUM('super_admin','admin','manager','supervisor','staff') NOT NULL DEFAULT 'staff';

-- Today's admins can add other admins, which is a super admin's job now, so
-- they keep that by becoming super admins. They can then demote each other.
UPDATE users_panel SET role = 'super_admin' WHERE role = 'admin';

-- One row per capability a manager, supervisor or staff account has (keys
-- from internal/models/capabilities). No row: not allowed.
-- IF NOT EXISTS / INSERT IGNORE: a first attempt died on the INSERT below
-- after this table was created, so the migration must re-run cleanly.
CREATE TABLE IF NOT EXISTS panel_user_capabilities (
	user_panel_id BIGINT UNSIGNED NOT NULL,
	capability    VARCHAR(64)     NOT NULL,
	granted_by    BIGINT UNSIGNED DEFAULT NULL,
	granted_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (user_panel_id, capability),
	CONSTRAINT fk_panel_user_capabilities_user FOREIGN KEY (user_panel_id) REFERENCES users_panel(id) ON DELETE CASCADE,
	CONSTRAINT fk_panel_user_capabilities_granted_by FOREIGN KEY (granted_by) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Existing staff keep exactly what the role-wide staff settings
-- (panel_staff_capabilities) gave them: a saved setting, else the default
-- staff had before this migration.
INSERT IGNORE INTO panel_user_capabilities (user_panel_id, capability, granted_by, granted_at)
SELECT u.id, d.capability, NULL, NOW()
FROM users_panel u
CROSS JOIN (
	SELECT 'loans.view' AS capability, TRUE AS staff_default
	UNION ALL SELECT 'loans.review', TRUE
	UNION ALL SELECT 'memberships.view', TRUE
	UNION ALL SELECT 'memberships.add', TRUE
	UNION ALL SELECT 'memberships.review', TRUE
	UNION ALL SELECT 'members.view', TRUE
	UNION ALL SELECT 'members.edit', TRUE
	UNION ALL SELECT 'members.delete', FALSE
	UNION ALL SELECT 'portal.view', TRUE
	UNION ALL SELECT 'portal.manage', FALSE
	UNION ALL SELECT 'claims.view', TRUE
	UNION ALL SELECT 'claims.review', FALSE
	UNION ALL SELECT 'locks.view', TRUE
	UNION ALL SELECT 'locks.unlock', FALSE
	UNION ALL SELECT 'contact.view', TRUE
	UNION ALL SELECT 'contact.reply', FALSE
	UNION ALL SELECT 'blasts.view', TRUE
	UNION ALL SELECT 'blasts.send', FALSE
	UNION ALL SELECT 'election.controller.view', TRUE
	UNION ALL SELECT 'election.control', FALSE
	UNION ALL SELECT 'election.members.view', TRUE
	UNION ALL SELECT 'election.masterlist.view', TRUE
	UNION ALL SELECT 'election.masterlist.manage', FALSE
	UNION ALL SELECT 'maintenance.view', TRUE
	UNION ALL SELECT 'maintenance.manage', FALSE
	UNION ALL SELECT 'metrics.view', TRUE
	UNION ALL SELECT 'audit.view', TRUE
) d
-- The literals above take the session collation (utf8mb4_general_ci, see
-- pkg/database) while the table uses the server default; without an explicit
-- COLLATE MySQL rejects the comparison as an illegal mix of collations.
LEFT JOIN panel_staff_capabilities s ON s.capability = d.capability COLLATE utf8mb4_general_ci
WHERE u.role = 'staff' AND COALESCE(s.allowed, d.staff_default);

DROP TABLE IF EXISTS panel_staff_capabilities;
