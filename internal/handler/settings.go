package handler

import (
	"net/http"

	"ntdkhiem/ppbudget-go/internal/middleware"
)

// GenerateAPIToken generates a new API token for the authenticated user
func (h *Handler) GenerateAPIToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token, err := h.svc.GenerateAPIToken(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to generate api token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate API token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":   token,
		"message": "API token generated successfully",
	})
}

// GetAPIToken retrieves the API token for the authenticated user
func (h *Handler) GetAPIToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token, err := h.svc.GetAPIToken(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get api token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch API token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}
