package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"ntdkhiem/ppbudget-go/internal/config"
	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/internal/repository"
	"ntdkhiem/ppbudget-go/pkg/money"

	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

type Service struct {
	repo   *repository.Repository
	logger *slog.Logger
	cfg    *config.Config

	mu           sync.RWMutex
	nextAutoSync time.Time
}

func New(repo *repository.Repository, logger *slog.Logger, cfg *config.Config) *Service {
	return &Service{repo: repo, logger: logger, cfg: cfg}
}

func (s *Service) SetNextAutoSync(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAutoSync = t
}

func (s *Service) GetNextAutoSync() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextAutoSync
}

type IngestRequest struct {
	SimplefinAccountID string `json:"simplefin_account_id"`
	SimplefinTxID      string `json:"simplefin_transaction_id"`
	Amount             string `json:"amount"`
	Date               string `json:"date"`
	Description        string `json:"description"`
}

func (s *Service) Ingest(ctx context.Context, req IngestRequest) error {
	// 1. Parse Amount and Date
	amount, err := money.NewFromString(req.Amount)
	if err != nil {
		return fmt.Errorf("%w: invalid amount", apperrors.ErrInvalidInput)
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return fmt.Errorf("%w: invalid date format, expected YYYY-MM-DD", apperrors.ErrInvalidInput)
	}

	// 2. Start DB Transaction
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 3. Look up internal account ID
	accountID, err := s.repo.GetAccountBySimplefinID(ctx, tx, req.SimplefinAccountID)
	if err != nil {
		if err == apperrors.ErrNotFound {
			return fmt.Errorf("account not found for simplefin_id: %s", req.SimplefinAccountID)
		}
		return err
	}

	// Auto-categorization
	var categoryID *string
	var subscriptionID *string
	isReviewed := false

	rules, err := s.repo.ListRulesDetailed(ctx)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		match := false
		if rule.Strictness == "all" || len(rule.Conditions) == 0 {
			match = true
		}

		for _, cond := range rule.Conditions {
			condMatch := false
			if cond.Field == "description" {
				switch cond.Operator {
				case "contains":
					condMatch = strings.Contains(strings.ToLower(req.Description), strings.ToLower(cond.Value))
				case "is_exactly":
					condMatch = req.Description == cond.Value
				case "starts_with":
					condMatch = strings.HasPrefix(req.Description, cond.Value)
				case "ends_with":
					condMatch = strings.HasSuffix(req.Description, cond.Value)
				}
			} else if cond.Field == "amount" {
				// simple numerical check for amount
				condAmt, _ := money.NewFromString(cond.Value)
				if cond.Operator == "greater_than" {
					condMatch = amount.ToInt64() > condAmt.ToInt64()
				} else if cond.Operator == "less_than" {
					condMatch = amount.ToInt64() < condAmt.ToInt64()
				} else if cond.Operator == "is_exactly" {
					condMatch = amount.ToInt64() == condAmt.ToInt64()
				}
			} else if cond.Field == "source_account" {
				if cond.Operator == "is_exactly" {
					condMatch = req.SimplefinAccountID == cond.Value
				}
			}

			if rule.Strictness == "all" {
				if !condMatch {
					match = false
					break
				}
			} else { // "any"
				if condMatch {
					match = true
					break
				}
			}
		}

		if match {
			for _, act := range rule.Actions {
				if act.ActionType == "set_category" {
					catID := act.Value
					categoryID = &catID
				} else if act.ActionType == "link_to_subscription" {
					subID := act.Value
					subscriptionID = &subID
				}
			}
			isReviewed = true
			break
		}
	}

	// 4. Insert idempotently
	_, created, err := s.repo.InsertIngestedTransaction(ctx, tx, accountID, amount, date, req.Description, req.SimplefinTxID, categoryID, subscriptionID, isReviewed)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if created {
		s.logger.Info("transaction ingested", "simplefin_tx_id", req.SimplefinTxID, "amount", amount.String())
	} else {
		s.logger.Info("duplicate transaction ignored", "simplefin_tx_id", req.SimplefinTxID)
	}

	return nil
}

func (s *Service) GetTransaction(ctx context.Context, id string) (*domain.Transaction, error) {
	return s.repo.GetTransaction(ctx, id)
}

type TransferRequest struct {
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	Amount        string `json:"amount"`
	Date          string `json:"date"`
	Description   string `json:"description"`
}

