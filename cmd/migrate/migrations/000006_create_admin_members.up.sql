-- Cooperative members managed from the panel's Members page. Owned by this
-- service: branch ledger DBs are read-only and fairchild_be's `users` only
-- holds portal accounts, so neither can back add/edit/delete.
-- Display ids are "FC" + id (FC20001), hence the AUTO_INCREMENT start.
-- Deletes are soft (deleted_at) so audit-log targets keep pointing at a row
-- and ids are never reused.
CREATE TABLE admin_members (
	id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	membership_type          ENUM('coop','laboratory') NOT NULL,
	category                 ENUM('regular','associate','kiddie','aflatoun') NOT NULL,
	branch                   ENUM('main','knowles','tiic') NOT NULL,
	first_name               VARCHAR(100)    NOT NULL,
	last_name                VARCHAR(100)    NOT NULL,
	middle_name              VARCHAR(100)    DEFAULT NULL,
	suffix                   VARCHAR(20)     DEFAULT NULL,
	date_of_birth            DATE            NOT NULL,
	gender                   VARCHAR(20)     DEFAULT NULL,
	nationality              VARCHAR(60)     DEFAULT NULL,
	email                    VARCHAR(255)    DEFAULT NULL,
	mobile_number            VARCHAR(20)     DEFAULT NULL,
	address                  VARCHAR(500)    DEFAULT NULL,
	guardian_full_name       VARCHAR(200)    DEFAULT NULL,
	guardian_mobile_number   VARCHAR(20)     DEFAULT NULL,
	status                   ENUM('active','inactive') NOT NULL DEFAULT 'active',
	joined_at                DATE            NOT NULL,
	created_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	updated_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	created_at               TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at               TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	deleted_at               TIMESTAMP       NULL DEFAULT NULL,
	PRIMARY KEY (id),
	KEY idx_admin_members_category (deleted_at, category),
	KEY idx_admin_members_branch (branch),
	KEY idx_admin_members_last_name (last_name),
	CONSTRAINT fk_admin_members_created_by FOREIGN KEY (created_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL,
	CONSTRAINT fk_admin_members_updated_by FOREIGN KEY (updated_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=20001 DEFAULT CHARSET=utf8mb4;
