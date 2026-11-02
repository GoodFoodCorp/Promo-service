package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goodfood/promo-service/internal/domain"
)

func TestWelcomeOfferOnlyForWhoMayUseIt(t *testing.T) {
	uc, _ := setup()
	ctx := context.Background()
	visitor := Actor{}
	courier := Actor{UserID: "liv-1", RoleSlugs: []string{"livreur"}}

	// No welcome code: nothing to show.
	offer, err := uc.GetWelcomeOffer(ctx, customer)
	require.NoError(t, err)
	assert.False(t, offer.Available)
	assert.False(t, offer.Eligible)

	created, err := uc.CreatePromoCode(ctx, admin, domain.PromoCodeInput{
		Code: WelcomeCode, PercentOff: 20, IsActive: true, OncePerCustomer: true,
	})
	require.NoError(t, err)

	offer, err = uc.GetWelcomeOffer(ctx, customer)
	require.NoError(t, err)
	assert.Equal(t, WelcomeOffer{Code: WelcomeCode, PercentOff: 20, Available: true, Eligible: true}, offer)

	offer, _ = uc.GetWelcomeOffer(ctx, visitor)
	assert.True(t, offer.Available)
	assert.False(t, offer.Eligible, "a visitor is eligible once signed up, not before")
	offer, _ = uc.GetWelcomeOffer(ctx, courier)
	assert.False(t, offer.Eligible, "staff never order")
	offer, _ = uc.GetWelcomeOffer(ctx, admin)
	assert.False(t, offer.Eligible)

	// Used once: no longer offered to that customer, still to the others.
	_, err = uc.RedeemPromoCode(ctx, WelcomeCode, "order-1", customer.UserID, 2000)
	require.NoError(t, err)
	offer, _ = uc.GetWelcomeOffer(ctx, customer)
	assert.False(t, offer.Eligible)
	offer, _ = uc.GetWelcomeOffer(ctx, Actor{UserID: "cust-2", RoleSlugs: []string{RoleCustomer}})
	assert.True(t, offer.Eligible)

	// Switched off by head office: shown to nobody.
	_, err = uc.UpdatePromoCode(ctx, admin, created.ID, domain.PromoCodeInput{
		Code: WelcomeCode, PercentOff: 20, IsActive: false, OncePerCustomer: true,
	})
	require.NoError(t, err)
	offer, _ = uc.GetWelcomeOffer(ctx, Actor{UserID: "cust-2", RoleSlugs: []string{RoleCustomer}})
	assert.False(t, offer.Available)
	assert.False(t, offer.Eligible)
}
