-- Foods become a per-user memory of what you actually eat: logging an item
-- remembers it, so the next time you only pick it and give a quantity. That
-- upsert needs one row per (profile, name, unit) to land on.
--
-- Case-insensitive on the name, because "Banán" and "banán" are the same food
-- to the person typing them. The unit is part of the key: milk logged in ml and
-- milk logged in g are different measurements and must not overwrite each other.

-- Collapse any pre-existing duplicates onto the most recently updated row.
-- Nothing wrote foods before now except the seed, so this is normally a no-op.
DELETE FROM foods a
USING foods b
WHERE a.profile_id = b.profile_id
  AND lower(a.name) = lower(b.name)
  AND a.basis_unit = b.basis_unit
  AND (a.updated_at, a.id) < (b.updated_at, b.id);

CREATE UNIQUE INDEX foods_profile_name_unit_idx
  ON foods (profile_id, lower(name), basis_unit);
