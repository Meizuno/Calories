-- Restores the external-SSO shape: profiles point back at their original auth
-- service ids. Profiles created after the cutover have no legacy id, so they
-- cannot be represented here and are dropped.
DROP INDEX IF EXISTS profiles_legacy_user_idx;
ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_user_fk;
DELETE FROM profiles WHERE legacy_user_id IS NULL;
UPDATE profiles SET user_id = legacy_user_id;
ALTER TABLE profiles DROP COLUMN legacy_user_id;
ALTER TABLE profiles ALTER COLUMN user_id SET NOT NULL;

DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS users;
