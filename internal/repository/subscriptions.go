package repository

import (
	"context"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

func (r *Repository) CreateSubscription(ctx context.Context, userID, name string, amount int64, cycle string, nextDate time.Time, categoryID *string) error {
	if categoryID != nil {
		if err := r.checkOwnership(ctx, nil, "categories", *categoryID, userID); err != nil {
			return err
		}
	}
	query := `
		INSERT INTO subscriptions (user_id, name, amount, billing_cycle, next_billing_date, category_id)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query, userID, name, amount, cycle, nextDate, categoryID)
	return err
}

func (r *Repository) ListSubscriptions(ctx context.Context, userID string) ([]domain.Subscription, error) {
	r.RolloverSubscriptions(ctx, userID)

	query := `
		SELECT id, user_id, name, amount, billing_cycle, next_billing_date, category_id, created_at, updated_at
		FROM subscriptions
		WHERE user_id = $1
		ORDER BY next_billing_date ASC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		var amt int64
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &amt, &s.BillingCycle, &s.NextBillingDate, &s.CategoryID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Amount = money.Money(amt)
		subs = append(subs, s)
	}
	return subs, nil
}

func (r *Repository) DeleteSubscription(ctx context.Context, userID, id string) error {
	query := `DELETE FROM subscriptions WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateSubscription(ctx context.Context, userID, id string, name string, amount int64, cycle string, nextDate time.Time, categoryID *string) error {
	if categoryID != nil {
		if err := r.checkOwnership(ctx, nil, "categories", *categoryID, userID); err != nil {
			return err
		}
	}
	query := `
		UPDATE subscriptions 
		SET name = $1, amount = $2, billing_cycle = $3, next_billing_date = $4, category_id = $5, updated_at = NOW()
		WHERE id = $6 AND user_id = $7
	`
	tag, err := r.pool.Exec(ctx, query, name, amount, cycle, nextDate, categoryID, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) RolloverSubscriptions(ctx context.Context, userID string) {
	// Advance monthly subscriptions that are in the past to the current month (or future)
	queryMonthly := `
		UPDATE subscriptions 
		SET next_billing_date = next_billing_date + 
			((EXTRACT(year FROM age(CURRENT_DATE, next_billing_date)) * 12) + 
			EXTRACT(month FROM age(CURRENT_DATE, next_billing_date)) + 1) * INTERVAL '1 month'
		WHERE user_id = $1 AND billing_cycle = 'monthly' AND next_billing_date < date_trunc('month', CURRENT_DATE);
	`
	_, _ = r.pool.Exec(ctx, queryMonthly, userID)

	// Advance yearly subscriptions
	queryYearly := `
		UPDATE subscriptions 
		SET next_billing_date = next_billing_date + 
			(EXTRACT(year FROM age(CURRENT_DATE, next_billing_date)) + 1) * INTERVAL '1 year'
		WHERE user_id = $1 AND billing_cycle = 'yearly' AND next_billing_date < date_trunc('month', CURRENT_DATE);
	`
	_, _ = r.pool.Exec(ctx, queryYearly, userID)
}
