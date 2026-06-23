package service

import (
	"context"
	"time"

	"ntdkhiem/firefly-go/internal/domain"
)

func (s *Service) ListSubscriptions(ctx context.Context) ([]domain.Subscription, error) {
	return s.repo.ListSubscriptions(ctx)
}

func (s *Service) CreateSubscription(ctx context.Context, name string, amount int64, cycle string, nextDate time.Time, categoryID *string) error {
	return s.repo.CreateSubscription(ctx, name, amount, cycle, nextDate, categoryID)
}

func (s *Service) DeleteSubscription(ctx context.Context, id string) error {
	return s.repo.DeleteSubscription(ctx, id)
}
