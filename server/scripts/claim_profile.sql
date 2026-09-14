-- Hand the pre-cutover diary to a local account.
--
-- Migration 000004 moved profiles.user_id from the external SSO id onto
-- users.id. Those old ids carry no email and the auth service is gone, so every
-- existing profile was parked: its old id kept in legacy_user_id, its user_id
-- set to NULL. This script attaches that profile to a real account.
--
-- cmd/claim does the same thing, but it is not built into the Docker image
-- (the Dockerfile builds only ./cmd/server), so this runs anywhere psql can
-- reach the database.
--
-- BEFORE RUNNING: sign in on the site with the Google account once, so the
-- users row exists. That sign-in also creates an empty profile, which this
-- script removes — a user owns exactly one.
--
--   psql "$DATABASE_URL" -v email=yuramiron16@gmail.com -f claim_profile.sql
--
-- Everything runs in one transaction and every check aborts rather than
-- guessing, so a failed run changes nothing. Re-running after success is a
-- no-op that reports there is nothing left to claim.

\set ON_ERROR_STOP on
\if :{?email}
\else
  \set email 'yuramiron16@gmail.com'
\endif

BEGIN;

-- psql variables cannot be read inside a DO block, so pass the email through a
-- temp table.
CREATE TEMP TABLE _claim_target ON COMMIT DROP AS
SELECT lower(trim(:'email')) AS email;

DO $$
DECLARE
  v_email      text;
  v_user       text;
  v_legacy     bigint;
  v_legacy_n   int;
  v_current    bigint;
  v_current_rows bigint;
  v_foods      bigint;
  v_meals      bigint;
BEGIN
  SELECT email INTO v_email FROM _claim_target;

  SELECT id INTO v_user FROM users WHERE email_norm = v_email;
  IF v_user IS NULL THEN
    RAISE EXCEPTION
      'No account for %. Sign in with Google on the site first, then re-run.', v_email;
  END IF;

  -- The pre-cutover profiles: unowned, but still carrying their old SSO id.
  SELECT count(*) INTO v_legacy_n
  FROM profiles WHERE user_id IS NULL AND legacy_user_id IS NOT NULL;

  IF v_legacy_n = 0 THEN
    RAISE NOTICE 'Nothing to claim — no unclaimed profile remains. Already done?';
    RETURN;
  END IF;
  IF v_legacy_n > 1 THEN
    RAISE EXCEPTION
      'Found % unclaimed profiles; this script only handles one. Use cmd/claim to pick explicitly.', v_legacy_n;
  END IF;

  SELECT id INTO v_legacy
  FROM profiles WHERE user_id IS NULL AND legacy_user_id IS NOT NULL;

  -- The empty profile created when the account first signed in. Only remove it
  -- if it really is empty: if anything was logged against it, that is real data
  -- and merging is a decision for a human, not this script.
  SELECT id INTO v_current FROM profiles WHERE user_id = v_user;
  IF v_current IS NOT NULL THEN
    SELECT count(*) INTO v_current_rows FROM foods WHERE profile_id = v_current;
    SELECT v_current_rows + count(*) INTO v_current_rows FROM meals WHERE profile_id = v_current;
    IF v_current_rows > 0 THEN
      RAISE EXCEPTION
        'Account % already owns profile % with % rows of its own data — refusing to discard it.',
        v_email, v_current, v_current_rows;
    END IF;
    DELETE FROM profiles WHERE id = v_current;
    RAISE NOTICE 'Removed the empty profile % created at sign-in.', v_current;
  END IF;

  UPDATE profiles SET user_id = v_user, updated_at = now() WHERE id = v_legacy;

  SELECT count(*) INTO v_foods FROM foods WHERE profile_id = v_legacy;
  SELECT count(*) INTO v_meals FROM meals WHERE profile_id = v_legacy;
  RAISE NOTICE 'Profile % now belongs to % — % foods, % meals.', v_legacy, v_email, v_foods, v_meals;
END $$;

COMMIT;

-- Final state, for eyeballing before you walk away.
SELECT p.id AS profile,
       coalesce(u.email, '(unclaimed)') AS owner,
       p.name,
       p.shared,
       (SELECT count(*) FROM foods f WHERE f.profile_id = p.id) AS foods,
       (SELECT count(*) FROM meals m WHERE m.profile_id = p.id) AS meals
FROM profiles p
LEFT JOIN users u ON u.id = p.user_id
ORDER BY p.id;