func (s *Service) CreateTransfer(ctx context.Context, req TransferRequest) error {
	amount, err := money.NewFromString(req.Amount)
	if err != nil {
		return fmt.Errorf("%w: invalid amount", apperrors.ErrInvalidInput)
	}
	if amount.ToInt64() <= 0 {
		return fmt.Errorf("%w: transfer amount must be positive", apperrors.ErrInvalidInput)
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return fmt.Errorf("%w: invalid date format", apperrors.ErrInvalidInput)
	}

	return s.repo.CreateTransfer(ctx, req.FromAccountID, req.ToAccountID, amount, date, req.Description)
}

func (s *Service) ListTransactions(ctx context.Context, accountID string, cursorDate *time.Time, cursorID *string, unreviewedOnly bool, startDate, endDate *time.Time, search string) ([]domain.TransactionWithBalance, error) {
	return s.repo.ListTransactions(ctx, accountID, cursorDate, cursorID, unreviewedOnly, startDate, endDate, search)
}

func (s *Service) ReviewTransaction(ctx context.Context, txnID string, categoryID *string) error {
	return s.repo.MarkReviewed(ctx, txnID, categoryID)
}

func (s *Service) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	return s.repo.ListAccounts(ctx)
}

// Categories

func (s *Service) CreateCategory(ctx context.Context, name, catType string) error {
	return s.repo.CreateCategory(ctx, name, catType)
}

func (s *Service) UpdateCategory(ctx context.Context, id, name, catType string) error {
	return s.repo.UpdateCategory(ctx, id, name, catType)
}

func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	return s.repo.DeleteCategory(ctx, id)
}

func (s *Service) ListCategories(ctx context.Context) ([]domain.Category, error) {
	return s.repo.ListCategories(ctx)
}

// Rules

func (s *Service) ListRulesDetailed(ctx context.Context) ([]domain.Rule, error) {
	return s.repo.ListRulesDetailed(ctx)
}

func (s *Service) CreateRule(ctx context.Context, rule *domain.Rule) error {
	return s.repo.CreateRule(ctx, rule)
}

func (s *Service) UpdateRule(ctx context.Context, rule *domain.Rule) error {
	return s.repo.UpdateRule(ctx, rule)
}

func (s *Service) GetRule(ctx context.Context, id string) (*domain.Rule, error) {
	return s.repo.GetRule(ctx, id)
}

func (s *Service) DeleteRule(ctx context.Context, id string) error {
	return s.repo.DeleteRule(ctx, id)
}

// Budgets

func (s *Service) CreateBudget(ctx context.Context, budget *domain.Budget) error {
	return s.repo.CreateBudget(ctx, budget)
}

func (s *Service) UpdateBudget(ctx context.Context, budget *domain.Budget) error {
	return s.repo.UpdateBudget(ctx, budget)
}

func (s *Service) DeleteBudget(ctx context.Context, id string) error {
	return s.repo.DeleteBudget(ctx, id)
}

func (s *Service) GetBudgetsSummary(ctx context.Context, month time.Time) ([]domain.BudgetSummary, error) {
	return s.repo.GetBudgetsSummary(ctx, month)
}

// Accounts

func (s *Service) CreateAccount(ctx context.Context, name, accType, currency string, initialBalance int64) error {
	return s.repo.CreateAccount(ctx, name, accType, currency, initialBalance)
}

func (s *Service) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	return s.repo.GetAccount(ctx, id)
}

func (s *Service) UpdateAccount(ctx context.Context, id, name, accType, currency string, initialBalance int64) error {
	return s.repo.UpdateAccount(ctx, id, name, accType, currency, initialBalance)
}

func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	return s.repo.DeleteAccount(ctx, id)
}

// Transactions (Manual)

func (s *Service) CreateTransaction(ctx context.Context, accountID string, amount int64, date time.Time, description string, notes *string, categoryID *string, subscriptionID *string, linkedTransactionID *string) error {
	return s.repo.CreateTransaction(ctx, accountID, amount, date, description, notes, categoryID, subscriptionID, linkedTransactionID)
}

func (s *Service) DeleteTransaction(ctx context.Context, id string) error {
	return s.repo.DeleteTransaction(ctx, id)
}

func (s *Service) BulkDeleteTransactions(ctx context.Context, ids []string) error {
	return s.repo.BulkDeleteTransactions(ctx, ids)
}

func (s *Service) BulkUpdateTransactionsCategory(ctx context.Context, ids []string, categoryID string) error {
	return s.repo.BulkUpdateTransactionsCategory(ctx, ids, categoryID)
}

