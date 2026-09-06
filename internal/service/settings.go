package service

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/repository"
)

func (s *Service) ExportTransactions(ctx context.Context, userID string) ([]repository.ExportTransactionRow, error) {
	return s.repo.ExportTransactions(ctx, userID)
}

func (s *Service) UpdateUserPassword(ctx context.Context, userID, newHash string) error {
	return s.repo.UpdateUserPassword(ctx, userID, newHash)
}

func (s *Service) DeleteUserAccount(ctx context.Context, userID string) error {
	return s.repo.DeleteUser(ctx, userID)
}
