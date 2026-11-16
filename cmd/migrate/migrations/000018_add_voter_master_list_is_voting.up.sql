-- The master list's Category column says whether a listed member is voting
-- or non-voting. Rows already on the list were all voting.
ALTER TABLE voter_master_list
	ADD COLUMN is_voting BOOLEAN NOT NULL DEFAULT TRUE AFTER listed_name;
