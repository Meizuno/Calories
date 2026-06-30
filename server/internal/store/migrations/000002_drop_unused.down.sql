ALTER TABLE foods ADD COLUMN note text;

CREATE TABLE recipes (
    id           bigserial PRIMARY KEY,
    profile_id   bigint NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    name         text NOT NULL,
    servings     double precision NOT NULL DEFAULT 1,
    composition  text,
    archived     boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX recipes_profile_idx ON recipes (profile_id);

CREATE TABLE recipe_items (
    id         bigserial PRIMARY KEY,
    recipe_id  bigint NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    food_id    bigint NOT NULL REFERENCES foods (id) ON DELETE RESTRICT,
    quantity   double precision NOT NULL,
    unit       text NOT NULL DEFAULT 'g'
);
CREATE INDEX recipe_items_recipe_idx ON recipe_items (recipe_id);
