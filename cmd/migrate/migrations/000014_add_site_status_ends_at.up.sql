-- When maintenance lifts on its own. NULL = stays on until an admin turns it
-- off. Once ends_at has passed the site counts as live again, both here and in
-- fairchild_be, which reads this row to show the countdown and to refuse
-- member sign-ins while maintenance is on.
ALTER TABLE site_status
	ADD COLUMN ends_at TIMESTAMP NULL DEFAULT NULL AFTER message;
