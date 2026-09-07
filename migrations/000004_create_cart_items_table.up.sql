CREATE TABLE IF NOT EXISTS cart_items (
    id bigserial PRIMARY KEY,
    cart_id bigint NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    menu_item_id bigint NOT NULL REFERENCES menu_items(id),
    quantity integer NOT NULL,
    price_kopecks_snapshot integer NOT NULL,
    name_snapshot text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT cart_items_quantity_positive CHECK (quantity > 0),
    CONSTRAINT cart_items_price_positive CHECK (price_kopecks_snapshot > 0),
    CONSTRAINT cart_items_unique_menu_item UNIQUE (cart_id, menu_item_id)
);
