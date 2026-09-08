CREATE INDEX IF NOT EXISTS orders_partner_status_created_at_idx
    ON orders(partner_id, status, created_at DESC);
