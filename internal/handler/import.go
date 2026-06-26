package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"ntdkhiem/ppbudget-go/internal/service"
)

func (h *Handler) SimpleFinClaim(w http.ResponseWriter, r *http.Request) {
	var req service.SimplefinClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.SetupToken == "" {
		writeError(w, http.StatusBadRequest, "missing setup_token")
		return
	}

	resp, err := h.svc.SimpleFinClaim(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to claim simplefin token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to claim token")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SimpleFinFetchAccounts(w http.ResponseWriter, r *http.Request) {
	var req service.SimplefinFetchAccountsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.AccessURL == "" {
		writeError(w, http.StatusBadRequest, "missing access_url")
		return
	}

	resp, err := h.svc.SimpleFinFetchAccounts(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to fetch simplefin accounts", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch accounts")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SimpleFinExecute(w http.ResponseWriter, r *http.Request) {
	var req service.SimplefinExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.AccessURL == "" {
		writeError(w, http.StatusBadRequest, "missing access_url")
		return
	}

	err := h.svc.SimpleFinExecute(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to execute simplefin import", "error", err)
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
	// Read from simplefin.json
	b, err := os.ReadFile("simplefin.json")
	if err != nil {
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
	if err := json.Unmarshal(b, &config); err != nil {
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
	})
}

func (h *Handler) SimpleFinAutoSyncToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	b, err := os.ReadFile("simplefin.json")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read simplefin.json")
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(b, &config); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse simplefin.json")
		return
	}

	config["auto_sync"] = req.Enabled
	if out, err := json.MarshalIndent(config, "", "  "); err == nil {
		_ = os.WriteFile("simplefin.json", out, 0644)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"auto_sync": req.Enabled,
	})
}

