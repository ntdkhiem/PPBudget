package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/internal/middleware"
)

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx := r.Context()
	subs, err := h.svc.ListSubscriptions(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list subscriptions")
		return
	}
	if subs == nil {
		subs = []domain.Subscription{} // ensure we don't return null
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Name            string  `json:"name"`
		Amount          int64   `json:"amount"` // in cents
		BillingCycle    string  `json:"billing_cycle"`
		NextBillingDate string  `json:"next_billing_date"`
		CategoryID      *string `json:"category_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if body.Name == "" || body.Amount <= 0 || (body.BillingCycle != "weekly" && body.BillingCycle != "monthly" && body.BillingCycle != "yearly") || body.NextBillingDate == "" {
		writeError(w, http.StatusBadRequest, "invalid or missing required fields")
		return
	}

	nextDate, err := time.Parse("2006-01-02", body.NextBillingDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
		return
	}

	ctx := r.Context()
	if err := h.svc.CreateSubscription(ctx, userID, body.Name, body.Amount, body.BillingCycle, nextDate, body.CategoryID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "success"})
}

func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	ctx := r.Context()
	if err := h.svc.DeleteSubscription(ctx, userID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete subscription")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var body struct {
		Name            string  `json:"name"`
		Amount          int64   `json:"amount"` // in cents
		BillingCycle    string  `json:"billing_cycle"`
		NextBillingDate string  `json:"next_billing_date"`
		CategoryID      *string `json:"category_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if body.Name == "" || body.Amount <= 0 || (body.BillingCycle != "weekly" && body.BillingCycle != "monthly" && body.BillingCycle != "yearly") || body.NextBillingDate == "" {
		writeError(w, http.StatusBadRequest, "invalid or missing required fields")
		return
	}

	nextDate, err := time.Parse("2006-01-02", body.NextBillingDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
		return
	}

	ctx := r.Context()
	if err := h.svc.UpdateSubscription(ctx, userID, id, body.Name, body.Amount, body.BillingCycle, nextDate, body.CategoryID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}
