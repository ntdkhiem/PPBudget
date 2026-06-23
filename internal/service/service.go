package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ntdkhiem/firefly-go/internal/domain"
	"ntdkhiem/firefly-go/internal/repository"
	"ntdkhiem/firefly-go/pkg/money"

	apperrors "ntdkhiem/firefly-go/internal/errors"
)

type Service struct {
	repo   *repository.Repository
	logger *slog.Logger
}

func New(repo *repository.Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
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
	isReviewed := false

	rules, err := s.repo.ListRulesDetailed(ctx)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}
		
		match := true
		for _, cond := range rule.Conditions {
			if cond.Field == "description" {
				lowerDesc := strings.ToLower(req.Description)
				lowerVal := strings.ToLower(cond.Value)
				if cond.Operator == "contains" && !strings.Contains(lowerDesc, lowerVal) {
					match = false
					break
				}
				if cond.Operator == "equals" && lowerDesc != lowerVal {
					match = false
					break
				}
			}
		}

		if match {
			for _, act := range rule.Actions {
				if act.ActionType == "set_category" {
					catID := act.Value
					categoryID = &catID
				}
			}
			isReviewed = true
			break
		}
	}

	// 4. Insert idempotently
	created, err := s.repo.InsertIngestedTransaction(ctx, tx, accountID, amount, date, req.Description, req.SimplefinTxID, categoryID, isReviewed)
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

func (s *Service) ListTransactions(ctx context.Context, accountID string, cursorDate *time.Time, cursorID *string) ([]domain.TransactionWithBalance, error) {
	return s.repo.ListTransactions(ctx, accountID, cursorDate, cursorID)
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

func (s *Service) CreateTransaction(ctx context.Context, accountID string, amount int64, date time.Time, description string, categoryID *string) error {
	return s.repo.CreateTransaction(ctx, accountID, amount, date, description, categoryID)
}

func (s *Service) DeleteTransaction(ctx context.Context, id string) error {
	return s.repo.DeleteTransaction(ctx, id)
}

func (s *Service) UpdateTransaction(ctx context.Context, id, accountID string, amount int64, date time.Time, description string, categoryID *string) error {
	return s.repo.UpdateTransaction(ctx, id, accountID, amount, date, description, categoryID)
}

// Reports

func (s *Service) GetNetWorthTrend(ctx context.Context, startDate, endDate time.Time) ([]domain.NetWorthPoint, error) {
	return s.repo.GetNetWorthTrend(ctx, startDate, endDate)
}

func (s *Service) GetSpendingByCategory(ctx context.Context, startDate, endDate time.Time) ([]domain.CategorySpend, error) {
	return s.repo.GetSpendingByCategory(ctx, startDate, endDate)
}
