package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goodfood/promo-service/internal/domain"
)

type PromoRepository struct {
	pool *pgxpool.Pool
}

func NewPromoRepository(pool *pgxpool.Pool) *PromoRepository {
	return &PromoRepository{pool: pool}
}

const promoCols = `id, code, percent_off, min_order_amount_cents, max_redemptions, redemptions_used, starts_at, expires_at, is_active, created_at, updated_at`

func scanPromo(row pgx.Row) (*domain.PromoCode, error) {
	var p domain.PromoCode
	err := row.Scan(&p.ID, &p.Code, &p.PercentOff, &p.MinOrderAmountCents, &p.MaxRedemptions,
		&p.RedemptionsUsed, &p.StartsAt, &p.ExpiresAt, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("promo code not found")
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PromoRepository) Create(ctx context.Context, p *domain.PromoCode) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO promo_codes (`+promoCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.ID, p.Code, p.PercentOff, p.MinOrderAmountCents, p.MaxRedemptions,
		p.RedemptionsUsed, p.StartsAt, p.ExpiresAt, p.IsActive, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PromoRepository) GetByCode(ctx context.Context, code string) (*domain.PromoCode, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+promoCols+` FROM promo_codes WHERE code = $1`, code)
	return scanPromo(row)
}

func (r *PromoRepository) GetByID(ctx context.Context, id string) (*domain.PromoCode, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+promoCols+` FROM promo_codes WHERE id = $1`, id)
	return scanPromo(row)
}

func (r *PromoRepository) List(ctx context.Context) ([]domain.PromoCode, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+promoCols+` FROM promo_codes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PromoCode
	for rows.Next() {
		p, err := scanPromo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *PromoRepository) Update(ctx context.Context, p *domain.PromoCode) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE promo_codes SET percent_off = $1, min_order_amount_cents = $2, max_redemptions = $3,
		 starts_at = $4, expires_at = $5, is_active = $6, updated_at = $7 WHERE id = $8`,
		p.PercentOff, p.MinOrderAmountCents, p.MaxRedemptions, p.StartsAt, p.ExpiresAt, p.IsActive, p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("promo code not found")
	}
	return nil
}

func (r *PromoRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM promo_codes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("promo code not found")
	}
	return nil
}

// Redeem atomically validates and records a redemption for (code, orderID).
// A retry for an orderID that already redeemed this code returns the original
// redemption unchanged instead of consuming a second usage slot.
func (r *PromoRepository) Redeem(ctx context.Context, code, orderID, customerID string, orderAmountCents int64, now time.Time) (*domain.Redemption, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `SELECT `+promoCols+` FROM promo_codes WHERE code = $1 FOR UPDATE`, code)
	promo, err := scanPromo(row)
	if err != nil {
		return nil, err
	}

	// Un retry sur la même commande ne doit jamais consommer une seconde utilisation.
	var existing domain.Redemption
	err = tx.QueryRow(ctx,
		`SELECT id, promo_code_id, order_id, customer_id, discount_cents, redeemed_at
		 FROM promo_redemptions WHERE promo_code_id = $1 AND order_id = $2`,
		promo.ID, orderID).
		Scan(&existing.ID, &existing.PromoCodeID, &existing.OrderID, &existing.CustomerID, &existing.DiscountCents, &existing.RedeemedAt)
	if err == nil {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return &existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if err := promo.CheckEligible(now, orderAmountCents); err != nil {
		return nil, err
	}
	discount := promo.DiscountFor(orderAmountCents)

	redemption := &domain.Redemption{
		ID:            uuid.NewString(),
		PromoCodeID:   promo.ID,
		OrderID:       orderID,
		CustomerID:    customerID,
		DiscountCents: discount,
		RedeemedAt:    now,
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO promo_redemptions (id, promo_code_id, order_id, customer_id, discount_cents, redeemed_at)
		 VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (promo_code_id, order_id) DO NOTHING`,
		redemption.ID, redemption.PromoCodeID, redemption.OrderID, redemption.CustomerID, redemption.DiscountCents, redemption.RedeemedAt)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		// Une requête concurrente a gagné la course : on relit sa redemption.
		if scanErr := tx.QueryRow(ctx,
			`SELECT id, promo_code_id, order_id, customer_id, discount_cents, redeemed_at
			 FROM promo_redemptions WHERE promo_code_id = $1 AND order_id = $2`,
			promo.ID, orderID).
			Scan(&existing.ID, &existing.PromoCodeID, &existing.OrderID, &existing.CustomerID, &existing.DiscountCents, &existing.RedeemedAt); scanErr != nil {
			return nil, scanErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return &existing, nil
	}

	if _, err := tx.Exec(ctx,
		`UPDATE promo_codes SET redemptions_used = redemptions_used + 1, updated_at = $1 WHERE id = $2`,
		now, promo.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return redemption, nil
}
