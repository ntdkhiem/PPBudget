package repository

import (
	"context"
	"errors"
	"math"
	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// isDuplicateBudget reports whether err is budgets_user_category_month_key
// rejecting a second budget for a category in the same month.
func isDuplicateBudget(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "budgets_user_category_month_key"
}

func (r *Repository) CreateBudget(ctx context.Context, budget *domain.Budget) error {
	if budget.CategoryID != "" {
		if err := r.checkOwnership(ctx, nil, "categories", budget.CategoryID, budget.UserID); err != nil {
			return err
		}
	}
	query := `
		INSERT INTO budgets (name, category_id, amount, period_type, start_date, end_date, bucket, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(ctx, query, budget.Name, budget.CategoryID, budget.Amount.ToInt64(), budget.PeriodType, budget.StartDate, budget.EndDate, budget.Bucket, budget.UserID).
		Scan(&budget.ID, &budget.CreatedAt, &budget.UpdatedAt)
	if isDuplicateBudget(err) {
		return apperrors.ErrConflict
	}
	return err
}

func (r *Repository) UpdateBudget(ctx context.Context, budget *domain.Budget) error {
	if budget.CategoryID != "" {
		if err := r.checkOwnership(ctx, nil, "categories", budget.CategoryID, budget.UserID); err != nil {
			return err
		}
	}

	if budget.PeriodType == "monthly" {
		query := `
			UPDATE budgets SET name = $1, amount = $2, bucket = $3, period_type = $4, updated_at = NOW()
			WHERE user_id = $5 AND category_id = $6 AND start_date >= $7
		`
		tag, err := r.pool.Exec(ctx, query, budget.Name, budget.Amount.ToInt64(), budget.Bucket, budget.PeriodType, budget.UserID, budget.CategoryID, budget.StartDate)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperrors.ErrNotFound
		}
		// Also update the specific budget ID just in case it doesn't match the category/date condition (e.g. if start_date was changed, which shouldn't happen, but let's be safe)
		r.pool.Exec(ctx, `UPDATE budgets SET name = $1, category_id = $2, amount = $3, period_type = $4, start_date = $5, end_date = $6, bucket = $7, updated_at = NOW() WHERE id = $8 AND user_id = $9`,
			budget.Name, budget.CategoryID, budget.Amount.ToInt64(), budget.PeriodType, budget.StartDate, budget.EndDate, budget.Bucket, budget.ID, budget.UserID)
	} else {
		query := `
			UPDATE budgets SET name = $1, category_id = $2, amount = $3, period_type = $4, start_date = $5, end_date = $6, bucket = $7, updated_at = NOW()
			WHERE id = $8 AND user_id = $9 RETURNING updated_at
		`
		err := r.pool.QueryRow(ctx, query, budget.Name, budget.CategoryID, budget.Amount.ToInt64(), budget.PeriodType, budget.StartDate, budget.EndDate, budget.Bucket, budget.ID, budget.UserID).
			Scan(&budget.UpdatedAt)
		if err != nil {
			if err == pgx.ErrNoRows {
				return apperrors.ErrNotFound
			}
			if isDuplicateBudget(err) {
				return apperrors.ErrConflict
			}
			return err
		}
	}

	return nil
}

func (r *Repository) DeleteBudget(ctx context.Context, userID, id string, allMonths bool) error {
	if allMonths {
		// Delete all budgets with the same category_id
		query := `
			DELETE FROM budgets 
			WHERE user_id = $1 
			  AND category_id = (SELECT category_id FROM budgets WHERE id = $2 AND user_id = $1)
			  AND start_date >= (SELECT start_date FROM budgets WHERE id = $2 AND user_id = $1)
		`
		tag, err := r.pool.Exec(ctx, query, userID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperrors.ErrNotFound
		}
		return nil
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// effectiveTransactionsCTE is every live transaction of user $1 with the
// month it counts toward and its link-adjusted amount. Budget spending and
// unbudgeted spending both read it, so the two always add up the same way.
const effectiveTransactionsCTE = `
	effective_transactions AS (
		SELECT
			t.id,
			t.category_id,
			-- The month it counts toward, not the day it posted: rent paid
			-- early belongs to the month it pays for.
			t.effective_date AS date,
			t.amount
			- COALESCE((SELECT SUM(amount) FROM transaction_links WHERE source_transaction_id = t.id), 0)
			+ COALESCE((SELECT SUM(amount) FROM transaction_links WHERE target_transaction_id = t.id), 0) as eff_amount
		FROM transactions t
		WHERE t.user_id = $1 AND t.deleted_at IS NULL
	)`

// budgetMonthBounds returns the first and last day of the month containing t.
func budgetMonthBounds(t time.Time) (time.Time, time.Time) {
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, -1)
}

// resolveBudgetMonth returns the month whose budget rows apply to the month
// starting targetStart. A future month has no rows of its own and is projected
// from the latest month up to today's; the current or a past month uses its
// own rows, copying last month's forward the first time it is viewed.
func (r *Repository) resolveBudgetMonth(ctx context.Context, userID string, targetStart, today time.Time) time.Time {
	currentMonthStart, _ := budgetMonthBounds(today)

	if targetStart.After(currentMonthStart) {
		var latestStart *time.Time
		err := r.pool.QueryRow(ctx, "SELECT MAX(start_date) FROM budgets WHERE user_id = $1 AND start_date <= $2", userID, currentMonthStart).Scan(&latestStart)
		if err == nil && latestStart != nil {
			return *latestStart
		}
		return targetStart
	}

	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM budgets WHERE user_id = $1 AND start_date = $2", userID, targetStart).Scan(&count)
	if err == nil && count == 0 {
		_ = r.rolloverBudgets(ctx, userID, targetStart)
	}
	return targetStart
}

// budgetDayRates spreads a month's spending over the days gone and what is
// left over the days remaining. Both counts include today, which can still be
// spent; a finished month has no days remaining and a future one none gone.
func budgetDayRates(monthStart, monthEnd, today time.Time, spent, left int64) (spentPerDay, leftPerDay int64) {
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	totalDays := int64(monthEnd.Sub(monthStart).Hours()/24) + 1
	clamp := func(n int64) int64 { return max(0, min(n, totalDays)) }

	elapsed := clamp(int64(day.Sub(monthStart).Hours()/24) + 1)
	remaining := clamp(int64(monthEnd.Sub(day).Hours()/24) + 1)

	if elapsed > 0 {
		spentPerDay = roundDiv(spent, elapsed)
	}
	if remaining > 0 && left > 0 {
		leftPerDay = roundDiv(left, remaining)
	}
	return spentPerDay, leftPerDay
}

// roundDiv divides to the nearest cent, half away from zero: $620 over 30
// days is $20.67 a day, not the $20.66 integer division truncates to.
func roundDiv(a, b int64) int64 {
	return int64(math.Round(float64(a) / float64(b)))
}

// GetBudgetsSummary returns the budgets for targetMonth with what has been
// spent against each. today is the viewer's calendar date, which decides
// which months are past or future and the per-day figures.
func (r *Repository) GetBudgetsSummary(ctx context.Context, userID string, targetMonth, today time.Time) ([]domain.BudgetSummary, error) {
	targetMonthStart, targetMonthEnd := budgetMonthBounds(targetMonth)
	budgetTemplateMonth := r.resolveBudgetMonth(ctx, userID, targetMonthStart, today)

	query := `
		WITH ` + effectiveTransactionsCTE + `,
		budget_spent AS (
			-- Outflows less inflows: a refund in the category gives budget back.
			SELECT b.id as budget_id, COALESCE(-SUM(t.eff_amount), 0) as spent_total
			FROM budgets b
			LEFT JOIN effective_transactions t ON t.category_id = b.category_id
				AND t.date >= $3::date
				AND t.date <= $4::date
			WHERE b.user_id = $1
			  AND b.start_date = $2::date
			GROUP BY b.id
		)
		SELECT b.id, b.user_id, b.name, b.category_id, b.amount as amount_cents, b.period_type, b.start_date, b.end_date, b.bucket, b.created_at, b.updated_at,
		       s.spent_total
		FROM budgets b
		JOIN budget_spent s ON b.id = s.budget_id
		WHERE b.user_id = $1
		  AND b.start_date = $2::date
		  AND ($2::date = $3::date OR b.period_type != 'one_time')
		ORDER BY b.start_date DESC
	`
	rows, err := r.pool.Query(ctx, query, userID, budgetTemplateMonth, targetMonthStart, targetMonthEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.BudgetSummary
	for rows.Next() {
		var s domain.BudgetSummary
		var amountCents int64
		var spentTotal int64

		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.CategoryID, &amountCents, &s.PeriodType, &s.StartDate, &s.EndDate, &s.Bucket, &s.CreatedAt, &s.UpdatedAt, &spentTotal); err != nil {
			return nil, err
		}

		s.Amount = money.Money(amountCents)
		s.SpentTotal = money.Money(spentTotal)
		s.LeftTotal = money.Money(amountCents - spentTotal)
		spentPerDay, leftPerDay := budgetDayRates(targetMonthStart, targetMonthEnd, today, spentTotal, amountCents-spentTotal)
		s.SpentPerDay = money.Money(spentPerDay)
		s.LeftPerDay = money.Money(leftPerDay)

		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// GetUnbudgetedSpending returns what was spent in targetMonth outside any
// budget: expense categories with no budget that month, plus uncategorized
// outflows. Income and transfer categories are not spending and are left out.
func (r *Repository) GetUnbudgetedSpending(ctx context.Context, userID string, targetMonth, today time.Time) ([]domain.CategorySpend, error) {
	targetMonthStart, targetMonthEnd := budgetMonthBounds(targetMonth)
	budgetTemplateMonth := r.resolveBudgetMonth(ctx, userID, targetMonthStart, today)

	// A category nets refunds against spending, as a budget does. Uncategorized
	// counts outflows only: its inflows are mostly paychecks, which would
	// otherwise cancel out real spending.
	query := `
		WITH ` + effectiveTransactionsCTE + `
		SELECT t.category_id, COALESCE(c.name, 'Uncategorized'),
		       -SUM(CASE WHEN t.category_id IS NULL THEN LEAST(t.eff_amount, 0) ELSE t.eff_amount END) AS spent
		FROM effective_transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.date >= $3::date AND t.date <= $4::date
		  AND (t.category_id IS NULL OR c.type = 'expense')
		  AND NOT EXISTS (
			SELECT 1 FROM budgets b
			WHERE b.user_id = $1 AND b.category_id = t.category_id
			  AND b.start_date = $2::date
			  AND ($2::date = $3::date OR b.period_type != 'one_time')
		  )
		GROUP BY t.category_id, c.name
		HAVING -SUM(CASE WHEN t.category_id IS NULL THEN LEAST(t.eff_amount, 0) ELSE t.eff_amount END) > 0
		ORDER BY spent DESC, 2
	`
	rows, err := r.pool.Query(ctx, query, userID, budgetTemplateMonth, targetMonthStart, targetMonthEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spending := []domain.CategorySpend{}
	for rows.Next() {
		var s domain.CategorySpend
		var total int64
		if err := rows.Scan(&s.CategoryID, &s.Name, &total); err != nil {
			return nil, err
		}
		s.TotalSpent = money.Money(total)
		spending = append(spending, s)
	}
	return spending, rows.Err()
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

	// Two requests for a new month can both find it empty and both copy; the
	// unique key lets the second copy skip instead of duplicating every row.
	queryCopy := `
		INSERT INTO budgets (name, category_id, amount, period_type, start_date, end_date, bucket, user_id, created_at, updated_at)
		SELECT name, category_id, amount, period_type, $1, $2, bucket, $3, NOW(), NOW()
		FROM budgets
		WHERE user_id = $3 AND start_date = $4 AND period_type != 'one_time'
		ON CONFLICT (user_id, category_id, start_date) DO NOTHING
	`
	_, err = r.pool.Exec(ctx, queryCopy, targetStart, targetEnd, userID, *latestStart)
	return err
}

func (r *Repository) ListAllBudgets(ctx context.Context, userID string) ([]domain.Budget, error) {
	query := `SELECT id, name, category_id, amount, period_type, start_date, end_date, bucket FROM budgets WHERE user_id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var budgets []domain.Budget
	for rows.Next() {
		var b domain.Budget
		if err := rows.Scan(&b.ID, &b.Name, &b.CategoryID, &b.Amount, &b.PeriodType, &b.StartDate, &b.EndDate, &b.Bucket); err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, nil
}
