-- Recreates the (empty) election tables exactly as 000009_create_elections did.

-- Cooperative elections set up from the panel: positions to fill and the
-- members running for them. Every active portal member may vote while an
-- election is open. A draft (published_at NULL) is only visible here.
CREATE TABLE elections (
	id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	title                    VARCHAR(200)    NOT NULL,
	description              TEXT            DEFAULT NULL,
	starts_at                TIMESTAMP       NOT NULL,
	ends_at                  TIMESTAMP       NOT NULL,
	published_at             TIMESTAMP       NULL DEFAULT NULL,
	created_by_user_panel_id BIGINT UNSIGNED DEFAULT NULL,
	created_by_name          VARCHAR(200)    NOT NULL,
	created_at               TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at               TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_elections_starts (starts_at),
	CONSTRAINT fk_elections_creator FOREIGN KEY (created_by_user_panel_id) REFERENCES users_panel(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE election_positions (
	id          BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
	election_id BIGINT UNSIGNED  NOT NULL,
	title       VARCHAR(150)     NOT NULL,
	-- How many candidates win, and how many a voter may pick.
	seats       TINYINT UNSIGNED NOT NULL DEFAULT 1,
	sort_order  INT              NOT NULL DEFAULT 0,
	PRIMARY KEY (id),
	KEY idx_election_positions_election (election_id, sort_order),
	CONSTRAINT fk_election_positions_election FOREIGN KEY (election_id) REFERENCES elections(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- A candidate is a portal member (user_id/member_no as of when added). No FK
-- to fairchild_be's `users` on purpose (same reason as contact_message_replies).
CREATE TABLE election_candidates (
	id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	position_id BIGINT UNSIGNED NOT NULL,
	user_id     BIGINT UNSIGNED NOT NULL,
	member_no   VARCHAR(50)     NOT NULL,
	name        VARCHAR(255)    NOT NULL,
	bio         VARCHAR(1000)   DEFAULT NULL,
	sort_order  INT             NOT NULL DEFAULT 0,
	PRIMARY KEY (id),
	KEY idx_election_candidates_position (position_id, sort_order),
	CONSTRAINT fk_election_candidates_position FOREIGN KEY (position_id) REFERENCES election_positions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- One row per candidate a member voted for. Written by the member portal
-- (not built yet); the panel only reads it for results and turnout.
CREATE TABLE election_votes (
	id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	election_id  BIGINT UNSIGNED NOT NULL,
	position_id  BIGINT UNSIGNED NOT NULL,
	candidate_id BIGINT UNSIGNED NOT NULL,
	user_id      BIGINT UNSIGNED NOT NULL,
	created_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE KEY uq_election_votes_ballot (position_id, candidate_id, user_id),
	KEY idx_election_votes_voter (election_id, user_id),
	KEY idx_election_votes_candidate (candidate_id),
	CONSTRAINT fk_election_votes_election FOREIGN KEY (election_id) REFERENCES elections(id) ON DELETE CASCADE,
	CONSTRAINT fk_election_votes_candidate FOREIGN KEY (candidate_id) REFERENCES election_candidates(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
