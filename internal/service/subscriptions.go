package service

import (
	"context"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
)

func (s *Service) ListSubscriptions(ctx context.Context, userID string) ([]domain.Subscription, error) {
	return s.repo.ListSubscriptions(ctx, userID)
}

func (s *Service) CreateSubscription(ctx context.Context, userID, name string, amount int64, cycle string, nextDate time.Time, categoryID *string) error {
	return s.repo.CreateSubscription(ctx, userID, name, amount, cycle, nextDate, categoryID)
}

func (s *Service) DeleteSubscription(ctx context.Context, userID, id string) error {
	return s.repo.DeleteSubscription(ctx, userID, id)
}

func (s *Service) UpdateSubscription(ctx context.Context, userID, id string, name string, amount int64, cycle string, nextDate time.Time, categoryID *string) error {
	return s.repo.UpdateSubscription(ctx, userID, id, name, amount, cycle, nextDate, categoryID)
}
