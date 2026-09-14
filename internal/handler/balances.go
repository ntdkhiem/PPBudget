package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// validID rejects malformed ids before they reach Postgres, which would otherwise
// fail the uuid cast and surface as a 500.
func validID(w http.ResponseWriter, id, name string) bool {
	if id == "" {
		writeError(w, http.StatusBadRequest, name+" is required")
		return false
	}
	if uuid.Validate(id) != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return false
	}
	return true
}

// ListBalanceSnapshots handles GET /accounts/{id}/balances.
func (h *Handler) ListBalanceSnapshots(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accountID := chi.URLParam(r, "id")
	if !validID(w, accountID, "account id") {
		return
	}

	snapshots, err := h.svc.ListBalanceSnapshots(r.Context(), userID, accountID)
	if err != nil {
		writeBalanceError(h, w, err, "failed to fetch balance snapshots")
		return
	}
	if snapshots == nil {
		snapshots = []domain.BalanceSnapshot{}
	}
	writeJSON(w, http.StatusOK, snapshots)
}

// RecordManualBalance handles POST /accounts/{id}/balances.
func (h *Handler) RecordManualBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accountID := chi.URLParam(r, "id")
	if !validID(w, accountID, "account id") {
		return
	}

	var body struct {
		Balance  *int64  `json:"balance"`
		AsOfDate *string `json:"as_of_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if body.Balance == nil {
		writeError(w, http.StatusBadRequest, "balance is required")
		return
	}

	var asOf *time.Time
	if body.AsOfDate != nil && *body.AsOfDate != "" {
		t, err := time.Parse("2006-01-02", *body.AsOfDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid as_of_date format, expected YYYY-MM-DD")
			return
		}
		asOf = &t
	}

	err := h.svc.RecordManualBalance(r.Context(), userID, accountID, *body.Balance, asOf)
	if err != nil {
		writeBalanceError(h, w, err, "failed to record balance")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// DeleteBalanceSnapshot handles DELETE /accounts/{id}/balances/{snapshotId}.
func (h *Handler) DeleteBalanceSnapshot(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accountID := chi.URLParam(r, "id")
	if !validID(w, accountID, "account id") {
		return
	}
	snapshotID := chi.URLParam(r, "snapshotId")
	if !validID(w, snapshotID, "snapshot id") {
		return
	}

	err := h.svc.DeleteBalanceSnapshot(r.Context(), userID, accountID, snapshotID)
	if err != nil {
		writeBalanceError(h, w, err, "failed to delete balance snapshot")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeBalanceError maps service/repository errors to HTTP responses for the
// balance snapshot endpoints. ErrForbidden is mapped to the same 404 used for
// ErrNotFound (rather than 403) so a request against another user's account
// doesn't leak whether that account exists.
func writeBalanceError(h *Handler, w http.ResponseWriter, err error, genericMsg string) {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, apperrors.ErrForbidden):
		writeError(w, http.StatusNotFound, "account not found")
	case errors.Is(err, apperrors.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		if h.logger != nil {
			h.logger.Error("balance snapshot request failed", "error", err)
		}
		writeError(w, http.StatusInternalServerError, genericMsg)
	}
}
