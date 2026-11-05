-- Replies sent from the panel to fairchild_be's contact_messages (the public
-- Contact Us form). That table is owned by fairchild_be and is only read
-- here; whether a message has been answered is derived from this table.
-- Only replies the provider accepted are stored. No FK to contact_messages
-- on purpose: a cross-service FK would block fairchild_be's own down
-- migrations.
CREATE TABLE contact_message_replies (
	id                    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	contact_message_id    BIGINT UNSIGNED NOT NULL,
	channel               ENUM('email','sms') NOT NULL,
	recipient             VARCHAR(255)    NOT NULL,
	body                  VARCHAR(2000)   NOT NULL,
	sent_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	sent_by_name          VARCHAR(200)    NOT NULL,
	created_at            TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_contact_message_replies_message (contact_message_id, created_at),
	CONSTRAINT fk_contact_message_replies_sender FOREIGN KEY (sent_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
