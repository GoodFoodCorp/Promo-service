package application

import (
	"context"
	"time"

	"goodfood/promo-service/internal/domain"
)

// Actor is the authenticated caller (from the auth-service JWT).
type Actor struct {
	UserID    string
	RoleSlugs []string
}

func (a Actor) HasRole(slug string) bool {
	for _, r := range a.RoleSlugs {
		if r == slug {
			return true
		}
	}
	return false
}

const RoleAdmin = "admin"

type UseCases struct {
	promos domain.PromoRepository
}

func NewUseCases(promos domain.PromoRepository) *UseCases {
	return &UseCases{promos: promos}
}

func requireAdmin(actor Actor) error {
	if !actor.HasRole(RoleAdmin) {
		return domain.NewForbiddenError("only head office can manage promo codes")
	}
	return nil
}

// CreatePromoCode — head office only.
func (uc *UseCases) CreatePromoCode(ctx context.Context, actor Actor, in domain.PromoCodeInput) (*domain.PromoCode, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	promo, err := domain.NewPromoCode(in)
	if err != nil {
		return nil, err
	}
	if existing, err := uc.promos.GetByCode(ctx, promo.Code); err == nil && existing != nil {
		return nil, domain.NewConflictError("a promo code with this name already exists")
	}
	if err := uc.promos.Create(ctx, promo); err != nil {
		return nil, err
	}
	return promo, nil
}

// ListPromoCodes — head office only (management view, all codes incl. inactive).
func (uc *UseCases) ListPromoCodes(ctx context.Context, actor Actor) ([]domain.PromoCode, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	return uc.promos.List(ctx)
}

// UpdatePromoCode — head office only.
func (uc *UseCases) UpdatePromoCode(ctx context.Context, actor Actor, id string, in domain.PromoCodeInput) (*domain.PromoCode, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	promo, err := uc.promos.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := promo.ApplyUpdate(in); err != nil {
		return nil, err
	}
	if err := uc.promos.Update(ctx, promo); err != nil {
		return nil, err
	}
	return promo, nil
}

// DeletePromoCode — head office only, and only if never redeemed.
func (uc *UseCases) DeletePromoCode(ctx context.Context, actor Actor, id string) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	promo, err := uc.promos.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !promo.CanBeDeleted() {
		return domain.NewConflictError("this code has already been used — deactivate it instead of deleting it")
	}
	return uc.promos.Delete(ctx, id)
}

// PreviewPromoCode — any authenticated customer. Read-only: safe to call on
// every keystroke of the cart's promo field, never consumes a redemption.
func (uc *UseCases) PreviewPromoCode(ctx context.Context, actor Actor, code string, orderAmountCents int64) (*domain.PromoCode, int64, error) {
	if actor.UserID == "" {
		return nil, 0, domain.NewForbiddenError("authentication required")
	}
	promo, err := uc.promos.GetByCode(ctx, domain.NormalizeCode(code))
	if err != nil {
		return nil, 0, err
	}
	if err := promo.CheckEligible(time.Now().UTC(), orderAmountCents); err != nil {
		return nil, 0, err
	}
	return promo, promo.DiscountFor(orderAmountCents), nil
}

// RedeemPromoCode — called by order-service, forwarding the customer's own
// JWT, at the moment an order is actually placed. Atomic and idempotent: a
// retry for the same orderID never double-consumes the code.
func (uc *UseCases) RedeemPromoCode(ctx context.Context, actor Actor, code, orderID string, orderAmountCents int64) (*domain.Redemption, error) {
	if actor.UserID == "" {
		return nil, domain.NewForbiddenError("authentication required")
	}
	if orderID == "" {
		return nil, domain.NewValidationError("orderId is required")
	}
	return uc.promos.Redeem(ctx, domain.NormalizeCode(code), orderID, actor.UserID, orderAmountCents, time.Now().UTC())
}
