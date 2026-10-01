package repository

// Gated on TEST_DATABASE_URL; see balances_integration_test.go for the helpers.

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

func mergeTestCategory(t *testing.T, pool *pgxpool.Pool, userID, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO categories (user_id, name, type) VALUES ($1, $2, 'expense') RETURNING id`,
		userID, name,
	).Scan(&id); err != nil {
		t.Fatalf("insert category %s: %v", name, err)
	}
	return id
}

func mergeTestBudget(t *testing.T, pool *pgxpool.Pool, userID, categoryID, start, end, bucket string, amount int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO budgets (name, category_id, amount, period_type, start_date, end_date, bucket, user_id)
		 VALUES ('b', $1, $2, 'monthly', $3::date, $4::date, $5, $6)`,
		categoryID, amount, start, end, bucket, userID,
	); err != nil {
		t.Fatalf("insert budget: %v", err)
	}
}

func TestMergeCategoriesIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "merge")
	accID, err := repo.CreateAccount(ctx, userID, "Merge Test", "asset", "USD", 0, nil)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	target := mergeTestCategory(t, pool, userID, "Food")
	grocery := mergeTestCategory(t, pool, userID, "Grocery")
	dining := mergeTestCategory(t, pool, userID, "Dining")

	setCategory := func(txnID, catID string) {
		if _, err := pool.Exec(ctx, `UPDATE transactions SET category_id = $1 WHERE id = $2`, catID, txnID); err != nil {
			t.Fatalf("set category: %v", err)
		}
	}
	live1 := insertTxn(t, pool, userID, accID, -1000, "2026-06-03", "costco")
	setCategory(live1, grocery)
	live2 := insertTxn(t, pool, userID, accID, -2000, "2026-06-04", "cafe")
	setCategory(live2, dining)
	deleted := insertTxn(t, pool, userID, accID, -500, "2026-06-05", "deleted cafe")
	setCategory(deleted, dining)
	softDeleteTxn(t, pool, deleted)
	already := insertTxn(t, pool, userID, accID, -700, "2026-06-06", "already food")
	setCategory(already, target)

	if _, err := pool.Exec(ctx,
		`INSERT INTO subscriptions (name, amount, billing_cycle, next_billing_date, category_id, user_id)
		 VALUES ('Meal kit', 6000, 'monthly', '2026-07-01', $1, $2)`, dining, userID); err != nil {
		t.Fatalf("insert subscription: %v", err)
	}

	var ruleID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO rules (name, user_id) VALUES ('cafes', $1) RETURNING id`, userID).Scan(&ruleID); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO rule_actions (rule_id, action_type, value) VALUES ($1, 'set_category', $2)`, ruleID, dining); err != nil {
		t.Fatalf("insert rule action: %v", err)
	}

	// June: the target has a budget, so both source limits add onto it.
	mergeTestBudget(t, pool, userID, target, "2026-06-01", "2026-06-30", "needs", 10000)
	mergeTestBudget(t, pool, userID, grocery, "2026-06-01", "2026-06-30", "needs", 5000)
	mergeTestBudget(t, pool, userID, dining, "2026-06-01", "2026-06-30", "wants", 2500)
	// July: only the sources budgeted, so the target gets one combined row
	// carrying the bucket of the larger limit.
	mergeTestBudget(t, pool, userID, grocery, "2026-07-01", "2026-07-31", "needs", 2000)
	mergeTestBudget(t, pool, userID, dining, "2026-07-01", "2026-07-31", "wants", 3000)

	res, err := repo.MergeCategories(ctx, userID, target, []string{grocery, dining})
	if err != nil {
		t.Fatalf("MergeCategories: %v", err)
	}
	if res.Transactions != 2 || res.Subscriptions != 1 || res.RuleActions != 1 || res.Budgets != 4 || res.DeletedCategories != 2 {
		t.Errorf("result = %+v, want 2 transactions, 1 subscription, 1 rule action, 4 budgets, 2 deleted", res)
	}

	for _, id := range []string{live1, live2, deleted, already} {
		var cat *string
		if err := pool.QueryRow(ctx, `SELECT category_id FROM transactions WHERE id = $1`, id).Scan(&cat); err != nil {
			t.Fatalf("read transaction: %v", err)
		}
		if cat == nil || *cat != target {
			t.Errorf("transaction %s category = %v, want target", id, cat)
		}
	}

	var subCat, ruleValue string
	if err := pool.QueryRow(ctx, `SELECT category_id FROM subscriptions WHERE user_id = $1`, userID).Scan(&subCat); err != nil {
		t.Fatalf("read subscription: %v", err)
	}
	if subCat != target {
		t.Errorf("subscription category = %s, want target", subCat)
	}
	if err := pool.QueryRow(ctx, `SELECT value FROM rule_actions WHERE rule_id = $1`, ruleID).Scan(&ruleValue); err != nil {
		t.Fatalf("read rule action: %v", err)
	}
	if ruleValue != target {
		t.Errorf("rule action value = %s, want target", ruleValue)
	}

	rows, err := pool.Query(ctx,
		`SELECT category_id, to_char(start_date, 'YYYY-MM'), amount, bucket, name FROM budgets WHERE user_id = $1 ORDER BY start_date`, userID)
	if err != nil {
		t.Fatalf("read budgets: %v", err)
	}
	type budgetRow struct {
		cat, month, bucket, name string
		amount                   int64
	}
	var budgets []budgetRow
	for rows.Next() {
		var b budgetRow
		if err := rows.Scan(&b.cat, &b.month, &b.amount, &b.bucket, &b.name); err != nil {
			t.Fatalf("scan budget: %v", err)
		}
		budgets = append(budgets, b)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("budgets rows: %v", err)
	}
	want := []budgetRow{
		{cat: target, month: "2026-06", amount: 17500, bucket: "needs", name: "b"},
		{cat: target, month: "2026-07", amount: 5000, bucket: "wants", name: "Food Budget"},
	}
	if len(budgets) != len(want) {
		t.Fatalf("budgets = %+v, want %+v", budgets, want)
	}
	for i := range want {
		if budgets[i] != want[i] {
			t.Errorf("budget %d = %+v, want %+v", i, budgets[i], want[i])
		}
	}

	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM categories WHERE id = ANY($1)`, []string{grocery, dining}).Scan(&remaining); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d source categories survived the merge", remaining)
	}
}

// TestMergeCategoriesTenancyIntegration: naming another user's category, as
// source or target, changes nothing and reads as not found.
func TestMergeCategoriesTenancyIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	alice := createTestUser(t, pool, "merge-alice")
	bob := createTestUser(t, pool, "merge-bob")
	aliceCat := mergeTestCategory(t, pool, alice, "Mine")
	bobCat := mergeTestCategory(t, pool, bob, "Theirs")

	if _, err := repo.MergeCategories(ctx, alice, aliceCat, []string{bobCat}); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("merging a foreign source: err = %v, want ErrNotFound", err)
	}
	if _, err := repo.MergeCategories(ctx, alice, bobCat, []string{aliceCat}); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("merging into a foreign target: err = %v, want ErrNotFound", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM categories WHERE id = ANY($1)`, []string{aliceCat, bobCat}).Scan(&n); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if n != 2 {
		t.Errorf("%d of 2 categories left after rejected merges", n)
	}
}
