DELETE FROM promo_codes WHERE code = 'BIENVENUE20' AND redemptions_used = 0;
DROP INDEX IF EXISTS promo_redemptions_code_customer_idx;
ALTER TABLE promo_codes DROP COLUMN IF EXISTS once_per_customer;
