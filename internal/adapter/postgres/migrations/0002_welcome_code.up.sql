-- A code can be limited to one use per customer (a welcome offer).
ALTER TABLE promo_codes ADD COLUMN once_per_customer BOOLEAN NOT NULL DEFAULT FALSE;

-- "Has this customer already used this code?" is asked on every preview.
CREATE INDEX promo_redemptions_code_customer_idx ON promo_redemptions (promo_code_id, customer_id);

-- The welcome offer advertised on the home page: 20 % on a first order.
-- Head office can edit or deactivate it; it is only created if missing.
INSERT INTO promo_codes (id, code, percent_off, min_order_amount_cents, max_redemptions, redemptions_used,
                         starts_at, expires_at, is_active, once_per_customer)
VALUES (gen_random_uuid(), 'BIENVENUE20', 20, 0, NULL, 0, now(), NULL, TRUE, TRUE)
ON CONFLICT (code) DO NOTHING;
