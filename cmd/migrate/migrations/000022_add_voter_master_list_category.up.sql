-- The master list's Category column (voting, non-voting or ineligible) and
-- its Delinquent column, so blasts can reach each group. is_voting stays the
-- vote check: category = 'voting'. Rows already on the list keep their
-- voting / non-voting split.
ALTER TABLE voter_master_list
	ADD COLUMN category      VARCHAR(20) NOT NULL DEFAULT 'voting' AFTER is_voting,
	ADD COLUMN is_delinquent BOOLEAN     NOT NULL DEFAULT FALSE AFTER category;

UPDATE voter_master_list SET category = IF(is_voting, 'voting', 'non_voting');
