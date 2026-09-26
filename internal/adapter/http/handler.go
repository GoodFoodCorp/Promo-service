package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"goodfood/promo-service/internal/application"
)

type PromoHandler struct {
	uc *application.UseCases
}

func NewPromoHandler(uc *application.UseCases) *PromoHandler {
	return &PromoHandler{uc: uc}
}

// POST /api/promos
func (h *PromoHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req promoCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	promo, err := h.uc.CreatePromoCode(r.Context(), actorFrom(r), req.toInput())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(promo))
}

// GET /api/promos
func (h *PromoHandler) List(w http.ResponseWriter, r *http.Request) {
	promos, err := h.uc.ListPromoCodes(r.Context(), actorFrom(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponseList(promos))
}

// PUT /api/promos/{id}
func (h *PromoHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req promoCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	promo, err := h.uc.UpdatePromoCode(r.Context(), actorFrom(r), chi.URLParam(r, "id"), req.toInput())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(promo))
}

// DELETE /api/promos/{id}
func (h *PromoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.DeletePromoCode(r.Context(), actorFrom(r), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/promos/preview?code=&amountCents=
func (h *PromoHandler) Preview(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	amountCents, err := strconv.ParseInt(r.URL.Query().Get("amountCents"), 10, 64)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "amountCents must be a valid integer")
		return
	}
	promo, discount, err := h.uc.PreviewPromoCode(r.Context(), actorFrom(r), code, amountCents)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, previewResponse{
		Valid:         true,
		Code:          promo.Code,
		PercentOff:    promo.PercentOff,
		DiscountCents: discount,
	})
}

// POST /api/promos/redeem
func (h *PromoHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	var req redeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	redemption, err := h.uc.RedeemPromoCode(r.Context(), actorFrom(r), req.Code, req.OrderID, req.OrderAmountCents)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, redeemResponse{
		DiscountCents: redemption.DiscountCents,
		Code:          req.Code,
	})
}
