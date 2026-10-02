package repository

// An inflow in a budgeted category (a refund, a friend paying you back) gives
// budget back rather than counting as more spending. Gated on TEST_DATABASE_URL.

import (
	"context"
	"testing"
	"time"
)

func TestBudgetSpentNetsRefundsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "budget-refund")

	acct, err := repo.CreateAccount(ctx, userID, "Checking", "asset", "USD", 500_000, nil)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	var groceries string
	if err := pool.QueryRow(ctx,
		`INSERT INTO categories (user_id, name, type) VALUES ($1, 'Groceries', 'expense') RETURNING id`, userID,
	).Scan(&groceries); err != nil {
		t.Fatalf("category: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO budgets (user_id, name, category_id, amount, period_type, start_date, end_date, bucket)
		 VALUES ($1, 'Groceries', $2, 50000, 'monthly', '2026-06-01', '2026-06-30', 'needs')`,
		userID, groceries); err != nil {
		t.Fatalf("budget: %v", err)
	}

	for _, txn := range []struct {
		amount int64
		date   string
		desc   string
	}{
		{-8_000, "2026-06-05", "GROCER"},
		{2_000, "2026-06-09", "GROCER REFUND"},
	} {
		id := insertTxn(t, pool, userID, acct, txn.amount, txn.date, txn.desc)
		if _, err := pool.Exec(ctx, `UPDATE transactions SET category_id = $1 WHERE id = $2`, groceries, id); err != nil {
			t.Fatalf("categorise %s: %v", txn.desc, err)
		}
	}

	june := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	sums, err := repo.GetBudgetsSummary(ctx, userID, june)
	if err != nil {
		t.Fatalf("GetBudgetsSummary: %v", err)
	}
	var found bool
	for _, b := range sums {
		if b.CategoryID != groceries {
			continue
		}
		found = true
		if s := b.SpentTotal.ToInt64(); s != 6_000 {
			t.Errorf("budgets page spent: %d, want 6000", s)
		}
		if l := b.LeftTotal.ToInt64(); l != 44_000 {
			t.Errorf("budgets page left: %d, want 44000", l)
		}
	}
	if !found {
		t.Fatal("no groceries budget in June")
	}

	summary, err := repo.GetReportsSummary(ctx, userID, june, mustParseDate(t, "2026-06-30"))
	if err != nil {
		t.Fatalf("GetReportsSummary: %v", err)
	}
	if l := summary.LeftToSpend.ToInt64(); l != 44_000 {
		t.Errorf("reports left to spend: %d, want 44000", l)
	}
}
