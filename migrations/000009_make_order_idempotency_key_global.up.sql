ALTER TABLE orders
DROP CONSTRAINT orders_cart_idempotency_unique;

ALTER TABLE orders
ADD CONSTRAINT orders_idempotency_key_unique UNIQUE (idempotency_key);
