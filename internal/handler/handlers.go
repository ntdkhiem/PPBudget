package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"ntdkhiem/firefly-go/internal/domain"
	"time"

	"ntdkhiem/firefly-go/internal/config"
	"ntdkhiem/firefly-go/internal/service"

	apperrors "ntdkhiem/firefly-go/internal/errors"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

type Handler struct {
	svc    *service.Service
	logger *slog.Logger
	cfg    *config.Config
}

func New(svc *service.Service, logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{svc: svc, logger: logger, cfg: cfg}
}

func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	var req service.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.SimplefinAccountID == "" || req.SimplefinTxID == "" || req.Amount == "" || req.Date == "" {
		writeError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	err := h.svc.Ingest(r.Context(), req)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to ingest transaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req service.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	err := h.svc.CreateTransfer(r.Context(), req)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create transfer")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "transfer created"})
}

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")

	// Handle optional cursor pagination
	var cursorDate *time.Time
	var cursorID *string
	if dateStr := r.URL.Query().Get("cursor_date"); dateStr != "" {
		t, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			cursorDate = &t
			id := r.URL.Query().Get("cursor_id")
			cursorID = &id
		}
	}

	txns, err := h.svc.ListTransactions(r.Context(), accountID, cursorDate, cursorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch transactions")
		return
	}

	writeJSON(w, http.StatusOK, txns)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	// Simple Tier 1 password check
	if body.Password != h.cfg.AdminPassword {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Generate JWT
	claims := jwt.MapClaims{
		"role": "admin",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": tokenString})
}

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.svc.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch accounts")
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (h *Handler) ReviewTransaction(w http.ResponseWriter, r *http.Request) {
	txnID := chi.URLParam(r, "id")
	if txnID == "" {
		writeError(w, http.StatusBadRequest, "transaction id is required")
		return
	}

	var body struct {
		CategoryID *string `json:"category_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) // allow empty body

	err := h.svc.ReviewTransaction(r.Context(), txnID, body.CategoryID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to review transaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reviewed"})
}

// Helpers
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// --- Categories & Rules ---

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.svc.ListCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.svc.ListRulesDetailed(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule domain.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := h.svc.CreateRule(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) GetRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := h.svc.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get rule")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var rule domain.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	rule.ID = id
	if err := h.svc.UpdateRule(r.Context(), &rule); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteRule(r.Context(), id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ApplyRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var payload struct {
		RunAll    bool   `json:"run_all"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	var sDate, eDate *time.Time
	if !payload.RunAll {
		if payload.StartDate != "" {
			t, err := time.Parse(time.RFC3339, payload.StartDate)
			if err == nil {
				sDate = &t
			}
		}
		if payload.EndDate != "" {
			t, err := time.Parse(time.RFC3339, payload.EndDate)
			if err == nil {
				eDate = &t
			}
		}
	}

	updatedCount, err := h.svc.ApplyRule(r.Context(), id, payload.RunAll, sDate, eDate)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to apply rule")
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"updated_count": updatedCount})
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if body.Name == "" || body.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}

	err := h.svc.CreateCategory(r.Context(), body.Name, body.Type)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create category")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	err := h.svc.UpdateCategory(r.Context(), id, body.Name, body.Type)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "category not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update category")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.svc.DeleteCategory(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "category not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete category")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Budgets ---

func (h *Handler) CreateBudget(w http.ResponseWriter, r *http.Request) {
	var budget domain.Budget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := h.svc.CreateBudget(r.Context(), &budget); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create budget")
		return
	}
	writeJSON(w, http.StatusCreated, budget)
}

func (h *Handler) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var budget domain.Budget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	budget.ID = id
	if err := h.svc.UpdateBudget(r.Context(), &budget); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "budget not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update budget")
		return
	}
	writeJSON(w, http.StatusOK, budget)
}

func (h *Handler) DeleteBudget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteBudget(r.Context(), id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "budget not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete budget")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) GetBudgetsSummary(w http.ResponseWriter, r *http.Request) {
	monthStr := r.URL.Query().Get("month")
	if monthStr == "" {
		writeError(w, http.StatusBadRequest, "month is required")
		return
	}

	month, err := time.Parse("2006-01", monthStr)
	if err != nil {
		month, err = time.Parse("2006-01-02", monthStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid month format")
			return
		}
	}

	budgets, err := h.svc.GetBudgetsSummary(r.Context(), month)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch budgets summary")
		return
	}
	writeJSON(w, http.StatusOK, budgets)
}

// --- Accounts ---

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string `json:"name"`
		Type           string `json:"type"`
		Currency       string `json:"currency"`
		InitialBalance int64  `json:"initial_balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if body.Currency == "" {
		body.Currency = "USD"
	}

	err := h.svc.CreateAccount(r.Context(), body.Name, body.Type, body.Currency, body.InitialBalance)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create account")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	account, err := h.svc.GetAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get account")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var body struct {
		Name           string `json:"name"`
		Type           string `json:"type"`
		Currency       string `json:"currency"`
		InitialBalance int64  `json:"initial_balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	err := h.svc.UpdateAccount(r.Context(), id, body.Name, body.Type, body.Currency, body.InitialBalance)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update account")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.svc.DeleteAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Transactions (Manual) ---

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID   string  `json:"account_id"`
		Amount      int64   `json:"amount"`
		Date        string  `json:"date"`
		Description string  `json:"description"`
		CategoryID  *string `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	date, err := time.Parse("2006-01-02", body.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format")
		return
	}

	err = h.svc.CreateTransaction(r.Context(), body.AccountID, body.Amount, date, body.Description, body.CategoryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create transaction")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.svc.DeleteTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete transaction")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var body struct {
		AccountID   string  `json:"account_id"`
		Amount      int64   `json:"amount"`
		Date        string  `json:"date"`
		Description string  `json:"description"`
		CategoryID  *string `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	date, err := time.Parse("2006-01-02", body.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format")
		return
	}

	err = h.svc.UpdateTransaction(r.Context(), id, body.AccountID, body.Amount, date, body.Description, body.CategoryID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update transaction")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Reports ---

func (h *Handler) GetNetWorthTrend(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	endDate := time.Now()
	startDate := endDate.AddDate(0, -6, 0)

	if startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = parsed
		}
	}

	points, err := h.svc.GetNetWorthTrend(r.Context(), startDate, endDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch net worth trend")
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (h *Handler) GetSpendingByCategory(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endDate := now

	if startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = parsed
		}
	}

	spending, err := h.svc.GetSpendingByCategory(r.Context(), startDate, endDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch spending")
		return
	}
	writeJSON(w, http.StatusOK, spending)
}
