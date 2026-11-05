-- SMS/email blasts to portal members (announcements, notices), sent from the
-- panel. A blast's recipients are snapshotted when it's created; a
-- background worker sends them in batches and records each outcome, so
-- progress survives a restart. Recipient rows hold no FK to fairchild_be's
-- `users` on purpose (same reason as contact_message_replies).
CREATE TABLE message_blasts (
	id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	channel                  ENUM('email','sms','both') NOT NULL,
	subject                  VARCHAR(200)    DEFAULT NULL,
	email_body               TEXT            DEFAULT NULL,
	sms_body                 VARCHAR(480)    DEFAULT NULL,
	-- Audience filters, comma-separated (empty branches = every branch).
	audience_statuses        VARCHAR(100)    NOT NULL,
	audience_branches        VARCHAR(100)    NOT NULL DEFAULT '',
	status                   ENUM('sending','completed','cancelled') NOT NULL DEFAULT 'sending',
	-- Outside prod only allow-listed test recipients are messaged.
	test_mode                BOOLEAN         NOT NULL,
	created_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	created_by_name          VARCHAR(200)    NOT NULL,
	created_at               TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	finished_at              TIMESTAMP       NULL DEFAULT NULL,
	PRIMARY KEY (id),
	KEY idx_message_blasts_status (status, id),
	CONSTRAINT fk_message_blasts_sender FOREIGN KEY (created_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- One row per message: a member gets one row per channel. 'sending' rows are
-- claimed by a worker; claimed_at lets a stuck claim be retried.
CREATE TABLE message_blast_recipients (
	id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	blast_id   BIGINT UNSIGNED NOT NULL,
	user_id    BIGINT UNSIGNED NOT NULL,
	member_id  VARCHAR(50)     NOT NULL DEFAULT '',
	name       VARCHAR(255)    NOT NULL DEFAULT '',
	channel    ENUM('email','sms') NOT NULL,
	recipient  VARCHAR(255)    NOT NULL,
	status     ENUM('pending','sending','sent','failed','skipped') NOT NULL DEFAULT 'pending',
	error      VARCHAR(255)    DEFAULT NULL,
	claimed_at TIMESTAMP       NULL DEFAULT NULL,
	sent_at    TIMESTAMP       NULL DEFAULT NULL,
	PRIMARY KEY (id),
	KEY idx_blast_recipients_queue (blast_id, status, id),
	CONSTRAINT fk_blast_recipients_blast FOREIGN KEY (blast_id) REFERENCES message_blasts(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
