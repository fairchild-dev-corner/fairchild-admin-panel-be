ALTER TABLE elections
	DROP COLUMN master_list_updated_by,
	DROP COLUMN master_list_updated_at,
	DROP COLUMN master_list_file;

DROP TABLE IF EXISTS election_voters;
