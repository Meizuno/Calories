-- Recipes are deferred and the foods.note column is unused; drop them until the
-- recipe feature is actually built. IF EXISTS so this is a no-op on fresh DBs
-- (where 000001 is the only schema) and a real drop on already-migrated ones.
DROP TABLE IF EXISTS recipe_items;
DROP TABLE IF EXISTS recipes;
ALTER TABLE foods DROP COLUMN IF EXISTS note;
