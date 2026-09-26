package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"goodfood/promo-service/internal/domain"
)

type promoCodeRequest struct {
	Code                string     `json:"code"`
	PercentOff          int        `json:"percent_off"`
	MinOrderAmountCents int64      `json:"min_order_amount_cents"`
	MaxRedemptions      *int       `json:"max_redemptions"`
	StartsAt            *time.Time `json:"starts_at"`
	ExpiresAt           *time.Time `json:"expires_at"`
	IsActive            bool       `json:"is_active"`
}

func (r promoCodeRequest) toInput() domain.PromoCodeInput {
	return domain.PromoCodeInput{
		Code:                r.Code,
		PercentOff:          r.PercentOff,
		MinOrderAmountCents: r.MinOrderAmountCents,
		MaxRedemptions:      r.MaxRedemptions,
		StartsAt:            r.StartsAt,
		ExpiresAt:           r.ExpiresAt,
		IsActive:            r.IsActive,
	}
}

type promoCodeResponse struct {
	ID                  string     `json:"id"`
	Code                string     `json:"code"`
	PercentOff          int        `json:"percent_off"`
	MinOrderAmountCents int64      `json:"min_order_amount_cents"`
	MaxRedemptions      *int       `json:"max_redemptions"`
	RedemptionsUsed     int        `json:"redemptions_used"`
	StartsAt            time.Time  `json:"starts_at"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
	IsActive            bool       `json:"is_active"`
	CreatedAt           time.Time  `json:"created_at"`
}

func toResponse(p *domain.PromoCode) promoCodeResponse {
	return promoCodeResponse{
		ID:                  p.ID,
		Code:                p.Code,
		PercentOff:          p.PercentOff,
		MinOrderAmountCents: p.MinOrderAmountCents,
		MaxRedemptions:      p.MaxRedemptions,
		RedemptionsUsed:     p.RedemptionsUsed,
		StartsAt:            p.StartsAt,
		ExpiresAt:           p.ExpiresAt,
		IsActive:            p.IsActive,
		CreatedAt:           p.CreatedAt,
	}
}

func toResponseList(promos []domain.PromoCode) []promoCodeResponse {
	out := make([]promoCodeResponse, 0, len(promos))
	for i := range promos {
		out = append(out, toResponse(&promos[i]))
	}
	return out
}

type previewResponse struct {
	Valid         bool   `json:"valid"`
	Code          string `json:"code"`
	PercentOff    int    `json:"percent_off"`
	DiscountCents int64  `json:"discount_cents"`
}

type redeemRequest struct {
	Code             string `json:"code"`
	OrderID          string `json:"order_id"`
	OrderAmountCents int64  `json:"order_amount_cents"`
}

type redeemResponse struct {
	DiscountCents int64  `json:"discount_cents"`
	Code          string `json:"code"`
}

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	reqID, _ := r.Context().Value(ctxKeyRequestID).(string)
	writeJSON(w, status, errorResponse{Error: msg, RequestID: reqID})
}

func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var derr *domain.Error
	if errors.As(err, &derr) {
		status := map[domain.ErrorCode]int{
			domain.ErrCodeValidation: http.StatusBadRequest,
			domain.ErrCodeNotFound:   http.StatusNotFound,
			domain.ErrCodeForbidden:  http.StatusForbidden,
			domain.ErrCodeConflict:   http.StatusConflict,
		}[derr.Code]
		if status == 0 {
			status = http.StatusInternalServerError
		}
		writeError(w, r, status, derr.Message)
		return
	}
	writeError(w, r, http.StatusInternalServerError, "internal server error")
}
