package handler

import (
	"net/http"
	"strconv"

	"ntdkhiem/ppbudget-go/internal/middleware"
)

// GetWealthProgress returns the progress timeline, newest first. ?limit=N
// caps it; the service bounds whatever is asked for.
func (h *Handler) GetWealthProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive number")
			return
		}
		limit = n
	}

	events, err := h.svc.ListProgress(r.Context(), userID, limit)
	if err != nil {
		h.logger.Error("failed to list progress", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list progress")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
