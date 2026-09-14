package repository

// Integration tests for dashboard report queries. Gated on TEST_DATABASE_URL;
// see balances_integration_test.go for the helpers and the throwaway-database warning.

import (
	"context"
	"testing"

	"ntdkhiem/ppbudget-go/internal/domain"
)

func TestSpendingByCategoryIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "spending")
	accID, err := repo.CreateAccount(ctx, userID, "Spending Test", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
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

	groceries := newCategory("Groceries", "expense")
	dining := newCategory("Dining", "expense")
	transfer := newCategory("Transfer", "transfer")
	salary := newCategory("Salary", "income")

	// More than one page of transactions, which the old client-side aggregation truncated.
	for i := 0; i < 120; i++ {
		setCategory(insertTxn(t, pool, userID, accID, -1000, "2025-03-10", "groceries"), groceries)
	}

	// Dining: -$60 paid, $20 of it reimbursed by a linked income transaction -> effective -$40.
	dinner := insertTxn(t, pool, userID, accID, -6000, "2025-03-12", "dinner")
	setCategory(dinner, dining)
	reimburse := insertTxn(t, pool, userID, accID, 2000, "2025-03-13", "friend pays back")
	if _, err := pool.Exec(ctx,
		`INSERT INTO transaction_links (source_transaction_id, target_transaction_id, amount) VALUES ($1, $2, $3)`,
		reimburse, dinner, 2000,
	); err != nil {
		t.Fatalf("insert link: %v", err)
	}

	insertTxn(t, pool, userID, accID, -2500, "2025-03-14", "uncategorized")
	setCategory(insertTxn(t, pool, userID, accID, -50000, "2025-03-15", "to savings"), transfer)
	setCategory(insertTxn(t, pool, userID, accID, 300000, "2025-03-01", "paycheck"), salary)
	softDeleteTxn(t, pool, insertTxn(t, pool, userID, accID, -99900, "2025-03-16", "deleted"))
	insertTxn(t, pool, userID, accID, -77700, "2025-04-02", "out of range")

	spending, err := repo.GetSpendingByCategory(ctx, userID, mustParseDate(t, "2025-03-01"), mustParseDate(t, "2025-03-31"))
	if err != nil {
		t.Fatalf("GetSpendingByCategory: %v", err)
	}

	got := map[string]int64{}
	for _, s := range spending {
		got[s.Name] = s.TotalSpent.ToInt64()
		if s.Name == "Uncategorized" && s.CategoryID != nil {
			t.Errorf("Uncategorized row has category_id %q, want nil", *s.CategoryID)
		}
	}
	want := map[string]int64{"Groceries": 120000, "Dining": 4000, "Uncategorized": 2500}
	if len(got) != len(want) {
		t.Errorf("got categories %v, want %v", got, want)
	}
	for name, cents := range want {
		if got[name] != cents {
			t.Errorf("%s = %d, want %d", name, got[name], cents)
		}
	}
	if len(spending) > 0 && spending[0].Name != "Groceries" {
		t.Errorf("first row = %s, want Groceries (sorted by total desc)", spending[0].Name)
	}

	// Top Spending must agree with the summary's out_period.
	summary, err := repo.GetReportsSummary(ctx, userID, mustParseDate(t, "2025-03-01"), mustParseDate(t, "2025-03-31"))
	if err != nil {
		t.Fatalf("GetReportsSummary: %v", err)
	}
	var total int64
	for _, cents := range got {
		total += cents
	}
	if summary.OutPeriod.ToInt64() != total {
		t.Errorf("out_period = %d, sum of spending = %d", summary.OutPeriod.ToInt64(), total)
	}

	for _, tc := range []struct{ limit, want int }{{5, 5}, {0, 100}, {500, 100}} {
		txns, err := repo.ListTransactions(ctx, domain.TransactionFilter{UserID: userID, Limit: tc.limit})
		if err != nil {
			t.Fatalf("ListTransactions(limit=%d): %v", tc.limit, err)
		}
		if len(txns) != tc.want {
			t.Errorf("ListTransactions(limit=%d) returned %d rows, want %d", tc.limit, len(txns), tc.want)
		}
	}
}
