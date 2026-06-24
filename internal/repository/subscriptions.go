package repository

import (
	"context"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

func (r *Repository) CreateSubscription(ctx context.Context, name string, amount int64, cycle string, nextDate time.Time, categoryID *string) error {
	query := `
		INSERT INTO subscriptions (name, amount, billing_cycle, next_billing_date, category_id)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, name, amount, cycle, nextDate, categoryID)
	return err
}

func (r *Repository) ListSubscriptions(ctx context.Context) ([]domain.Subscription, error) {
	query := `
		SELECT id, name, amount, billing_cycle, next_billing_date, category_id, created_at, updated_at
		FROM subscriptions
		ORDER BY next_billing_date ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		var amt int64
		if err := rows.Scan(&s.ID, &s.Name, &amt, &s.BillingCycle, &s.NextBillingDate, &s.CategoryID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Amount = money.Money(amt)
		subs = append(subs, s)
	}
	return subs, nil
}

func (r *Repository) DeleteSubscription(ctx context.Context, id string) error {
	query := `DELETE FROM subscriptions WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateSubscription(ctx context.Context, id string, name string, amount int64, cycle string, nextDate time.Time, categoryID *string) error {
	query := `
		UPDATE subscriptions 
		SET name = $1, amount = $2, billing_cycle = $3, next_billing_date = $4, category_id = $5, updated_at = NOW()
		WHERE id = $6
	`
	tag, err := r.pool.Exec(ctx, query, name, amount, cycle, nextDate, categoryID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
