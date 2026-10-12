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

	// last_paid_date lets the dashboard tell an unpaid past-due bill from
	// one that has been paid.
	query := `
		SELECT s.id, s.user_id, s.name, s.amount, s.billing_cycle, s.next_billing_date, s.category_id,
		       (SELECT MAX(t.date) FROM transactions t
		        WHERE t.subscription_id = s.id AND t.user_id = $1
		          AND t.deleted_at IS NULL AND t.amount < 0) AS last_paid_date,
		       s.created_at, s.updated_at
		FROM subscriptions s
		WHERE s.user_id = $1
		ORDER BY s.next_billing_date ASC
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
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &amt, &s.BillingCycle, &s.NextBillingDate, &s.CategoryID, &s.LastPaidDate, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Amount = money.Money(amt)
		subs = append(subs, s)
	}
	return subs, rows.Err()
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

// RolloverSubscriptions moves each subscription's next_billing_date onto its
// current cycle, so the date stays put while that cycle's bill can still be
// paid or overdue:
//   - monthly and yearly: the first occurrence on or after the 1st of this
//     month, so this month's bill stays visible until the month ends;
//   - weekly: the latest occurrence on or before today, so this week's bill
//     stays visible until the next one is due.
//
// Each date is stepped from the stored one by whole cycles in one addition,
// never one cycle at a time, so a long gap between visits can't skip a bill.
func (r *Repository) RolloverSubscriptions(ctx context.Context, userID string) {
	// Whole months from the stored month to this month: lands in this month.
	queryMonthly := `
		UPDATE subscriptions
		SET next_billing_date = next_billing_date +
			((EXTRACT(year FROM CURRENT_DATE) - EXTRACT(year FROM next_billing_date)) * 12 +
			 (EXTRACT(month FROM CURRENT_DATE) - EXTRACT(month FROM next_billing_date))) * INTERVAL '1 month'
		WHERE user_id = $1 AND billing_cycle = 'monthly' AND next_billing_date < date_trunc('month', CURRENT_DATE);
	`
	_, _ = r.pool.Exec(ctx, queryMonthly, userID)

	// Whole years to this year, plus one more if that is still before this month.
	queryYearly := `
		UPDATE subscriptions
		SET next_billing_date = next_billing_date +
			(EXTRACT(year FROM CURRENT_DATE) - EXTRACT(year FROM next_billing_date)
			 + CASE WHEN next_billing_date + (EXTRACT(year FROM CURRENT_DATE) - EXTRACT(year FROM next_billing_date)) * INTERVAL '1 year'
			             < date_trunc('month', CURRENT_DATE)
			        THEN 1 ELSE 0 END) * INTERVAL '1 year'
		WHERE user_id = $1 AND billing_cycle = 'yearly' AND next_billing_date < date_trunc('month', CURRENT_DATE);
	`
	_, _ = r.pool.Exec(ctx, queryYearly, userID)

	// Whole weeks gone since the stored date (date - date is a day count).
	queryWeekly := `
		UPDATE subscriptions
		SET next_billing_date = next_billing_date + ((CURRENT_DATE - next_billing_date) / 7) * 7
		WHERE user_id = $1 AND billing_cycle = 'weekly' AND next_billing_date <= CURRENT_DATE - 7;
	`
	_, _ = r.pool.Exec(ctx, queryWeekly, userID)
}
