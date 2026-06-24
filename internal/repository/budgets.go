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
		INSERT INTO budgets (name, category_id, amount, period_type, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(ctx, query, budget.Name, budget.CategoryID, budget.Amount.ToInt64(), budget.PeriodType, budget.StartDate, budget.EndDate).
		Scan(&budget.ID, &budget.CreatedAt, &budget.UpdatedAt)
	return err
}

func (r *Repository) UpdateBudget(ctx context.Context, budget *domain.Budget) error {
	query := `
		UPDATE budgets SET name = $1, category_id = $2, amount = $3, period_type = $4, start_date = $5, end_date = $6, updated_at = NOW()
		WHERE id = $7 RETURNING updated_at
	`
	err := r.pool.QueryRow(ctx, query, budget.Name, budget.CategoryID, budget.Amount.ToInt64(), budget.PeriodType, budget.StartDate, budget.EndDate, budget.ID).
		Scan(&budget.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) DeleteBudget(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM budgets WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) GetBudgetsSummary(ctx context.Context, month time.Time) ([]domain.BudgetSummary, error) {
	// The month passed in is used to filter budgets that overlap with the month
	// And transactions are summed up during that budget's period, but capped to the month if we want monthly?
	// Wait, the requirement says "aggregates spent_total, spent_per_day, left_total, and left_per_day dynamically via SQL".
	// Let's look at the query:
	// A budget has start_date and end_date. For the summary, we calculate spent_total by joining transactions within start_date and end_date.
	
	query := `
		WITH budget_spent AS (
			SELECT b.id as budget_id, COALESCE(SUM(ABS(t.amount)), 0) as spent_total
			FROM budgets b
			LEFT JOIN transactions t ON t.category_id = b.category_id 
				AND t.date >= b.start_date 
				AND t.date <= b.end_date
				AND t.deleted_at IS NULL
			WHERE b.start_date <= $1::date + INTERVAL '1 month - 1 day'
			  AND b.end_date >= $1::date
			GROUP BY b.id
		)
		SELECT b.id, b.name, b.category_id, b.amount as amount_cents, b.period_type, b.start_date, b.end_date, b.created_at, b.updated_at,
		       s.spent_total,
			   (b.end_date - b.start_date) + 1 as total_days,
			   (LEAST(CURRENT_DATE, b.end_date) - b.start_date) + 1 as elapsed_days
		FROM budgets b
		JOIN budget_spent s ON b.id = s.budget_id
		ORDER BY b.start_date DESC
	`
	rows, err := r.pool.Query(ctx, query, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.BudgetSummary
	for rows.Next() {
		var s domain.BudgetSummary
		var amountCents int64
		var spentTotal int64
		var totalDays int
		var elapsedDays int
		
		if err := rows.Scan(&s.ID, &s.Name, &s.CategoryID, &amountCents, &s.PeriodType, &s.StartDate, &s.EndDate, &s.CreatedAt, &s.UpdatedAt, &spentTotal, &totalDays, &elapsedDays); err != nil {
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
