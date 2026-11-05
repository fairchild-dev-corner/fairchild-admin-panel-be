-- One voters' master list, shared by every election (replaces the
-- per-election election_voters from 000010). Uploaded from an Excel file on
-- the Generate Master List page; each upload replaces the whole list. A
-- member may vote when they're on it and their portal account is active.
-- Only members with a portal account are stored (user_id/member_no as
-- matched at upload). No FK to fairchild_be's `users` on purpose.
CREATE TABLE voter_master_list (
	id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	user_id     BIGINT UNSIGNED NOT NULL,
	member_no   VARCHAR(50)     NOT NULL,
	-- As written in the uploaded file.
	listed_name VARCHAR(255)    NOT NULL DEFAULT '',
	PRIMARY KEY (id),
	UNIQUE KEY uq_voter_master_list_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Every upload, newest last; the newest describes the current list.
CREATE TABLE voter_master_list_uploads (
	id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	file_name                VARCHAR(255)    DEFAULT NULL,
	member_count             INT UNSIGNED    NOT NULL,
	uploaded_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	uploaded_by_name         VARCHAR(200)    NOT NULL,
	uploaded_at              TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	CONSTRAINT fk_voter_master_list_uploads_by FOREIGN KEY (uploaded_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

DROP TABLE IF EXISTS election_voters;

ALTER TABLE elections
	DROP COLUMN master_list_updated_by,
	DROP COLUMN master_list_updated_at,
	DROP COLUMN master_list_file;
