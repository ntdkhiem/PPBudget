package handler

import (
	"net/http"
	"strconv"

	"ntdkhiem/ppbudget-go/internal/middleware"
)

func (h *Handler) GlobalSearch(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"query":   "",
			"results": map[string]interface{}{},
		})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	result, err := h.svc.GlobalSearch(r.Context(), userID, query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query":   query,
		"results": result,
	})
}
