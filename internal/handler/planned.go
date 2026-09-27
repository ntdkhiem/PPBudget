package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/internal/middleware"
)

// maxPlannedLabel keeps a label to something a list row can show.
const maxPlannedLabel = 120

// ListPlannedExpenses returns the user's planned expenses, soonest first.
func (h *Handler) ListPlannedExpenses(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	expenses, err := h.svc.ListPlannedExpenses(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list planned expenses", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list planned expenses")
		return
	}
	if expenses == nil {
		expenses = []domain.PlannedExpense{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"expenses": expenses})
}

// UpsertPlannedExpense creates a planned expense (POST) or replaces one
// (PUT /{id}).
//
// Each one is cash the plan holds back, so a wrong amount here quietly moves
// the emergency fund's gap and every sweep. Refusing a malformed row with a
// reason beats storing it.
func (h *Handler) UpsertPlannedExpense(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var e domain.PlannedExpense
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	// The route decides between create and update, never the body.
	e.ID = ""
	if id := chi.URLParam(r, "id"); id != "" {
		if !validID(w, id, "planned expense id") {
			return
		}
		e.ID = id
	}

	e.Label = strings.TrimSpace(e.Label)
	switch {
	case e.Label == "":
		writeError(w, http.StatusBadRequest, "a planned expense needs a label")
		return
	case utf8.RuneCountInString(e.Label) > maxPlannedLabel:
		writeError(w, http.StatusBadRequest, "keep the label under 120 characters")
		return
	case e.Amount <= 0:
		writeError(w, http.StatusBadRequest, "a planned expense needs an amount above zero")
		return
	case e.TargetDate.IsZero():
		writeError(w, http.StatusBadRequest, "a planned expense needs a date")
		return
	}
	// A calendar day, not an instant: whatever time of day arrived, the day
	// the client meant is the one stored.
	y, m, d := e.TargetDate.Date()
	e.TargetDate = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)

	var err error
	id := e.ID
	if id == "" {
		id, err = h.svc.CreatePlannedExpense(r.Context(), userID, e)
	} else {
		err = h.svc.UpdatePlannedExpense(r.Context(), userID, e)
	}
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			writeError(w, http.StatusNotFound, "planned expense not found")
		case errors.Is(err, apperrors.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("failed to save planned expense", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save planned expense")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// DeletePlannedExpense removes one planned expense.
func (h *Handler) DeletePlannedExpense(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	if !validID(w, id, "planned expense id") {
		return
	}

	if err := h.svc.DeletePlannedExpense(r.Context(), userID, id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "planned expense not found")
			return
		}
		h.logger.Error("failed to delete planned expense", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete planned expense")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
