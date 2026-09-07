CREATE TABLE IF NOT EXISTS order_items (
    id bigserial PRIMARY KEY,
    order_id bigint NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id bigint NOT NULL REFERENCES menu_items(id),
    partner_item_id text NOT NULL,
    name_snapshot text NOT NULL,
    quantity integer NOT NULL,
    price_kopecks_snapshot integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT order_items_quantity_positive CHECK (quantity > 0),
    CONSTRAINT order_items_price_positive CHECK (price_kopecks_snapshot > 0)
);
