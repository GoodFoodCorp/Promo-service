//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"goodfood/promo-service/internal/domain"
)

// Run with: TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/adapter/postgres/
func TestRedeemThenRelease(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	if err := Migrate(url); err != nil {
		t.Fatal(err)
	}
	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := NewPromoRepository(pool)

	max := 1
	now := time.Now().UTC().Add(time.Minute) // after StartsAt, set at creation
	code := "IT" + uuid.NewString()[:8]
	promo, err := domain.NewPromoCode(domain.PromoCodeInput{Code: code, PercentOff: 10, MaxRedemptions: &max, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, promo); err != nil {
		t.Fatal(err)
	}
	order := uuid.NewString()
	if _, err := repo.Redeem(ctx, promo.Code, order, "cust-1", 2000, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Redeem(ctx, promo.Code, uuid.NewString(), "cust-2", 2000, now); err == nil {
		t.Fatal("the single usage is consumed")
	}

	if err := repo.Release(ctx, order, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Release(ctx, order, now); err != nil {
		t.Fatal("release must be idempotent:", err)
	}
	stored, _ := repo.GetByCode(ctx, promo.Code)
	if stored.RedemptionsUsed != 0 {
		t.Fatalf("redemptions_used = %d", stored.RedemptionsUsed)
	}
	if _, err := repo.Redeem(ctx, promo.Code, uuid.NewString(), "cust-2", 2000, now); err != nil {
		t.Fatal("the freed usage can be redeemed again:", err)
	}
}
