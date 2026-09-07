CREATE TABLE IF NOT EXISTS orders (
    id bigserial PRIMARY KEY,
    cart_id bigint NOT NULL REFERENCES carts(id),
    restaurant_id bigint NOT NULL REFERENCES restaurants(id),
    partner_id text NOT NULL,
    partner_order_id text,
    status text NOT NULL,
    customer_name text NOT NULL,
    customer_phone text NOT NULL,
    delivery_address text NOT NULL,
    customer_comment text,
    total_kopecks integer NOT NULL,
    idempotency_key text NOT NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT orders_status_valid CHECK (status IN (
        'pending_partner',
        'accepted',
        'cooking',
        'ready',
        'delivering',
        'delivered',
        'cancelled'
    )),
    CONSTRAINT orders_total_positive CHECK (total_kopecks > 0),
    CONSTRAINT orders_idempotency_key_not_blank CHECK (length(trim(idempotency_key)) > 0),
    CONSTRAINT orders_cart_idempotency_unique UNIQUE (cart_id, idempotency_key)
);
