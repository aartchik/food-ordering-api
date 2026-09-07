CREATE TABLE IF NOT EXISTS menu_items (
    id bigserial PRIMARY KEY,
    restaurant_id bigint NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    partner_item_id text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    price_kopecks integer NOT NULL,
    is_available boolean NOT NULL DEFAULT true,
    preparation_min integer NOT NULL DEFAULT 0,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT menu_items_price_positive CHECK (price_kopecks > 0),
    CONSTRAINT menu_items_preparation_min_range CHECK (preparation_min >= 0 AND preparation_min <= 240),
    CONSTRAINT menu_items_partner_item_unique UNIQUE (restaurant_id, partner_item_id)
);
