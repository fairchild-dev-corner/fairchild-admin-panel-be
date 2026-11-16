-- When a note's author last edited it; NULL for a note never edited.
ALTER TABLE loan_application_notes
	ADD COLUMN edited_at TIMESTAMP NULL DEFAULT NULL AFTER created_at;
