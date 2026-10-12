package repository

// The month a transaction counts toward: rent posted on 28 June for July.
// Spending by period follows the budget month; the ledger and balances keep
// the posted date. Gated on TEST_DATABASE_URL.

import (
	"context"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
)

func TestBudgetMonthMovesSpendingNotTheLedgerIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "budget-month")

	acct, err := repo.CreateAccount(ctx, userID, "Checking", "asset", "USD", 500_000, nil)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	var rent string
	if err := pool.QueryRow(ctx,
		`INSERT INTO categories (user_id, name, type) VALUES ($1, 'Rent', 'expense') RETURNING id`, userID,
	).Scan(&rent); err != nil {
		t.Fatalf("category: %v", err)
	}
	for _, month := range []string{"2026-06-01", "2026-07-01"} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO budgets (user_id, name, category_id, amount, period_type, start_date, end_date, bucket)
			 VALUES ($1, 'Rent', $2, 250000, 'monthly', $3::date, ($3::date + INTERVAL '1 month - 1 day')::date, 'needs')`,
			userID, rent, month); err != nil {
			t.Fatalf("budget %s: %v", month, err)
		}
	}

	txnID := insertTxn(t, pool, userID, acct, -250_000, "2026-06-28", "RENT - OAK APTS")
	posted := mustParseDate(t, "2026-06-28")
	july := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	balanceBefore := balanceAt(t, pool, acct, "2026-06-30")

	// Categorise it and count it toward July, as the edit dialog does.
	if err := repo.UpdateTransaction(ctx, userID, txnID, acct, -250_000, posted, "RENT - OAK APTS",
		nil, &rent, nil, nil, nil, domain.BudgetMonthChange{Set: true, Month: &july}); err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}

	got, err := repo.GetTransaction(ctx, userID, txnID)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if got.BudgetMonth == nil || !got.BudgetMonth.Equal(july) {
		t.Fatalf("budget month: got %v want July", got.BudgetMonth)
	}
	if !got.Date.Equal(posted) {
		t.Errorf("the posted date should stay 28 June; got %v", got.Date)
	}

	spent := func(month time.Time) int64 {
		t.Helper()
		sums, err := repo.GetBudgetsSummary(ctx, userID, month, time.Now())
		if err != nil {
			t.Fatalf("GetBudgetsSummary %s: %v", month.Format("Jan"), err)
		}
		for _, b := range sums {
			if b.CategoryID == rent {
				return b.SpentTotal.ToInt64()
			}
		}
		t.Fatalf("no rent budget in %s", month.Format("Jan"))
		return 0
	}
	if s := spent(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)); s != 0 {
		t.Errorf("June budget: spent %d, want 0", s)
	}
	if s := spent(july); s != 250_000 {
		t.Errorf("July budget: spent %d, want 250000", s)
	}

	byCategory := func(from, to string) int64 {
		t.Helper()
		rows, err := repo.GetSpendingByCategory(ctx, userID, mustParseDate(t, from), mustParseDate(t, to))
		if err != nil {
			t.Fatalf("GetSpendingByCategory: %v", err)
		}
		var total int64
		for _, r := range rows {
			total += r.TotalSpent.ToInt64()
		}
		return total
	}
	if s := byCategory("2026-06-01", "2026-06-30"); s != 0 {
		t.Errorf("June spending report: %d, want 0", s)
	}
	if s := byCategory("2026-07-01", "2026-07-31"); s == 0 {
		t.Error("July spending report should include the rent")
	}

	// The ledger is the bank's: listed under June by default, and under July
	// only when asked for by budget month.
	list := func(basis, from, to string) int {
		t.Helper()
		start, end := mustParseDate(t, from), mustParseDate(t, to)
		txns, err := repo.ListTransactions(ctx, domain.TransactionFilter{
			UserID: userID, StartDate: &start, EndDate: &end, DateBasis: basis,
		})
		if err != nil {
			t.Fatalf("ListTransactions: %v", err)
		}
		return len(txns)
	}
	if n := list("", "2026-06-01", "2026-06-30"); n != 1 {
		t.Errorf("June ledger: %d rows, want 1", n)
	}
	if n := list(domain.DateBasisBudget, "2026-07-01", "2026-07-31"); n != 1 {
		t.Errorf("July by budget month: %d rows, want 1", n)
	}
	if n := list(domain.DateBasisBudget, "2026-06-01", "2026-06-30"); n != 0 {
		t.Errorf("June by budget month: %d rows, want 0", n)
	}

	// Money left on the 28th whatever month it counts toward.
	if after := balanceAt(t, pool, acct, "2026-06-30"); after != balanceBefore {
		t.Errorf("balance on 30 June moved from %d to %d", balanceBefore, after)
	}

	// An edit that does not name the field leaves it; null clears it.
	if err := repo.UpdateTransaction(ctx, userID, txnID, acct, -250_000, posted, "RENT",
		nil, &rent, nil, nil, nil, domain.BudgetMonthChange{}); err != nil {
		t.Fatalf("UpdateTransaction (unnamed): %v", err)
	}
	if got, _ := repo.GetTransaction(ctx, userID, txnID); got.BudgetMonth == nil {
		t.Error("an edit that does not name the budget month should leave it")
	}
	if err := repo.UpdateTransaction(ctx, userID, txnID, acct, -250_000, posted, "RENT",
		nil, &rent, nil, nil, nil, domain.BudgetMonthChange{Set: true}); err != nil {
		t.Fatalf("UpdateTransaction (clear): %v", err)
	}
	if got, _ := repo.GetTransaction(ctx, userID, txnID); got.BudgetMonth != nil {
		t.Errorf("null should clear the budget month; got %v", got.BudgetMonth)
	}

	// The schema holds budget months to the first of a month.
	if _, err := pool.Exec(ctx, `UPDATE transactions SET budget_month = '2026-07-15' WHERE id = $1`, txnID); err == nil {
		t.Error("a budget month that is not the first of a month should be refused")
	}
}

// The planning baseline buckets by budget month: rent paid in the last days
// of last month counts toward this one.
func TestPlanningBaselineFollowsBudgetMonthIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "budget-month-plan")

	acct, err := repo.CreateAccount(ctx, userID, "Checking", "asset", "USD", 500_000, nil)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	now := time.Now().UTC()
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastMonth := thisMonth.AddDate(0, -1, 0)
	posted := thisMonth.AddDate(0, 0, -2) // two days before this month

	txnID := insertTxn(t, pool, userID, acct, -250_000, posted.Format("2006-01-02"), "RENT")
	if _, err := pool.Exec(ctx, `UPDATE transactions SET budget_month = $1 WHERE id = $2`, thisMonth, txnID); err != nil {
		t.Fatalf("set budget month: %v", err)
	}

	pb, err := repo.GetPlanningBaseline(ctx, userID, 3)
	if err != nil {
		t.Fatalf("GetPlanningBaseline: %v", err)
	}
	outflow := map[string]int64{}
	for _, m := range pb.Months {
		outflow[m.Month.Format("2006-01")] = m.Outflow.ToInt64()
	}
	if o := outflow[lastMonth.Format("2006-01")]; o != 0 {
		t.Errorf("last month's outflow: %d, want 0", o)
	}
	if o := outflow[thisMonth.Format("2006-01")]; o != 250_000 {
		t.Errorf("this month's outflow: %d, want 250000", o)
	}
}
