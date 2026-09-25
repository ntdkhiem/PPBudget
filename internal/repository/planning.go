package repository

import (
	"context"
	"fmt"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// GetPlanningBaseline returns the trailing-N-month baseline the financial
// planning page projects from: per-month income, outflow split across budget
// buckets, and month-end net worth, plus current per-account balances.
//
// Cashflow uses the same link-adjusted, transfer-excluded definition as
// GetSpendingByCategory and GetReportsSummary so the numbers reconcile with the
// dashboard. Net worth uses the same account_balance_at shape as
// GetNetWorthTrend.
func (r *Repository) GetPlanningBaseline(ctx context.Context, userID string, months int) (*domain.PlanningBaseline, error) {
	if months < 1 {
		months = 1
	}
	if months > 24 {
		months = 24
	}

	baseline := &domain.PlanningBaseline{
		Months:   []domain.PlanningMonth{},
		Accounts: []domain.PlanningAccount{},
	}

	byMonth, order, err := r.planningCashflow(ctx, userID, months)
	if err != nil {
		return nil, fmt.Errorf("planning cashflow: %w", err)
	}
	if err := r.planningNetWorth(ctx, userID, months, byMonth); err != nil {
		return nil, fmt.Errorf("planning net worth: %w", err)
	}
	for _, key := range order {
		baseline.Months = append(baseline.Months, *byMonth[key])
	}

	if err := r.planningAccounts(ctx, userID, baseline); err != nil {
		return nil, fmt.Errorf("planning accounts: %w", err)
	}

	return baseline, nil
}

// planningCashflow aggregates income and outflow per month, attributing each
// outflow to a budget bucket.
//
// Bucket attribution deliberately falls back to the most recent budget row at
// or before the transaction's month rather than requiring an exact-month row:
// rows for past months are only materialized lazily by GetBudgetsSummary's
// rollover, so an exact join would report almost everything as unbucketed.
// one_time budgets are excluded from that carry-forward since they describe a
// single month only.
func (r *Repository) planningCashflow(ctx context.Context, userID string, months int) (map[string]*domain.PlanningMonth, []string, error) {
	query := `
		WITH months AS (
			SELECT date_trunc('month', d)::date AS month_start,
			       (date_trunc('month', d) + INTERVAL '1 month - 1 day')::date AS month_end
			FROM generate_series(
				date_trunc('month', CURRENT_DATE) - (($2::int - 1) * INTERVAL '1 month'),
				date_trunc('month', CURRENT_DATE),
				'1 month'::interval
			) d
		),
		effective AS (
			SELECT
				date_trunc('month', t.date)::date AS month_start,
				t.category_id,
				(t.amount
				 - COALESCE((SELECT SUM(l.amount) FROM transaction_links l WHERE l.source_transaction_id = t.id), 0)
				 + COALESCE((SELECT SUM(l.amount) FROM transaction_links l WHERE l.target_transaction_id = t.id), 0)
				) AS eff_amount
			FROM transactions t
			LEFT JOIN categories c ON t.category_id = c.id
			WHERE t.user_id = $1
			  AND t.deleted_at IS NULL
			  AND (c.type IS NULL OR c.type <> 'transfer')
			  AND t.date >= (SELECT MIN(month_start) FROM months)
			  AND t.date <= (SELECT MAX(month_end) FROM months)
		),
		bucketed AS (
			SELECT
				e.month_start,
				e.eff_amount,
				(
					SELECT b.bucket
					FROM budgets b
					WHERE b.user_id = $1
					  AND b.category_id = e.category_id
					  AND b.start_date <= e.month_start
					  AND (b.period_type <> 'one_time' OR b.start_date = e.month_start)
					ORDER BY b.start_date DESC, b.period_type ASC
					LIMIT 1
				) AS bucket
			FROM effective e
		)
		SELECT
			m.month_start,
			COALESCE(SUM(x.eff_amount) FILTER (WHERE x.eff_amount > 0), 0)::bigint AS income,
			COALESCE(SUM(-x.eff_amount) FILTER (WHERE x.eff_amount < 0), 0)::bigint AS outflow,
			COALESCE(SUM(-x.eff_amount) FILTER (WHERE x.eff_amount < 0 AND x.bucket = 'needs'), 0)::bigint AS needs,
			COALESCE(SUM(-x.eff_amount) FILTER (WHERE x.eff_amount < 0 AND x.bucket = 'wants'), 0)::bigint AS wants,
			COALESCE(SUM(-x.eff_amount) FILTER (WHERE x.eff_amount < 0 AND x.bucket = 'savings'), 0)::bigint AS savings,
			COALESCE(SUM(-x.eff_amount) FILTER (WHERE x.eff_amount < 0 AND x.bucket IS NULL), 0)::bigint AS unbucketed
		FROM months m
		LEFT JOIN bucketed x ON x.month_start = m.month_start
		GROUP BY m.month_start
		ORDER BY m.month_start ASC
	`

	rows, err := r.pool.Query(ctx, query, userID, months)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	byMonth := make(map[string]*domain.PlanningMonth)
	var order []string

	for rows.Next() {
		var m domain.PlanningMonth
		var income, outflow, needs, wants, savings, unbucketed int64
		if err := rows.Scan(&m.Month, &income, &outflow, &needs, &wants, &savings, &unbucketed); err != nil {
			return nil, nil, err
		}
		m.Income = money.Money(income)
		m.Outflow = money.Money(outflow)
		m.Needs = money.Money(needs)
		m.Wants = money.Money(wants)
		m.Savings = money.Money(savings)
		m.Unbucketed = money.Money(unbucketed)

		key := monthKey(m.Month)
		entry := m
		byMonth[key] = &entry
		order = append(order, key)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return byMonth, order, nil
}

// planningNetWorth fills month-end net worth into an existing month map.
func (r *Repository) planningNetWorth(ctx context.Context, userID string, months int, byMonth map[string]*domain.PlanningMonth) error {
	query := `
		WITH dates AS (
			SELECT date_trunc('month', d)::date AS month_start,
			       (date_trunc('month', d) + INTERVAL '1 month - 1 day')::date AS end_of_month
			FROM generate_series(
				date_trunc('month', CURRENT_DATE) - (($2::int - 1) * INTERVAL '1 month'),
				date_trunc('month', CURRENT_DATE),
				'1 month'::interval
			) d
		),
		balances AS (
			SELECT
				d.month_start,
				account_balance_at(a.id, LEAST(d.end_of_month, CURRENT_DATE)) AS balance
			FROM dates d
			CROSS JOIN accounts a
			WHERE a.user_id = $1 AND a.type IN ('asset', 'liability')
		)
		SELECT month_start, COALESCE(SUM(balance), 0)::bigint AS net_worth
		FROM balances
		GROUP BY month_start
		ORDER BY month_start ASC
	`

	rows, err := r.pool.Query(ctx, query, userID, months)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var month time.Time
		var netWorth int64
		if err := rows.Scan(&month, &netWorth); err != nil {
			return err
		}
		if entry, ok := byMonth[monthKey(month)]; ok {
			entry.NetWorth = money.Money(netWorth)
		}
	}
	return rows.Err()
}

// planningAccounts loads current per-account balances plus the asset and
// liability rollups the emergency-fund math needs.
func (r *Repository) planningAccounts(ctx context.Context, userID string, baseline *domain.PlanningBaseline) error {
	query := `
		SELECT a.id, a.name, a.type, a.role,
		       COALESCE(account_balance_at(a.id, 'infinity'::date), 0)::bigint AS balance
		FROM accounts a
		WHERE a.user_id = $1 AND a.type IN ('asset', 'liability')
		ORDER BY a.type ASC, a.name ASC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var acct domain.PlanningAccount
		var balance int64
		if err := rows.Scan(&acct.ID, &acct.Name, &acct.Type, &acct.Role, &balance); err != nil {
			return err
		}
		acct.Balance = money.Money(balance)
		baseline.Accounts = append(baseline.Accounts, acct)

		switch acct.Type {
		case "asset":
			baseline.LiquidAssets += acct.Balance
		case "liability":
			baseline.TotalLiabilities += acct.Balance
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Liabilities are stored negative, so net worth is a plain sum.
	baseline.NetWorth = baseline.LiquidAssets + baseline.TotalLiabilities
	return nil
}

func monthKey(t time.Time) string {
	return t.Format("2006-01")
}
