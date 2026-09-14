-- Restores the shape, but NOT the data: the external-SSO ids this column held
-- are gone for good once the up migration ran. Rolling back gives you an empty
-- legacy_user_id and a nullable user_id again, which is only useful if you are
-- re-running 000004's rollback on a database that never had real legacy ids.
ALTER TABLE profiles ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE profiles ADD COLUMN legacy_user_id text;
CREATE UNIQUE INDEX profiles_legacy_user_idx
    ON profiles (legacy_user_id) WHERE legacy_user_id IS NOT NULL;
