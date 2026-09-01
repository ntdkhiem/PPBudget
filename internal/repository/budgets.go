package repository

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateBudget(ctx context.Context, budget *domain.Budget) error {
	query := `
		INSERT INTO budgets (name, category_id, amount, period_type, start_date, end_date, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(ctx, query, budget.Name, budget.CategoryID, budget.Amount.ToInt64(), budget.PeriodType, budget.StartDate, budget.EndDate, budget.UserID).
		Scan(&budget.ID, &budget.CreatedAt, &budget.UpdatedAt)
	return err
}

func (r *Repository) UpdateBudget(ctx context.Context, budget *domain.Budget) error {
	query := `
		UPDATE budgets SET name = $1, category_id = $2, amount = $3, period_type = $4, start_date = $5, end_date = $6, updated_at = NOW()
		WHERE id = $7 AND user_id = $8 RETURNING updated_at
	`
	err := r.pool.QueryRow(ctx, query, budget.Name, budget.CategoryID, budget.Amount.ToInt64(), budget.PeriodType, budget.StartDate, budget.EndDate, budget.ID, budget.UserID).
		Scan(&budget.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) DeleteBudget(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) GetBudgetsSummary(ctx context.Context, userID string, month time.Time) ([]domain.BudgetSummary, error) {
	// Auto-rollover budgets if none exist for this month
	var count int
	targetStart := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM budgets WHERE user_id = $1 AND start_date = $2", userID, targetStart).Scan(&count)
	if err == nil && count == 0 {
		_ = r.rolloverBudgets(ctx, userID, month)
	}

	query := `
		WITH effective_transactions AS (
			SELECT 
				t.id,
				t.category_id,
				t.date,
				t.amount 
				- COALESCE((SELECT SUM(amount) FROM transaction_links WHERE source_transaction_id = t.id), 0)
				+ COALESCE((SELECT SUM(amount) FROM transaction_links WHERE target_transaction_id = t.id), 0) as eff_amount
			FROM transactions t
			WHERE t.user_id = $1 AND t.deleted_at IS NULL
		),
		budget_spent AS (
			SELECT b.id as budget_id, COALESCE(SUM(ABS(t.eff_amount)), 0) as spent_total
			FROM budgets b
			LEFT JOIN effective_transactions t ON t.category_id = b.category_id 
				AND t.date >= b.start_date 
				AND t.date <= b.end_date
			WHERE b.user_id = $1
			  AND b.start_date <= $2::date + INTERVAL '1 month - 1 day'
			  AND b.end_date >= $2::date
			GROUP BY b.id
		)
		SELECT b.id, b.user_id, b.name, b.category_id, b.amount as amount_cents, b.period_type, b.start_date, b.end_date, b.created_at, b.updated_at,
		       s.spent_total,
			   (b.end_date - b.start_date) + 1 as total_days,
			   (LEAST(CURRENT_DATE, b.end_date) - b.start_date) + 1 as elapsed_days
		FROM budgets b
		JOIN budget_spent s ON b.id = s.budget_id
		WHERE b.user_id = $1
		ORDER BY b.start_date DESC
	`
	rows, err := r.pool.Query(ctx, query, userID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.BudgetSummary
	for rows.Next() {
		var s domain.BudgetSummary
		var amountCents int64
		var spentTotal int64
		var totalDays int32
		var elapsedDays int32

		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.CategoryID, &amountCents, &s.PeriodType, &s.StartDate, &s.EndDate, &s.CreatedAt, &s.UpdatedAt, &spentTotal, &totalDays, &elapsedDays); err != nil {
			return nil, err
		}

		s.Amount = money.Money(amountCents)
		s.SpentTotal = money.Money(spentTotal)
		s.LeftTotal = money.Money(amountCents - spentTotal)

		if elapsedDays > 0 {
			s.SpentPerDay = money.Money(spentTotal / int64(elapsedDays))
		} else {
			s.SpentPerDay = money.Money(0)
		}

		remainingDays := totalDays - elapsedDays
		if remainingDays > 0 {
			s.LeftPerDay = money.Money(s.LeftTotal.ToInt64() / int64(remainingDays))
		} else {
			s.LeftPerDay = money.Money(0)
		}

		summaries = append(summaries, s)
	}
	return summaries, nil
}

func (r *Repository) rolloverBudgets(ctx context.Context, userID string, targetMonth time.Time) error {
	targetStart := time.Date(targetMonth.Year(), targetMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	targetEnd := targetStart.AddDate(0, 1, -1)

	queryLatestMonth := `
		SELECT MAX(start_date)
		FROM budgets
		WHERE user_id = $1 AND start_date < $2
	`
	var latestStart *time.Time
	err := r.pool.QueryRow(ctx, queryLatestMonth, userID, targetStart).Scan(&latestStart)
	if err != nil || latestStart == nil {
		return nil
	}

	queryCopy := `
		INSERT INTO budgets (name, category_id, amount, period_type, start_date, end_date, user_id, created_at, updated_at)
		SELECT name, category_id, amount, period_type, $1, $2, $3, NOW(), NOW()
		FROM budgets
		WHERE user_id = $3 AND start_date = $4
	`
	_, err = r.pool.Exec(ctx, queryCopy, targetStart, targetEnd, userID, *latestStart)
	return err
}
