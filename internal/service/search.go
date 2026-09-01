package service

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/domain"
)

func (s *Service) GlobalSearch(ctx context.Context, userID string, query string, limit int) (*domain.SearchResult, error) {
	result := &domain.SearchResult{
		Transactions:  make([]domain.Transaction, 0),
		Categories:    make([]domain.Category, 0),
		Accounts:      make([]domain.Account, 0),
		Subscriptions: make([]domain.Subscription, 0),
	}

	if query == "" {
		return result, nil
	}

	txns, err := s.repo.SearchTransactions(ctx, userID, query, limit)
	if err != nil {
		s.logger.Error("search failed", "error", err, "query", query, "user_id", userID)
		return nil, err
	}
	if txns != nil {
		result.Transactions = txns
	}

	return result, nil
}
