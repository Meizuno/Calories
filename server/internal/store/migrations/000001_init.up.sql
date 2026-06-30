-- A profile is the local account for one external (SSO) user. Everything in the
-- app hangs off a profile (profile_id); the profile links back to the external
-- user via user_id and carries that user's daily macro goal.
CREATE TABLE profiles (
    id          bigserial PRIMARY KEY,
    user_id     text NOT NULL UNIQUE,
    -- public_id is the opaque sharing handle used by /profile/{uuid}. gen_random_uuid
    -- is core in Postgres 13+ (no extension needed); stored as text for ergonomic codegen.
    public_id   text NOT NULL UNIQUE DEFAULT gen_random_uuid()::text,
    name        text NOT NULL DEFAULT '',
    kcal        double precision NOT NULL DEFAULT 2300,
    carb        double precision NOT NULL DEFAULT 253,
    protein     double precision NOT NULL DEFAULT 169,
    fat         double precision NOT NULL DEFAULT 68,
    shared      boolean NOT NULL DEFAULT false,
    onboarded   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE foods (
    id            bigserial PRIMARY KEY,
    profile_id    bigint NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    name          text NOT NULL,
    note          text,
    basis_unit    text NOT NULL DEFAULT 'g',
    basis_amount  double precision NOT NULL DEFAULT 100,
    kcal          double precision NOT NULL DEFAULT 0,
    carb          double precision NOT NULL DEFAULT 0,
    protein       double precision NOT NULL DEFAULT 0,
    fat           double precision NOT NULL DEFAULT 0,
    archived      boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX foods_profile_idx ON foods (profile_id);

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

-- A meal belongs to a (profile, date). There is no `days` table — a "day" is just
-- the meals sharing a date; the daily target comes from the profile's goal.
CREATE TABLE meals (
    id          bigserial PRIMARY KEY,
    profile_id  bigint NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    date        date NOT NULL,
    name        text NOT NULL,
    position    integer NOT NULL DEFAULT 0,
    note        text
);
CREATE INDEX meals_profile_date_idx ON meals (profile_id, date);

CREATE TABLE entries (
    id        bigserial PRIMARY KEY,
    meal_id   bigint NOT NULL REFERENCES meals (id) ON DELETE CASCADE,
    food_id   bigint REFERENCES foods (id) ON DELETE SET NULL,
    name      text NOT NULL,
    quantity  double precision NOT NULL,
    unit      text NOT NULL DEFAULT 'g',
    position  integer NOT NULL DEFAULT 0,
    kcal      double precision NOT NULL DEFAULT 0,
    carb      double precision NOT NULL DEFAULT 0,
    protein   double precision NOT NULL DEFAULT 0,
    fat       double precision NOT NULL DEFAULT 0
);
CREATE INDEX entries_meal_idx ON entries (meal_id);
