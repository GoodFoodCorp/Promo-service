package domain

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PromoCode is a network-wide discount code, created by head office. Every
// business rule (active window, usage cap, minimum order) lives here so the
// SQL layer only has to enforce them atomically, never decide them.
type PromoCode struct {
	ID                  string
	Code                string
	PercentOff          int
	MinOrderAmountCents int64
	MaxRedemptions      *int // nil = illimité
	RedemptionsUsed     int
	StartsAt            time.Time
	ExpiresAt           *time.Time // nil = jamais
	IsActive            bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type PromoCodeInput struct {
	Code                string
	PercentOff          int
	MinOrderAmountCents int64
	MaxRedemptions      *int
	StartsAt            *time.Time
	ExpiresAt           *time.Time
	IsActive            bool
}

// NormalizeCode is the single source of truth for how a code is compared:
// case-insensitive, trimmed. Used both when storing and when looking up.
func NormalizeCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func validateShared(in PromoCodeInput, startsAt time.Time, redemptionsUsed int) error {
	if in.PercentOff < 1 || in.PercentOff > 100 {
		return NewValidationError("percentOff must be between 1 and 100")
	}
	if in.MinOrderAmountCents < 0 {
		return NewValidationError("minOrderAmountCents cannot be negative")
	}
	if in.MaxRedemptions != nil {
		if *in.MaxRedemptions < 1 {
			return NewValidationError("maxRedemptions must be at least 1")
		}
		if *in.MaxRedemptions < redemptionsUsed {
			return NewValidationError("maxRedemptions cannot be lower than redemptions already used")
		}
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(startsAt) {
		return NewValidationError("expiresAt must be after startsAt")
	}
	return nil
}

// NewPromoCode creates a code, active from now (or a future StartsAt) unless
// explicitly deactivated.
func NewPromoCode(in PromoCodeInput) (*PromoCode, error) {
	code := NormalizeCode(in.Code)
	if code == "" {
		return nil, NewValidationError("code is required")
	}
	startsAt := time.Now().UTC()
	if in.StartsAt != nil {
		startsAt = *in.StartsAt
	}
	if err := validateShared(in, startsAt, 0); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &PromoCode{
		ID:                  uuid.NewString(),
		Code:                code,
		PercentOff:          in.PercentOff,
		MinOrderAmountCents: in.MinOrderAmountCents,
		MaxRedemptions:      in.MaxRedemptions,
		StartsAt:            startsAt,
		ExpiresAt:           in.ExpiresAt,
		IsActive:            true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// ApplyUpdate edits an existing code. The code string itself never changes —
// deactivate and create a new one instead, so old orders keep a stable label.
func (p *PromoCode) ApplyUpdate(in PromoCodeInput) error {
	startsAt := p.StartsAt
	if in.StartsAt != nil {
		startsAt = *in.StartsAt
	}
	if err := validateShared(in, startsAt, p.RedemptionsUsed); err != nil {
		return err
	}
	p.PercentOff = in.PercentOff
	p.MinOrderAmountCents = in.MinOrderAmountCents
	p.MaxRedemptions = in.MaxRedemptions
	p.StartsAt = startsAt
	p.ExpiresAt = in.ExpiresAt
	p.IsActive = in.IsActive
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// CheckEligible reports whether the code can currently be redeemed for an
// order of the given amount. Pure, no side effect — safe for a live preview.
func (p *PromoCode) CheckEligible(now time.Time, orderAmountCents int64) error {
	if !p.IsActive {
		return NewConflictError("ce code promo n'est plus actif")
	}
	if now.Before(p.StartsAt) {
		return NewConflictError("ce code promo n'est pas encore actif")
	}
	if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
		return NewConflictError("ce code promo a expiré")
	}
	if p.MaxRedemptions != nil && p.RedemptionsUsed >= *p.MaxRedemptions {
		return NewConflictError("ce code promo a atteint sa limite d'utilisation")
	}
	if orderAmountCents < p.MinOrderAmountCents {
		return NewConflictError("le montant de la commande est inférieur au minimum requis pour ce code")
	}
	return nil
}

// DiscountFor computes the discount in cents, rounded down.
func (p *PromoCode) DiscountFor(orderAmountCents int64) int64 {
	return orderAmountCents * int64(p.PercentOff) / 100
}

// CanBeDeleted refuses to erase a code with redemption history — a franchisé
// or le siège doivent pouvoir retracer une remise déjà accordée.
func (p *PromoCode) CanBeDeleted() bool { return p.RedemptionsUsed == 0 }

// Redemption is one successful application of a code to one order. The
// (PromoCodeID, OrderID) pair is unique: retrying the same order never
// double-charges the usage counter.
type Redemption struct {
	ID            string
	PromoCodeID   string
	OrderID       string
	CustomerID    string
	DiscountCents int64
	RedeemedAt    time.Time
}

type PromoRepository interface {
	Create(ctx context.Context, promo *PromoCode) error
	GetByCode(ctx context.Context, code string) (*PromoCode, error)
	GetByID(ctx context.Context, id string) (*PromoCode, error)
	List(ctx context.Context) ([]PromoCode, error)
	Update(ctx context.Context, promo *PromoCode) error
	Delete(ctx context.Context, id string) error

	// Redeem atomically validates and records a redemption for (code, orderID).
	// A retry for an orderID that already redeemed this code returns the
	// original redemption unchanged instead of consuming it twice.
	Redeem(ctx context.Context, code, orderID, customerID string, orderAmountCents int64, now time.Time) (*Redemption, error)
}
