package service

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/repository"
)

func (s *Service) GetUserSetting(ctx context.Context, userID, key string) (string, error) {
	return s.repo.GetUserSetting(ctx, userID, key)
}

func (s *Service) SetUserSetting(ctx context.Context, userID, key, value string) error {
	return s.repo.SetUserSetting(ctx, userID, key, value)
}

func (s *Service) ExportTransactions(ctx context.Context, userID string) ([]repository.ExportTransactionRow, error) {
	return s.repo.ExportTransactions(ctx, userID)
}

func (s *Service) ExportAllData(ctx context.Context, userID string, types []string) (map[string]interface{}, error) {
	data := make(map[string]interface{})

	shouldInclude := func(t string) bool {
		if len(types) == 0 {
			return true
		}
		for _, v := range types {
			if v == t {
				return true
			}
		}
		return false
	}

	if shouldInclude("accounts") {
		accounts, _ := s.repo.ListAccounts(ctx, userID)
		data["accounts"] = accounts
	}
	if shouldInclude("categories") {
		categories, _ := s.repo.ListCategories(ctx, userID)
		data["categories"] = categories
	}
	if shouldInclude("rules") {
		rules, _ := s.repo.ListRulesDetailed(ctx, userID)
		data["rules"] = rules
	}
	if shouldInclude("budgets") {
		budgets, _ := s.repo.ListAllBudgets(ctx, userID)
		data["budgets"] = budgets
	}
	if shouldInclude("subscriptions") {
		subscriptions, _ := s.repo.ListSubscriptions(ctx, userID)
		data["subscriptions"] = subscriptions
	}
	if shouldInclude("transactions") {
		txns, _ := s.repo.GetTransactionsByDateRange(ctx, userID, nil, nil)
		data["transactions"] = txns
	}

	return data, nil
}

func (s *Service) UpdateUserPassword(ctx context.Context, userID, newHash string) error {
	return s.repo.UpdateUserPassword(ctx, userID, newHash)
}

func (s *Service) DeleteUserAccount(ctx context.Context, userID string) error {
	return s.repo.DeleteUser(ctx, userID)
}
