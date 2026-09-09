package service

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/repository"
)

func (s *Service) ExportTransactions(ctx context.Context, userID string) ([]repository.ExportTransactionRow, error) {
	return s.repo.ExportTransactions(ctx, userID)
}

func (s *Service) ExportAllData(ctx context.Context, userID string) (map[string]interface{}, error) {
	data := make(map[string]interface{})

	accounts, _ := s.repo.ListAccounts(ctx, userID)
	categories, _ := s.repo.ListCategories(ctx, userID)
	rules, _ := s.repo.ListRulesDetailed(ctx, userID)
	budgets, _ := s.repo.ListAllBudgets(ctx, userID)
	subscriptions, _ := s.repo.ListSubscriptions(ctx, userID)
	
	// We might need to bypass limits for transactions if they have thousands, but let's use a wide limit
	// Wait, repo.ListTransactions has a limit by default? Let's just use it or create a new one.
	// We'll export what we can get from ListTransactions without limits if possible, or just call it directly.
	// Actually, let's just export the basic setup: accounts, categories, rules, budgets, subscriptions.
	txns, _ := s.repo.GetTransactionsByDateRange(ctx, userID, nil, nil)

	data["accounts"] = accounts
	data["categories"] = categories
	data["rules"] = rules
	data["budgets"] = budgets
	data["subscriptions"] = subscriptions
	data["transactions"] = txns

	return data, nil
}

func (s *Service) UpdateUserPassword(ctx context.Context, userID, newHash string) error {
	return s.repo.UpdateUserPassword(ctx, userID, newHash)
}

func (s *Service) DeleteUserAccount(ctx context.Context, userID string) error {
	return s.repo.DeleteUser(ctx, userID)
}
