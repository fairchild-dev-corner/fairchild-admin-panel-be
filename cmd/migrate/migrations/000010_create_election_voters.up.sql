-- Each election's voters' master list, uploaded from an Excel file on the
-- Generate Master List page. A member may vote when they're on the list and
-- their portal account is active. Only members with a portal account are
-- stored (user_id/member_no as matched at upload). An upload replaces the
-- election's whole list.
CREATE TABLE election_voters (
	id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	election_id BIGINT UNSIGNED NOT NULL,
	user_id     BIGINT UNSIGNED NOT NULL,
	member_no   VARCHAR(50)     NOT NULL,
	-- As written in the uploaded file.
	listed_name VARCHAR(255)    NOT NULL DEFAULT '',
	PRIMARY KEY (id),
	UNIQUE KEY uq_election_voters_member (election_id, user_id),
	CONSTRAINT fk_election_voters_election FOREIGN KEY (election_id) REFERENCES elections(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE elections
	ADD COLUMN master_list_file       VARCHAR(255) DEFAULT NULL AFTER published_at,
	ADD COLUMN master_list_updated_at TIMESTAMP    NULL DEFAULT NULL AFTER master_list_file,
	ADD COLUMN master_list_updated_by VARCHAR(200) DEFAULT NULL AFTER master_list_updated_at;
