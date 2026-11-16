-- Staff notes on a loan application (Loans > Loan Applications > Notes):
-- a comment thread, oldest first. author_name is a snapshot so a note still
-- reads correctly after its author is renamed or removed. body is the text
-- as written, with **bold**, *italic* and __underline__ markers.
CREATE TABLE loan_application_notes (
	id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	loan_application_id BIGINT UNSIGNED NOT NULL,
	user_panel_id       BIGINT UNSIGNED DEFAULT NULL,
	author_name         VARCHAR(200)    NOT NULL,
	body                TEXT            NOT NULL,
	created_at          TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_loan_application_notes_app (loan_application_id, created_at),
	CONSTRAINT fk_loan_application_notes_app FOREIGN KEY (loan_application_id) REFERENCES loan_applications(id) ON DELETE CASCADE,
	CONSTRAINT fk_loan_application_notes_author FOREIGN KEY (user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
