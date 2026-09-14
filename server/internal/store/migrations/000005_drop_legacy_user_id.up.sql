-- The cutover is done: every profile now belongs to a local users row, so the
-- parked external-SSO ids have no further use. Dropping legacy_user_id also
-- lets profiles.user_id go back to NOT NULL — it was only ever nullable to
-- represent "unclaimed", which can no longer happen.

-- Refuse rather than destroy. An unclaimed profile here means a diary that
-- would become unreachable the moment its legacy id is gone, so fail loudly and
-- let a human finish the handover first. Migrations run on boot, so this stops
-- the server starting — deliberately: a diary with no owner is worse.
DO $$
DECLARE n int;
BEGIN
    SELECT count(*) INTO n FROM profiles WHERE user_id IS NULL;
    IF n > 0 THEN
        RAISE EXCEPTION
            'Refusing to drop legacy_user_id: % profile(s) still have no owner. '
            'Attach them to an account first (see cmd/claim in git history), then retry.', n;
    END IF;
END $$;

DROP INDEX IF EXISTS profiles_legacy_user_idx;
ALTER TABLE profiles DROP COLUMN legacy_user_id;
ALTER TABLE profiles ALTER COLUMN user_id SET NOT NULL;