func (s *Service) UpdateTransaction(ctx context.Context, id, accountID string, amount int64, date time.Time, description string, notes *string, categoryID *string, subscriptionID *string, paysFor []domain.TransactionLink, paidBy []domain.TransactionLink) error {
	return s.repo.UpdateTransaction(ctx, id, accountID, amount, date, description, notes, categoryID, subscriptionID, paysFor, paidBy)
}

func (s *Service) ApplyRule(ctx context.Context, ruleID string, runAll bool, startDate, endDate *time.Time) (int, error) {
	rule, err := s.repo.GetRule(ctx, ruleID)
	if err != nil {
		return 0, err
	}

	var sDate, eDate *time.Time
	if !runAll {
		sDate = startDate
		eDate = endDate
	}

	txns, err := s.repo.GetTransactionsByDateRange(ctx, sDate, eDate)
	if err != nil {
		return 0, err
	}

	updatedCount := 0
	for _, t := range txns {
		match := false
		if rule.Strictness == "all" || len(rule.Conditions) == 0 {
			match = true
		}

		sfAccount := ""
		if t.SimplefinAccountID != nil {
			sfAccount = *t.SimplefinAccountID
		}

		for _, cond := range rule.Conditions {
			condMatch := false
			if cond.Field == "description" {
				switch cond.Operator {
				case "contains":
					condMatch = strings.Contains(strings.ToLower(t.Description), strings.ToLower(cond.Value))
				case "is_exactly":
					condMatch = t.Description == cond.Value
				case "starts_with":
					condMatch = strings.HasPrefix(t.Description, cond.Value)
				case "ends_with":
					condMatch = strings.HasSuffix(t.Description, cond.Value)
				}
			} else if cond.Field == "amount" {
				condAmt, err := money.NewFromString(cond.Value)
				if err == nil {
					if cond.Operator == "greater_than" {
						condMatch = t.Amount.ToInt64() > condAmt.ToInt64()
					} else if cond.Operator == "less_than" {
						condMatch = t.Amount.ToInt64() < condAmt.ToInt64()
					} else if cond.Operator == "is_exactly" {
						condMatch = t.Amount.ToInt64() == condAmt.ToInt64()
					}
				}
			} else if cond.Field == "source_account" {
				if cond.Operator == "is_exactly" {
					condMatch = sfAccount == cond.Value
				}
			}

			if rule.Strictness == "all" {
				if !condMatch {
					match = false
					break
				}
			} else { // "any"
				if condMatch {
					match = true
					break
				}
			}
		}

		if match {
			var newCatID *string = t.CategoryID
			var newSubID *string = t.SubscriptionID
			var newAccID string = t.AccountID
			needsUpdate := false

			for _, act := range rule.Actions {
				if act.ActionType == "set_category" {
					if act.Value != "" {
						catID := act.Value
						if newCatID == nil || *newCatID != catID {
							newCatID = &catID
							needsUpdate = true
						}
					}
				}
				if act.ActionType == "set_account" {
					if act.Value != "" && act.Value != newAccID {
						newAccID = act.Value
						needsUpdate = true
					}
				}
				if act.ActionType == "link_to_subscription" {
					if act.Value != "" {
						subID := act.Value
						if newSubID == nil || *newSubID != subID {
							newSubID = &subID
							needsUpdate = true
						}
					}
				}
			}

			if needsUpdate {
				err = s.repo.UpdateTransaction(ctx, t.ID, newAccID, t.Amount.ToInt64(), t.Date, t.Description, t.Notes, newCatID, newSubID, t.PaysFor, t.PaidBy)
				if err != nil {
					continue
				}
				updatedCount++
			}
		}
	}

	return updatedCount, nil
}

// Reports

func (s *Service) GetNetWorthTrend(ctx context.Context, startDate, endDate time.Time) ([]domain.NetWorthPoint, error) {
	return s.repo.GetNetWorthTrend(ctx, startDate, endDate)
}

func (s *Service) GetSpendingByCategory(ctx context.Context, startDate, endDate time.Time) ([]domain.CategorySpend, error) {
	return s.repo.GetSpendingByCategory(ctx, startDate, endDate)
}

func (s *Service) GetReportsSummary(ctx context.Context, startDate, endDate time.Time) (*domain.ReportsSummary, error) {
	return s.repo.GetReportsSummary(ctx, startDate, endDate)
}
