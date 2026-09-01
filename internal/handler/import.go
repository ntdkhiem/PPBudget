package handler

import (
	"encoding/json"
	"net/http"

	"ntdkhiem/ppbudget-go/internal/middleware"
	"ntdkhiem/ppbudget-go/internal/service"
)

func (h *Handler) SimpleFinClaim(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.SimplefinClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.SetupToken == "" {
		writeError(w, http.StatusBadRequest, "missing setup_token")
		return
	}

	resp, err := h.svc.SimpleFinClaim(r.Context(), userID, req)
	if err != nil {
		h.logger.Error("failed to claim simplefin token", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "failed to claim token")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SimpleFinFetchAccounts(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.SimplefinFetchAccountsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.AccessURL == "" {
		writeError(w, http.StatusBadRequest, "missing access_url")
		return
	}

	resp, err := h.svc.SimpleFinFetchAccounts(r.Context(), userID, req)
	if err != nil {
		h.logger.Error("failed to fetch simplefin accounts", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "failed to fetch accounts")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SimpleFinExecute(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.SimplefinExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.AccessURL == "" {
		writeError(w, http.StatusBadRequest, "missing access_url")
		return
	}

	err := h.svc.SimpleFinExecute(r.Context(), userID, req)
	if err != nil {
		h.logger.Error("failed to execute simplefin import", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "failed to execute import")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Import started successfully"})
}

func (h *Handler) SimpleFinStatus(w http.ResponseWriter, r *http.Request) {
	service.ImportProgress.RLock()
	defer service.ImportProgress.RUnlock()

	writeJSON(w, http.StatusOK, service.ImportProgress)
}

func (h *Handler) SimpleFinConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Read from db
	b, err := h.svc.GetSimplefinConfig(r.Context(), userID)
	if err != nil || b == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"connected": false})
		return
	}

	var config struct {
		AccessToken    string            `json:"access_token"`
		AccountMapping map[string]string `json:"account_mapping"`
		ImportPending  bool              `json:"import_pending"`
		ApplyRules     bool              `json:"apply_rules"`
		ContentDedup   bool              `json:"content_dedup"`
		AutoSync       bool              `json:"auto_sync"`
	}
	if err := json.Unmarshal([]byte(b), &config); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"connected": false})
		return
	}

	if config.AccessToken == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"connected": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connected":       true,
		"access_token":    config.AccessToken,
		"account_mapping": config.AccountMapping,
		"import_pending":  config.ImportPending,
		"apply_rules":     config.ApplyRules,
		"content_dedup":   config.ContentDedup,
		"auto_sync":       config.AutoSync,
		"next_sync_time":  h.svc.GetNextAutoSync(),
	})
}

func (h *Handler) SimpleFinAutoSyncToggle(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if err := h.svc.UpdateSimplefinAutoSync(r.Context(), userID, req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update simplefin config")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"auto_sync": req.Enabled,
	})
}

func (h *Handler) SimpleFinCronTrigger(w http.ResponseWriter, r *http.Request) {
	// Verify API key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" || apiKey != h.cfg.IngestAPIKey {
		writeError(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	err := h.svc.RunAutoSync(r.Context())
	if err != nil {
		h.logger.Error("cron triggered auto-sync failed", "error", err)
		writeError(w, http.StatusInternalServerError, "auto-sync failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}
