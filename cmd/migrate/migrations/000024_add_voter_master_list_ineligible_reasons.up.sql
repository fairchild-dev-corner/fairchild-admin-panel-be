-- Why a member on the voters' master list is ineligible: the requirements
-- they didn't meet (the upload's Remarks, or its Delinquent / Less than 4k /
-- Unpaid Membership Fee / NO TIN columns), one per line. Shown to the member
-- on the portal (fairchild_be reads it). NULL for everyone else.
ALTER TABLE voter_master_list
	ADD COLUMN ineligible_reasons TEXT NULL AFTER is_delinquent;
