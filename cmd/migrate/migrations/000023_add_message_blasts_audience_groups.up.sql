-- Blast audiences can also be narrowed to master list groups (voting,
-- non_voting, ineligible, delinquent; comma-separated, empty = anyone), or
-- be members the admin picked by hand (audience_picked > 0, and then the
-- status filter is empty).
ALTER TABLE message_blasts
	ADD COLUMN audience_groups VARCHAR(100) NOT NULL DEFAULT '' AFTER audience_branches,
	ADD COLUMN audience_picked INT UNSIGNED NOT NULL DEFAULT 0 AFTER audience_groups;
