package service

import (
	"context"
	"fmt"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// RecordManualBalance anchors the account's balance at asOf (nil = today, UTC).
func (s *Service) RecordManualBalance(ctx context.Context, userID, accountID string, balance int64, asOf *time.Time) error {
	now := time.Now().UTC()

	var date time.Time
	if asOf == nil {
		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else {
		d := asOf.UTC()
		date = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	}

	// Clients send their local calendar date, which can be one day ahead of UTC
	// (e.g. morning in UTC+7), so allow up to tomorrow in UTC.
	latestAllowed := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	if date.After(latestAllowed) {
		return fmt.Errorf("%w: as_of_date cannot be in the future", apperrors.ErrInvalidInput)
	}

	reportedAt := now
	return s.repo.UpsertBalanceSnapshot(ctx, nil, userID, accountID, date, money.Money(balance), nil, domain.BalanceSourceManual, &reportedAt)
}

func (s *Service) ListBalanceSnapshots(ctx context.Context, userID, accountID string) ([]domain.BalanceSnapshot, error) {
	return s.repo.ListBalanceSnapshots(ctx, userID, accountID)
}

func (s *Service) DeleteBalanceSnapshot(ctx context.Context, userID, accountID, snapshotID string) error {
	return s.repo.DeleteBalanceSnapshot(ctx, userID, accountID, snapshotID)
}
