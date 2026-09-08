ALTER TABLE orders
DROP CONSTRAINT orders_idempotency_key_unique;

ALTER TABLE orders
ADD CONSTRAINT orders_cart_idempotency_unique UNIQUE (cart_id, idempotency_key);
