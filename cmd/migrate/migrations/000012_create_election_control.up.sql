-- Single-row switch for the yearly election period, set from Election ->
-- Controller (admin password required). While it's off the voters' master
-- list can't be uploaded and the Generate Master List page is hidden.
CREATE TABLE election_control (
	id                       TINYINT UNSIGNED NOT NULL DEFAULT 1,
	enabled                  BOOLEAN          NOT NULL DEFAULT FALSE,
	updated_by_user_panel_id BIGINT UNSIGNED  DEFAULT NULL,
	updated_by_name          VARCHAR(200)     DEFAULT NULL,
	updated_at               TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	CONSTRAINT chk_election_control_singleton CHECK (id = 1),
	CONSTRAINT fk_election_control_user FOREIGN KEY (updated_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO election_control (id, enabled) VALUES (1, FALSE);
