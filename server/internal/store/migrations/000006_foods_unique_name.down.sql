-- The duplicates collapsed on the way up are not recoverable; this only drops
-- the constraint that keeps new ones from appearing.
DROP INDEX IF EXISTS foods_profile_name_unit_idx;
