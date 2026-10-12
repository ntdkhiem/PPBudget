package repository

// A category has at most one budget per month (budgets_user_category_month_key).
// Gated on TEST_DATABASE_URL, which must be migrated through 0034.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

func budgetTestCategory(t *testing.T, repo *Repository, userID, name string) string {
	t.Helper()
	var id string
	if err := repo.pool.QueryRow(context.Background(),
		`INSERT INTO categories (user_id, name, type) VALUES ($1, $2, 'expense') RETURNING id`, userID, name,
	).Scan(&id); err != nil {
		t.Fatalf("category %s: %v", name, err)
	}
	return id
}

func countBudgets(t *testing.T, repo *Repository, userID, start string) int {
	t.Helper()
	var n int
	if err := repo.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM budgets WHERE user_id = $1 AND start_date = $2::date`, userID, start,
	).Scan(&n); err != nil {
		t.Fatalf("count budgets: %v", err)
	}
	return n
}

// The first views of a month race to copy last month's budgets forward; only
// one copy may land.
func TestBudgetRolloverConcurrentIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "budget-rollover-race")

	for _, name := range []string{"Rent", "Groceries", "Dining"} {
		mergeTestBudget(t, pool, userID, budgetTestCategory(t, repo, userID, name), "2026-05-01", "2026-05-31", "needs", 10000)
	}

	june := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.GetBudgetsSummary(ctx, userID, june, time.Now()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("GetBudgetsSummary: %v", err)
	}

	if n := countBudgets(t, repo, userID, "2026-06-01"); n != 3 {
		t.Errorf("June budgets = %d, want 3 (one per category)", n)
	}
	sums, err := repo.GetBudgetsSummary(ctx, userID, june, time.Now())
	if err != nil {
		t.Fatalf("GetBudgetsSummary: %v", err)
	}
	if len(sums) != 3 {
		t.Errorf("June summary rows = %d, want 3", len(sums))
	}
}

func TestCreateBudgetDuplicateConflictIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "budget-duplicate")
	cat := budgetTestCategory(t, repo, userID, "Groceries")

	newBudget := func(periodType string) *domain.Budget {
		return &domain.Budget{
			UserID: userID, Name: "Groceries Budget", CategoryID: cat, Amount: money.Money(50000),
			PeriodType: periodType, Bucket: "needs",
			StartDate: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC),
		}
	}
	if err := repo.CreateBudget(ctx, newBudget("monthly")); err != nil {
		t.Fatalf("first CreateBudget: %v", err)
	}
	if err := repo.CreateBudget(ctx, newBudget("one_time")); !errors.Is(err, apperrors.ErrConflict) {
		t.Errorf("second CreateBudget err = %v, want ErrConflict", err)
	}
}

// A merge folds a source's one-time budget into the target's monthly one for
// the same month, rather than leaving the target with two.
func TestMergeCategoriesMixedPeriodTypesIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "budget-merge-mixed")
	target := budgetTestCategory(t, repo, userID, "Food")
	source := budgetTestCategory(t, repo, userID, "Party")

	mergeTestBudget(t, pool, userID, target, "2026-06-01", "2026-06-30", "needs", 10000)
	if _, err := pool.Exec(ctx,
		`INSERT INTO budgets (name, category_id, amount, period_type, start_date, end_date, bucket, user_id)
		 VALUES ('b', $1, 4000, 'one_time', '2026-06-01', '2026-06-30', 'wants', $2),
		        ('b', $1, 3000, 'one_time', '2026-07-01', '2026-07-31', 'wants', $2)`,
		source, userID); err != nil {
		t.Fatalf("insert source budgets: %v", err)
	}

	if _, err := repo.MergeCategories(ctx, userID, target, []string{source}); err != nil {
		t.Fatalf("MergeCategories: %v", err)
	}

	for _, want := range []struct {
		month, periodType string
		amount            int64
	}{
		{"2026-06-01", "monthly", 14000},
		{"2026-07-01", "one_time", 3000},
	} {
		var periodType string
		var amount int64
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*), MAX(period_type), COALESCE(SUM(amount), 0)
			 FROM budgets WHERE user_id = $1 AND category_id = $2 AND start_date = $3::date`,
			userID, target, want.month,
		).Scan(&n, &periodType, &amount); err != nil {
			t.Fatalf("read %s: %v", want.month, err)
		}
		if n != 1 || periodType != want.periodType || amount != want.amount {
			t.Errorf("%s: %d rows, %s, %d; want 1 row, %s, %d", want.month, n, periodType, amount, want.periodType, want.amount)
		}
	}
}

// Unbudgeted spending is what the month's budgets miss: expense categories
// without a budget (net of refunds) and uncategorized outflows. Income and
// transfers are not spending.
func TestGetUnbudgetedSpendingIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "budget-unbudgeted")

	acct, err := repo.CreateAccount(ctx, userID, "Checking", "asset", "USD", 0, nil)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	category := func(name, kind string) string {
		var id string
		if err := pool.QueryRow(ctx,
			`INSERT INTO categories (user_id, name, type) VALUES ($1, $2, $3) RETURNING id`, userID, name, kind,
		).Scan(&id); err != nil {
			t.Fatalf("category %s: %v", name, err)
		}
		return id
	}
	groceries := category("Groceries", "expense")
	coffee := category("Coffee", "expense")
	gifts := category("Gifts", "expense")
	salary := category("Salary", "income")
	moves := category("Card Payment", "transfer")
	mergeTestBudget(t, pool, userID, groceries, "2026-06-01", "2026-06-30", "needs", 50_000)

	for _, txn := range []struct {
		amount   int64
		date     string
		category *string
	}{
		{-8_000, "2026-06-03", &groceries}, // budgeted
		{-500, "2026-06-04", &coffee},
		{-300, "2026-06-10", &coffee},
		{200, "2026-06-11", &coffee},   // refund nets off
		{-1_000, "2026-06-12", &gifts}, // fully refunded: nothing left over
		{1_500, "2026-06-13", &gifts},
		{500_000, "2026-06-15", &salary},
		{-40_000, "2026-06-16", &moves},
		{-1_200, "2026-06-17", nil},     // uncategorized outflow
		{300_000, "2026-06-18", nil},    // uncategorized paycheck: not netted
		{-9_999, "2026-07-01", &coffee}, // next month
	} {
		id := insertTxn(t, pool, userID, acct, txn.amount, txn.date, "txn")
		if txn.category != nil {
			if _, err := pool.Exec(ctx, `UPDATE transactions SET category_id = $1 WHERE id = $2`, *txn.category, id); err != nil {
				t.Fatalf("categorise: %v", err)
			}
		}
	}

	june := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	got, err := repo.GetUnbudgetedSpending(ctx, userID, june, time.Now())
	if err != nil {
		t.Fatalf("GetUnbudgetedSpending: %v", err)
	}
	want := []struct {
		name  string
		spent int64
	}{{"Uncategorized", 1_200}, {"Coffee", 600}}
	if len(got) != len(want) {
		t.Fatalf("got %d rows %+v, want %d", len(got), got, len(want))
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].TotalSpent.ToInt64() != w.spent {
			t.Errorf("row %d = %s %d, want %s %d", i, got[i].Name, got[i].TotalSpent.ToInt64(), w.name, w.spent)
		}
	}
	if got[0].CategoryID != nil {
		t.Errorf("Uncategorized category_id = %v, want nil", *got[0].CategoryID)
	}
}
