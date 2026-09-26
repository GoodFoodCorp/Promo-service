package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goodfood/promo-service/internal/domain"
)

type fakeRepo struct {
	byCode      map[string]*domain.PromoCode
	byID        map[string]*domain.PromoCode
	redemptions map[string]*domain.Redemption // key: promoID+"|"+orderID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byCode:      map[string]*domain.PromoCode{},
		byID:        map[string]*domain.PromoCode{},
		redemptions: map[string]*domain.Redemption{},
	}
}

func (f *fakeRepo) Create(_ context.Context, p *domain.PromoCode) error {
	cp := *p
	f.byCode[p.Code] = &cp
	f.byID[p.ID] = &cp
	return nil
}

func (f *fakeRepo) GetByCode(_ context.Context, code string) (*domain.PromoCode, error) {
	p, ok := f.byCode[code]
	if !ok {
		return nil, domain.NewNotFoundError("promo code not found")
	}
	cp := *p
	return &cp, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (*domain.PromoCode, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, domain.NewNotFoundError("promo code not found")
	}
	cp := *p
	return &cp, nil
}

func (f *fakeRepo) List(_ context.Context) ([]domain.PromoCode, error) {
	out := make([]domain.PromoCode, 0, len(f.byID))
	for _, p := range f.byID {
		out = append(out, *p)
	}
	return out, nil
}

func (f *fakeRepo) Update(_ context.Context, p *domain.PromoCode) error {
	cp := *p
	f.byID[p.ID] = &cp
	f.byCode[p.Code] = &cp
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	p, ok := f.byID[id]
	if !ok {
		return domain.NewNotFoundError("promo code not found")
	}
	delete(f.byID, id)
	delete(f.byCode, p.Code)
	return nil
}

func (f *fakeRepo) Redeem(_ context.Context, code, orderID, customerID string, orderAmountCents int64, now time.Time) (*domain.Redemption, error) {
	promo, ok := f.byCode[code]
	if !ok {
		return nil, domain.NewNotFoundError("promo code not found")
	}
	key := promo.ID + "|" + orderID
	if existing, ok := f.redemptions[key]; ok {
		return existing, nil
	}
	if err := promo.CheckEligible(now, orderAmountCents); err != nil {
		return nil, err
	}
	redemption := &domain.Redemption{
		ID:            uuid.NewString(),
		PromoCodeID:   promo.ID,
		OrderID:       orderID,
		CustomerID:    customerID,
		DiscountCents: promo.DiscountFor(orderAmountCents),
		RedeemedAt:    now,
	}
	f.redemptions[key] = redemption
	promo.RedemptionsUsed++
	f.byID[promo.ID] = promo
	f.byCode[promo.Code] = promo
	return redemption, nil
}

var (
	admin    = Actor{UserID: "adm-1", RoleSlugs: []string{RoleAdmin}}
	customer = Actor{UserID: "cust-1", RoleSlugs: []string{"user"}}
)

func setup() (*UseCases, *fakeRepo) {
	repo := newFakeRepo()
	return NewUseCases(repo), repo
}

func TestCreatePromoCodeRequiresAdmin(t *testing.T) {
	uc, _ := setup()
	_, err := uc.CreatePromoCode(context.Background(), customer, domain.PromoCodeInput{Code: "WELCOME10", PercentOff: 10})
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeForbidden, derr.Code)
}

func TestCreatePromoCodeRejectsDuplicate(t *testing.T) {
	uc, _ := setup()
	in := domain.PromoCodeInput{Code: "WELCOME10", PercentOff: 10, IsActive: true}
	_, err := uc.CreatePromoCode(context.Background(), admin, in)
	require.NoError(t, err)

	_, err = uc.CreatePromoCode(context.Background(), admin, in)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeConflict, derr.Code)
}

func TestPreviewPromoCode(t *testing.T) {
	uc, _ := setup()
	_, err := uc.CreatePromoCode(context.Background(), admin, domain.PromoCodeInput{
		Code: "WELCOME10", PercentOff: 10, MinOrderAmountCents: 1000, IsActive: true,
	})
	require.NoError(t, err)

	promo, discount, err := uc.PreviewPromoCode(context.Background(), customer, "welcome10", 5000)
	require.NoError(t, err)
	assert.Equal(t, "WELCOME10", promo.Code)
	assert.Equal(t, int64(500), discount)
}

func TestPreviewPromoCodeBelowMinimum(t *testing.T) {
	uc, _ := setup()
	_, err := uc.CreatePromoCode(context.Background(), admin, domain.PromoCodeInput{
		Code: "WELCOME10", PercentOff: 10, MinOrderAmountCents: 1000, IsActive: true,
	})
	require.NoError(t, err)

	_, _, err = uc.PreviewPromoCode(context.Background(), customer, "WELCOME10", 500)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeConflict, derr.Code)
}

func TestRedeemPromoCodeIsIdempotent(t *testing.T) {
	uc, repo := setup()
	_, err := uc.CreatePromoCode(context.Background(), admin, domain.PromoCodeInput{
		Code: "WELCOME10", PercentOff: 10, IsActive: true,
	})
	require.NoError(t, err)

	first, err := uc.RedeemPromoCode(context.Background(), customer, "WELCOME10", "order-1", 2000)
	require.NoError(t, err)
	assert.Equal(t, int64(200), first.DiscountCents)

	second, err := uc.RedeemPromoCode(context.Background(), customer, "WELCOME10", "order-1", 2000)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "a retry on the same order must not consume a second redemption")

	promo, err := repo.GetByCode(context.Background(), "WELCOME10")
	require.NoError(t, err)
	assert.Equal(t, 1, promo.RedemptionsUsed)
}

func TestRedeemPromoCodeRespectsMaxRedemptions(t *testing.T) {
	uc, _ := setup()
	max := 1
	_, err := uc.CreatePromoCode(context.Background(), admin, domain.PromoCodeInput{
		Code: "ONESHOT", PercentOff: 20, MaxRedemptions: &max, IsActive: true,
	})
	require.NoError(t, err)

	_, err = uc.RedeemPromoCode(context.Background(), customer, "ONESHOT", "order-1", 1000)
	require.NoError(t, err)

	other := Actor{UserID: "cust-2", RoleSlugs: []string{"user"}}
	_, err = uc.RedeemPromoCode(context.Background(), other, "ONESHOT", "order-2", 1000)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeConflict, derr.Code)
}

func TestDeletePromoCodeRefusedAfterRedemption(t *testing.T) {
	uc, _ := setup()
	promo, err := uc.CreatePromoCode(context.Background(), admin, domain.PromoCodeInput{
		Code: "WELCOME10", PercentOff: 10, IsActive: true,
	})
	require.NoError(t, err)

	_, err = uc.RedeemPromoCode(context.Background(), customer, "WELCOME10", "order-1", 1000)
	require.NoError(t, err)

	err = uc.DeletePromoCode(context.Background(), admin, promo.ID)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.ErrCodeConflict, derr.Code)
}
