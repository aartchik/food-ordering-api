CREATE TABLE IF NOT EXISTS restaurants (
    id bigserial PRIMARY KEY,
    partner_id text NOT NULL UNIQUE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    address text NOT NULL DEFAULT '',
    is_open boolean NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
