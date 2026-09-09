package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ntdkhiem/ppbudget-go/internal/auth"
	"ntdkhiem/ppbudget-go/internal/middleware"
)


// ChangePassword allows the user to change their password
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	user, err := h.svc.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	if !auth.CheckPasswordHash(req.OldPassword, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "incorrect old password")
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash new password")
		return
	}

	if err := h.svc.UpdateUserPassword(r.Context(), userID, newHash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

// ExportTransactionsCSV exports all user transactions in CSV format
func (h *Handler) ExportTransactionsCSV(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	txns, err := h.svc.ExportTransactions(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export transactions")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="transactions.csv"`)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"Date", "Description", "Amount", "Account", "Category"})

	// Write rows
	for _, txn := range txns {
		writer.Write([]string{
			txn.Date.Format("2006-01-02"),
			txn.Description,
			fmt.Sprintf("%.2f", float64(txn.Amount)/100.0),
			txn.Account,
			txn.Category,
		})
	}
}

// DeleteUserAccount securely wipes all of the user's data
func (h *Handler) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.svc.DeleteUserAccount(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "account deleted"})
}

// ExportAllDataJSON exports all user data in JSON format
func (h *Handler) ExportAllDataJSON(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	typesParam := r.URL.Query().Get("types")
	var types []string
	if typesParam != "" {
		types = strings.Split(typesParam, ",")
	}

	data, err := h.svc.ExportAllData(r.Context(), userID, types)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export data")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="ppbudget_backup.json"`)

	json.NewEncoder(w).Encode(data)
}

