package repository

// Integration tests for the planning baseline query. Gated on TEST_DATABASE_URL;
// see balances_integration_test.go for the helpers and the throwaway-database warning.

import (
	"context"
	"testing"
	"time"
)

// monthsAgo returns the first day of the month n months before today, in the
// yyyy-MM-dd form the insert helpers take. The planning query is anchored on
// CURRENT_DATE, so fixtures must be positioned relative to today rather than
// pinned to fixed calendar dates.
func monthsAgo(n int) string {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -n, 0)
	return start.Format("2006-01-02")
}

// dayIn returns a date inside the month n months back, offset by day-1 days.
func dayIn(n, day int) string {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -n, 0)
	return start.AddDate(0, 0, day-1).Format("2006-01-02")
}

func TestPlanningBaselineIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "planning")
	accID, err := repo.CreateAccount(ctx, userID, "Planning Checking", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	cardID, err := repo.CreateAccount(ctx, userID, "Planning Card", "liability", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount (liability): %v", err)
	}

	newCategory := func(name, catType string) string {
		var id string
		if err := pool.QueryRow(ctx,
			`INSERT INTO categories (user_id, name, type) VALUES ($1, $2, $3) RETURNING id`,
			userID, name, catType,
		).Scan(&id); err != nil {
			t.Fatalf("insert category %s: %v", name, err)
		}
		return id
	}
	setCategory := func(txnID, catID string) {
		if _, err := pool.Exec(ctx, `UPDATE transactions SET category_id = $1 WHERE id = $2`, catID, txnID); err != nil {
			t.Fatalf("set category: %v", err)
		}
	}
	newBudget := func(name, catID, bucket, startDate string, amount int64) {
		start, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			t.Fatalf("parse budget start: %v", err)
		}
		end := start.AddDate(0, 1, -1)
		if _, err := pool.Exec(ctx,
			`INSERT INTO budgets (user_id, name, category_id, amount, period_type, start_date, end_date, bucket)
			 VALUES ($1, $2, $3, $4, 'monthly', $5::date, $6::date, $7)`,
			userID, name, catID, amount, startDate, end.Format("2006-01-02"), bucket,
		); err != nil {
			t.Fatalf("insert budget %s: %v", name, err)
		}
	}

	rent := newCategory("Rent", "expense")
	dining := newCategory("Dining", "expense")
	brokerage := newCategory("Brokerage", "expense")
	transfer := newCategory("Transfer", "transfer")
	salary := newCategory("Salary", "income")

	// Budgets exist only for the oldest month. The planning query must carry
	// these buckets forward, because rollover rows for past months are created
	// lazily and may never exist.
	newBudget("Rent", rent, "needs", monthsAgo(3), 200000)
	newBudget("Dining", dining, "wants", monthsAgo(3), 50000)
	newBudget("Brokerage", brokerage, "savings", monthsAgo(3), 100000)

	// Three complete months of identical activity, plus an uncategorized
	// outflow that must land in the unbucketed column.
	for _, n := range []int{3, 2, 1} {
		setCategory(insertTxn(t, pool, userID, accID, 500000, dayIn(n, 1), "paycheck"), salary)
		setCategory(insertTxn(t, pool, userID, accID, -200000, dayIn(n, 2), "rent"), rent)
		setCategory(insertTxn(t, pool, userID, accID, -50000, dayIn(n, 3), "dinner"), dining)
		setCategory(insertTxn(t, pool, userID, accID, -100000, dayIn(n, 4), "invest"), brokerage)
		insertTxn(t, pool, userID, accID, -25000, dayIn(n, 5), "mystery charge")

		// Neither of these may reach any bucket or the cashflow totals.
		setCategory(insertTxn(t, pool, userID, accID, -300000, dayIn(n, 6), "to savings"), transfer)
		softDeleteTxn(t, pool, insertTxn(t, pool, userID, accID, -99900, dayIn(n, 7), "deleted"))
	}

	baseline, err := repo.GetPlanningBaseline(ctx, userID, 6)
	if err != nil {
		t.Fatalf("GetPlanningBaseline: %v", err)
	}

	if len(baseline.Months) != 6 {
		t.Fatalf("got %d months, want 6", len(baseline.Months))
	}

	// The reconciliation invariant: every dollar of outflow is attributed to
	// exactly one bucket. If this breaks, essentials are wrong and so is every
	// emergency-fund number on the page.
	for _, m := range baseline.Months {
		sum := m.Needs + m.Wants + m.Savings + m.Unbucketed
		if sum != m.Outflow {
			t.Errorf("%s: needs+wants+savings+unbucketed = %d, want outflow %d",
				m.Month.Format("2006-01"), sum.ToInt64(), m.Outflow.ToInt64())
		}
	}

	byKey := map[string]int{}
	for i, m := range baseline.Months {
		byKey[m.Month.Format("2006-01")] = i
	}

	for _, n := range []int{3, 2, 1} {
		key := monthsAgo(n)[:7]
		idx, ok := byKey[key]
		if !ok {
			t.Fatalf("month %s missing from baseline", key)
		}
		m := baseline.Months[idx]

		checks := []struct {
			name string
			got  int64
			want int64
		}{
			{"income", m.Income.ToInt64(), 500000},
			// Transfers and soft-deleted rows are excluded.
			{"outflow", m.Outflow.ToInt64(), 375000},
			{"needs", m.Needs.ToInt64(), 200000},
			{"wants", m.Wants.ToInt64(), 50000},
			{"savings", m.Savings.ToInt64(), 100000},
			{"unbucketed", m.Unbucketed.ToInt64(), 25000},
		}
		for _, c := range checks {
			if c.got != c.want {
				t.Errorf("%s %s = %d, want %d", key, c.name, c.got, c.want)
			}
		}
	}

	// Cashflow must agree with the dashboard's own figures over the same range.
	start := mustParseDate(t, monthsAgo(2))
	end := start.AddDate(0, 1, -1)
	summary, err := repo.GetReportsSummary(ctx, userID, start, end)
	if err != nil {
		t.Fatalf("GetReportsSummary: %v", err)
	}
	planningMonth := baseline.Months[byKey[monthsAgo(2)[:7]]]
	if summary.InPeriod != planningMonth.Income {
		t.Errorf("income = %d, /reports/summary in_period = %d",
			planningMonth.Income.ToInt64(), summary.InPeriod.ToInt64())
	}
	if summary.OutPeriod != planningMonth.Outflow {
		t.Errorf("outflow = %d, /reports/summary out_period = %d",
			planningMonth.Outflow.ToInt64(), summary.OutPeriod.ToInt64())
	}

	// Account rollups. Assets minus the card's balance, with liabilities stored
	// negative so net worth is a plain sum.
	if _, err := pool.Exec(ctx,
		`INSERT INTO transactions (account_id, user_id, amount, date, description, is_reviewed)
		 VALUES ($1, $2, $3, $4::date, $5, true)`,
		cardID, userID, -40000, dayIn(1, 8), "card balance",
	); err != nil {
		t.Fatalf("insert card transaction: %v", err)
	}

	baseline, err = repo.GetPlanningBaseline(ctx, userID, 6)
	if err != nil {
		t.Fatalf("GetPlanningBaseline (after card): %v", err)
	}
	if len(baseline.Accounts) != 2 {
		t.Fatalf("got %d accounts, want 2", len(baseline.Accounts))
	}
	if baseline.TotalLiabilities.ToInt64() != -40000 {
		t.Errorf("total_liabilities = %d, want -40000", baseline.TotalLiabilities.ToInt64())
	}
	if baseline.NetWorth != baseline.LiquidAssets+baseline.TotalLiabilities {
		t.Errorf("net_worth = %d, want liquid_assets + total_liabilities = %d",
			baseline.NetWorth.ToInt64(),
			(baseline.LiquidAssets + baseline.TotalLiabilities).ToInt64())
	}
}

func TestPlanningBaselineClampsMonthsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "planning_clamp")

	for _, tc := range []struct{ in, want int }{
		{0, 1}, {-5, 1}, {1, 1}, {6, 6}, {24, 24}, {100, 24},
	} {
		baseline, err := repo.GetPlanningBaseline(ctx, userID, tc.in)
		if err != nil {
			t.Fatalf("GetPlanningBaseline(%d): %v", tc.in, err)
		}
		if len(baseline.Months) != tc.want {
			t.Errorf("months=%d returned %d rows, want %d", tc.in, len(baseline.Months), tc.want)
		}
	}
}

// A user with no data at all must still get a well-formed, zeroed baseline
// rather than an error or nil slices, since the page renders it directly.
func TestPlanningBaselineEmptyUserIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "planning_empty")

	baseline, err := repo.GetPlanningBaseline(ctx, userID, 6)
	if err != nil {
		t.Fatalf("GetPlanningBaseline: %v", err)
	}
	if baseline.Accounts == nil {
		t.Error("Accounts is nil, want an empty slice so JSON encodes as []")
	}
	if len(baseline.Months) != 6 {
		t.Errorf("got %d months, want 6 zeroed rows", len(baseline.Months))
	}
	for _, m := range baseline.Months {
		if m.Income != 0 || m.Outflow != 0 || m.NetWorth != 0 {
			t.Errorf("%s: want all zeroes, got income=%d outflow=%d net_worth=%d",
				m.Month.Format("2006-01"), m.Income.ToInt64(), m.Outflow.ToInt64(), m.NetWorth.ToInt64())
		}
	}
	if baseline.NetWorth != 0 || baseline.LiquidAssets != 0 {
		t.Errorf("want zero net worth and liquid assets, got %d / %d",
			baseline.NetWorth.ToInt64(), baseline.LiquidAssets.ToInt64())
	}
}
