package service

import (
	"context"

	"log/slog"
	"strings"
	"sync"
	"time"

	"ntdkhiem/ppbudget-go/internal/config"
	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/internal/repository"
	"ntdkhiem/ppbudget-go/pkg/money"
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

func (s *Service) GetTransaction(ctx context.Context, userID, id string) (*domain.Transaction, error) {
	return s.repo.GetTransaction(ctx, userID, id)
}

func (s *Service) ListTransactions(ctx context.Context, f domain.TransactionFilter) ([]domain.TransactionWithBalance, error) {
	return s.repo.ListTransactions(ctx, f)
}

func (s *Service) ReviewTransaction(ctx context.Context, userID, txnID string, categoryID *string) error {
	return s.repo.MarkReviewed(ctx, userID, txnID, categoryID)
}

func (s *Service) ListAccounts(ctx context.Context, userID string) ([]domain.Account, error) {
	return s.repo.ListAccounts(ctx, userID)
}

// Categories

func (s *Service) CreateCategory(ctx context.Context, userID, name, catType string) error {
	return s.repo.CreateCategory(ctx, userID, name, catType)
}

func (s *Service) UpdateCategory(ctx context.Context, userID, id, name, catType string) error {
	return s.repo.UpdateCategory(ctx, userID, id, name, catType)
}

func (s *Service) DeleteCategory(ctx context.Context, userID, id string) error {
	return s.repo.DeleteCategory(ctx, userID, id)
}

func (s *Service) ListCategories(ctx context.Context, userID string) ([]domain.Category, error) {
	return s.repo.ListCategories(ctx, userID)
}

// Rules

func (s *Service) ListRulesDetailed(ctx context.Context, userID string) ([]domain.Rule, error) {
	return s.repo.ListRulesDetailed(ctx, userID)
}

func (s *Service) CreateRule(ctx context.Context, userID string, rule *domain.Rule) error {
	rule.UserID = userID
	return s.repo.CreateRule(ctx, rule)
}

func (s *Service) UpdateRule(ctx context.Context, userID string, rule *domain.Rule) error {
	rule.UserID = userID
	return s.repo.UpdateRule(ctx, rule)
}

func (s *Service) GetRule(ctx context.Context, userID, id string) (*domain.Rule, error) {
	return s.repo.GetRule(ctx, userID, id)
}

func (s *Service) DeleteRule(ctx context.Context, userID, id string) error {
	return s.repo.DeleteRule(ctx, userID, id)
}

// Budgets

func (s *Service) CreateBudget(ctx context.Context, userID string, budget *domain.Budget) error {
	budget.UserID = userID
	return s.repo.CreateBudget(ctx, budget)
}

func (s *Service) UpdateBudget(ctx context.Context, userID string, budget *domain.Budget) error {
	budget.UserID = userID
	return s.repo.UpdateBudget(ctx, budget)
}

func (s *Service) DeleteBudget(ctx context.Context, userID, id string, allMonths bool) error {
	return s.repo.DeleteBudget(ctx, userID, id, allMonths)
}

func (s *Service) GetBudgetsSummary(ctx context.Context, userID string, month time.Time) ([]domain.BudgetSummary, error) {
	return s.repo.GetBudgetsSummary(ctx, userID, month)
}

// Accounts

func (s *Service) CreateAccount(ctx context.Context, userID, name, accType, currency string, initialBalance int64) error {
	return s.repo.CreateAccount(ctx, userID, name, accType, currency, initialBalance)
}

func (s *Service) GetAccount(ctx context.Context, userID, id string) (*domain.Account, error) {
	return s.repo.GetAccount(ctx, userID, id)
}

func (s *Service) UpdateAccount(ctx context.Context, userID, id, name, accType, currency string, initialBalance int64) error {
	return s.repo.UpdateAccount(ctx, userID, id, name, accType, currency, initialBalance)
}

func (s *Service) DeleteAccount(ctx context.Context, userID, id string) error {
	return s.repo.DeleteAccount(ctx, userID, id)
}

// Transactions (Manual)

func (s *Service) CreateTransaction(ctx context.Context, userID, accountID string, amount int64, date time.Time, description string, notes *string, categoryID *string, subscriptionID *string) error {
	return s.repo.CreateTransaction(ctx, userID, accountID, amount, date, description, notes, categoryID, subscriptionID)
}

func (s *Service) DeleteTransaction(ctx context.Context, userID, id string) error {
	return s.repo.DeleteTransaction(ctx, userID, id)
}

func (s *Service) BulkDeleteTransactions(ctx context.Context, userID string, ids []string) error {
	return s.repo.BulkDeleteTransactions(ctx, userID, ids)
}

func (s *Service) BulkUpdateTransactionsCategory(ctx context.Context, userID string, ids []string, categoryID string) error {
	return s.repo.BulkUpdateTransactionsCategory(ctx, userID, ids, categoryID)
}

func (s *Service) UpdateTransaction(ctx context.Context, userID, id, accountID string, amount int64, date time.Time, description string, notes *string, categoryID *string, subscriptionID *string, paysFor []domain.TransactionLink, paidBy []domain.TransactionLink) error {
	return s.repo.UpdateTransaction(ctx, userID, id, accountID, amount, date, description, notes, categoryID, subscriptionID, paysFor, paidBy)
}

func (s *Service) ApplyRule(ctx context.Context, userID, ruleID string, runAll bool, startDate, endDate *time.Time) (int, error) {
	rule, err := s.repo.GetRule(ctx, userID, ruleID)
	if err != nil {
		return 0, err
	}

	var sDate, eDate *time.Time
	if !runAll {
		sDate = startDate
		eDate = endDate
	}

	txns, err := s.repo.GetTransactionsByDateRange(ctx, userID, sDate, eDate)
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
				err = s.repo.UpdateTransaction(ctx, userID, t.ID, newAccID, t.Amount.ToInt64(), t.Date, t.Description, t.Notes, newCatID, newSubID, t.PaysFor, t.PaidBy)
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

func (s *Service) GetNetWorthTrend(ctx context.Context, userID string, startDate, endDate time.Time) ([]domain.NetWorthPoint, error) {
	return s.repo.GetNetWorthTrend(ctx, userID, startDate, endDate)
}

func (s *Service) GetSpendingByCategory(ctx context.Context, userID string, startDate, endDate time.Time) ([]domain.CategorySpend, error) {
	return s.repo.GetSpendingByCategory(ctx, userID, startDate, endDate)
}

func (s *Service) GetReportsSummary(ctx context.Context, userID string, startDate, endDate time.Time) (*domain.ReportsSummary, error) {
	return s.repo.GetReportsSummary(ctx, userID, startDate, endDate)
}
