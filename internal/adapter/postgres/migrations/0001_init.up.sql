CREATE TABLE promo_codes (
    id                      UUID PRIMARY KEY,
    code                    TEXT NOT NULL UNIQUE,
    percent_off             INT NOT NULL,
    min_order_amount_cents  BIGINT NOT NULL DEFAULT 0,
    max_redemptions         INT,
    redemptions_used        INT NOT NULL DEFAULT 0,
    starts_at               TIMESTAMPTZ NOT NULL,
    expires_at              TIMESTAMPTZ,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE promo_redemptions (
    id                UUID PRIMARY KEY,
    promo_code_id     UUID NOT NULL REFERENCES promo_codes(id),
    order_id          TEXT NOT NULL,
    customer_id       TEXT NOT NULL,
    discount_cents    BIGINT NOT NULL,
    redeemed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (promo_code_id, order_id)
);
