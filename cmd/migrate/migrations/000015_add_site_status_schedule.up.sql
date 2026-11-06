-- Maintenance can be scheduled ahead: maintenance_mode is saved on and
-- starts_at says when it begins (NULL = when it was saved). Until then members
-- are warned and can still sign in; from then on they're kept out.
-- signed_out_at records that every member session was revoked for the current
-- window, so that happens exactly once: on the save that starts it, or from
-- the admin panel's scheduler when a scheduled start passes.
ALTER TABLE site_status
	ADD COLUMN starts_at TIMESTAMP NULL DEFAULT NULL AFTER message,
	ADD COLUMN signed_out_at TIMESTAMP NULL DEFAULT NULL AFTER ends_at;

-- A window that's already on signed members out when it was turned on.
UPDATE site_status SET signed_out_at = updated_at WHERE maintenance_mode;
