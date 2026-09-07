CREATE TABLE IF NOT EXISTS carts (
    id bigserial PRIMARY KEY,
    user_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
