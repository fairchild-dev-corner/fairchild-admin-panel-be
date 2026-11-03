-- Online loan applications awaiting staff review. Nothing writes here yet:
-- fairchild_be has no member-facing "apply for a loan" endpoint, so until it
-- does, rows come from that future endpoint (or are inserted by staff).
-- member_id/member_name are snapshots taken at submission time, since the
-- member's ledger record lives in a branch DB we never join against.
CREATE TABLE loan_applications (
	id                        BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
	user_id                   BIGINT UNSIGNED  DEFAULT NULL,
	member_id                 VARCHAR(50)      NOT NULL,
	member_name               VARCHAR(200)     NOT NULL,
	product                   VARCHAR(60)      NOT NULL,
	amount                    DECIMAL(14,2)    NOT NULL,
	term_months               SMALLINT UNSIGNED NOT NULL,
	purpose                   VARCHAR(500)     NOT NULL,
	status                    ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending',
	review_note               VARCHAR(500)     DEFAULT NULL,
	reviewed_by_user_panel_id BIGINT UNSIGNED  DEFAULT NULL,
	reviewed_at               TIMESTAMP        NULL DEFAULT NULL,
	created_at                TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at                TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_loan_applications_status (status),
	KEY idx_loan_applications_created_at (created_at),
	KEY idx_loan_applications_user (user_id),
	CONSTRAINT fk_loan_applications_reviewer FOREIGN KEY (reviewed_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
