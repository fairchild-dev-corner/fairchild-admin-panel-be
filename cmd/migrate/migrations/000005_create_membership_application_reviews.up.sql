-- Review metadata for fairchild_be's membership_applications table. That
-- table is owned by fairchild_be's migrations, so instead of altering it from
-- here we only flip its existing `status` column and keep who/when/why in
-- this side table. No FK to membership_applications on purpose: a
-- cross-service FK would block fairchild_be's own down migrations.
CREATE TABLE membership_application_reviews (
	id                        BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	membership_application_id BIGINT UNSIGNED NOT NULL,
	decision                  ENUM('approved','rejected') NOT NULL,
	note                      VARCHAR(500)    DEFAULT NULL,
	reviewed_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	reviewed_at               TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE KEY uq_membership_application_reviews_app (membership_application_id),
	CONSTRAINT fk_membership_application_reviews_reviewer FOREIGN KEY (reviewed_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
