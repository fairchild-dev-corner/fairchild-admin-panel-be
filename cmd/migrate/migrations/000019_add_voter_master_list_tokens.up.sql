-- Election tokens per listed member, from the master list's ATTENDANCE,
-- COMPLETE SURVEY and LUCKY PICK PRIZE columns (pesos; 0 when blank). The
-- member portal shows them and records the claim in fairchild_be's
-- voter_token_claims.
ALTER TABLE voter_master_list
	ADD COLUMN attendance_token DECIMAL(10,2) NOT NULL DEFAULT 0 AFTER is_voting,
	ADD COLUMN survey_token     DECIMAL(10,2) NOT NULL DEFAULT 0 AFTER attendance_token,
	ADD COLUMN lucky_pick_token DECIMAL(10,2) NOT NULL DEFAULT 0 AFTER survey_token;
