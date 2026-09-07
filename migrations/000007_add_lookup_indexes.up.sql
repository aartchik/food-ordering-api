CREATE INDEX IF NOT EXISTS restaurants_open_name_idx
    ON restaurants(is_open, name);

CREATE INDEX IF NOT EXISTS menu_items_restaurant_available_idx
    ON menu_items(restaurant_id, is_available);

CREATE INDEX IF NOT EXISTS cart_items_cart_id_idx
    ON cart_items(cart_id);

CREATE INDEX IF NOT EXISTS orders_customer_phone_created_at_idx
    ON orders(customer_phone, created_at DESC);

CREATE INDEX IF NOT EXISTS orders_partner_order_id_idx
    ON orders(partner_order_id);

CREATE INDEX IF NOT EXISTS orders_status_updated_at_idx
    ON orders(status, updated_at DESC);

CREATE INDEX IF NOT EXISTS order_items_order_id_idx
    ON order_items(order_id);
