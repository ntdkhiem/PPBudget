package service

// Integration tests for the rules engine apply/preview path. Run against a
// real Postgres instance, gated on TEST_DATABASE_URL:
//
//	make test-integration
//
// Reuses sfSetupTestDB / sfCreateTestUser from simplefin_integration_test.go.

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ntdkhiem/ppbudget-go/internal/config"
	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/internal/repository"
)

func newRulesTestService(pool *pgxpool.Pool) *Service {
	return New(repository.New(pool), slog.New(slog.NewTextHandler(io.Discard, nil)), &config.Config{})
}

func mkAccount(t *testing.T, pool *pgxpool.Pool, userID, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO accounts (user_id, name, type, currency) VALUES ($1, $2, 'asset', 'USD') RETURNING id`,
		userID, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create account %q: %v", name, err)
	}
	return id
}

func mkCategory(t *testing.T, pool *pgxpool.Pool, userID, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO categories (user_id, name, type) VALUES ($1, $2, 'expense') RETURNING id`,
		userID, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create category %q: %v", name, err)
	}
	return id
}

func mkTxn(t *testing.T, pool *pgxpool.Pool, userID, accountID string, cents int64, date, description string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO transactions (account_id, user_id, amount, date, description)
		 VALUES ($1, $2, $3, $4::date, $5) RETURNING id`,
		accountID, userID, cents, date, description,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create transaction %q: %v", description, err)
	}
	return id
}

func mustParseDay(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return tm
}

func categoryOf(t *testing.T, pool *pgxpool.Pool, txnID string) *string {
	t.Helper()
	var cat *string
	if err := pool.QueryRow(context.Background(),
		`SELECT category_id FROM transactions WHERE id = $1`, txnID).Scan(&cat); err != nil {
		t.Fatalf("failed to read category of %s: %v", txnID, err)
	}
	return cat
}

func mkRule(t *testing.T, svc *Service, userID, name string, priority int, active bool, conds []domain.RuleCondition, acts []domain.RuleAction) *domain.Rule {
	t.Helper()
	r := &domain.Rule{
		Name:        name,
		TriggerType: "store-journal",
		Strictness:  StrictnessAll,
		Priority:    priority,
		IsActive:    active,
		Conditions:  conds,
		Actions:     acts,
	}
	if err := svc.CreateRule(context.Background(), userID, r); err != nil {
		t.Fatalf("failed to create rule %q: %v", name, err)
	}
	return r
}

// TestApplyRuleIntegration covers the core promise: a rule updates exactly the
// transactions it matches, and reports an accurate count.
func TestApplyRuleIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)

	userID := sfCreateTestUser(t, pool)
	account := mkAccount(t, pool, userID, "Checking")
	coffeeCat := mkCategory(t, pool, userID, "Coffee")

	starbucks1 := mkTxn(t, pool, userID, account, -450, "2026-01-05", "STARBUCKS #1")
	starbucks2 := mkTxn(t, pool, userID, account, -675, "2026-02-05", "starbucks #2")
	amazon := mkTxn(t, pool, userID, account, -9900, "2026-01-06", "AMZN MKTP")

	rule := mkRule(t, svc, userID, "Coffee", 0, true,
		[]domain.RuleCondition{cond(FieldDescription, OpContains, "starbucks")},
		[]domain.RuleAction{{ActionType: ActionSetCategory, Value: coffeeCat}},
	)

	updated, err := svc.ApplyRule(ctx, userID, rule.ID, true, nil, nil)
	if err != nil {
		t.Fatalf("ApplyRule: %v", err)
	}
	if updated != 2 {
		t.Errorf("updated_count = %d, want 2", updated)
	}

	// Case-insensitive matching means both Starbucks rows are categorized.
	for _, id := range []string{starbucks1, starbucks2} {
		got := categoryOf(t, pool, id)
		if got == nil || *got != coffeeCat {
			t.Errorf("transaction %s: category = %v, want %s", id, got, coffeeCat)
		}
	}
	if got := categoryOf(t, pool, amazon); got != nil {
		t.Errorf("non-matching transaction was categorized: %v", *got)
	}

	// Re-applying changes nothing, since the rows already hold the target
	// category. A rerun must not inflate the count.
	again, err := svc.ApplyRule(ctx, userID, rule.ID, true, nil, nil)
	if err != nil {
		t.Fatalf("second ApplyRule: %v", err)
	}
	if again != 0 {
		t.Errorf("re-applying an already-applied rule updated %d rows, want 0", again)
	}
}

// TestPreviewMatchesApplyIntegration is the property that makes preview worth
// trusting: whatever the dry run promises is exactly what the apply delivers.
func TestPreviewMatchesApplyIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)

	userID := sfCreateTestUser(t, pool)
	account := mkAccount(t, pool, userID, "Checking")
	shopping := mkCategory(t, pool, userID, "Shopping")

	for i, desc := range []string{"AMZN MKTP US", "AMZN Digital", "amzn retail", "TARGET", "COSTCO"} {
		mkTxn(t, pool, userID, account, int64(-1000*(i+1)), "2026-03-0"+string(rune('1'+i)), desc)
	}

	draft := &domain.Rule{
		Name:       "Amazon",
		Strictness: StrictnessAll,
		Conditions: []domain.RuleCondition{cond(FieldDescription, OpMatchesRegex, "^amzn")},
		Actions:    []domain.RuleAction{{ActionType: ActionSetCategory, Value: shopping}},
	}

	preview, err := svc.PreviewRule(ctx, userID, draft, true, nil, nil, 10)
	if err != nil {
		t.Fatalf("PreviewRule: %v", err)
	}
	if preview.MatchCount != 3 {
		t.Errorf("preview match_count = %d, want 3", preview.MatchCount)
	}
	if preview.WouldUpdateCount != 3 {
		t.Errorf("preview would_update_count = %d, want 3", preview.WouldUpdateCount)
	}
	if preview.TotalScanned != 5 {
		t.Errorf("preview total_scanned = %d, want 5", preview.TotalScanned)
	}
	if len(preview.Samples) != 3 {
		t.Fatalf("preview returned %d samples, want 3", len(preview.Samples))
	}
	for _, s := range preview.Samples {
		if s.NewCategoryID == nil || *s.NewCategoryID != shopping {
			t.Errorf("sample %s: new_category_id = %v, want %s", s.ID, s.NewCategoryID, shopping)
		}
		if s.CurrentCategoryID != nil {
			t.Errorf("sample %s: current_category_id = %v, want nil", s.ID, *s.CurrentCategoryID)
		}
	}

	// Nothing was written by the preview.
	var categorized int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE user_id = $1 AND category_id IS NOT NULL`, userID,
	).Scan(&categorized); err != nil {
		t.Fatalf("count categorized: %v", err)
	}
	if categorized != 0 {
		t.Fatalf("preview wrote %d categories; a dry run must not write", categorized)
	}

	// Now save and apply the same rule: the count must equal the preview.
	rule := mkRule(t, svc, userID, "Amazon", 0, true, draft.Conditions, draft.Actions)
	updated, err := svc.ApplyRule(ctx, userID, rule.ID, true, nil, nil)
	if err != nil {
		t.Fatalf("ApplyRule: %v", err)
	}
	if updated != preview.WouldUpdateCount {
		t.Errorf("apply updated %d, but preview promised %d", updated, preview.WouldUpdateCount)
	}
}

// TestApplyRuleDateScopingIntegration checks that a date range actually narrows
// the set, since the previous UI sent an empty range meaning "everything".
func TestApplyRuleDateScopingIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)

	userID := sfCreateTestUser(t, pool)
	account := mkAccount(t, pool, userID, "Checking")
	cat := mkCategory(t, pool, userID, "Coffee")

	old := mkTxn(t, pool, userID, account, -500, "2025-06-01", "STARBUCKS old")
	recent := mkTxn(t, pool, userID, account, -500, "2026-03-01", "STARBUCKS recent")

	rule := mkRule(t, svc, userID, "Coffee", 0, true,
		[]domain.RuleCondition{cond(FieldDescription, OpContains, "starbucks")},
		[]domain.RuleAction{{ActionType: ActionSetCategory, Value: cat}},
	)

	start := mustParseDay(t, "2026-01-01")
	updated, err := svc.ApplyRule(ctx, userID, rule.ID, false, &start, nil)
	if err != nil {
		t.Fatalf("ApplyRule: %v", err)
	}
	if updated != 1 {
		t.Errorf("updated_count = %d, want 1 (only the in-range transaction)", updated)
	}
	if got := categoryOf(t, pool, old); got != nil {
		t.Error("a transaction before the start date was updated")
	}
	if got := categoryOf(t, pool, recent); got == nil || *got != cat {
		t.Error("the in-range transaction was not updated")
	}
}

// TestApplyRulesPriorityIntegration checks that when two rules match the same
// transaction, the higher priority wins -- previously arbitrary, since the UI
// never set a priority.
func TestApplyRulesPriorityIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)

	userID := sfCreateTestUser(t, pool)
	account := mkAccount(t, pool, userID, "Checking")
	generic := mkCategory(t, pool, userID, "Shopping")
	specific := mkCategory(t, pool, userID, "Groceries")

	txnID := mkTxn(t, pool, userID, account, -5000, "2026-03-01", "AMZN FRESH GROCERY")

	// Both rules match. The higher priority must be the one that sticks.
	mkRule(t, svc, userID, "Broad Amazon", 1, true,
		[]domain.RuleCondition{cond(FieldDescription, OpContains, "amzn")},
		[]domain.RuleAction{{ActionType: ActionSetCategory, Value: generic}},
	)
	mkRule(t, svc, userID, "Amazon Fresh", 10, true,
		[]domain.RuleCondition{cond(FieldDescription, OpContains, "amzn fresh")},
		[]domain.RuleAction{{ActionType: ActionSetCategory, Value: specific}},
	)

	if _, err := svc.ApplyActiveRules(ctx, userID, true, nil, nil); err != nil {
		t.Fatalf("ApplyActiveRules: %v", err)
	}

	got := categoryOf(t, pool, txnID)
	if got == nil || *got != specific {
		t.Errorf("category = %v, want the higher-priority rule's category %s", got, specific)
	}
}

// TestApplyActiveRulesSkipsInactiveIntegration covers the enable/disable toggle
// the UI now exposes.
func TestApplyActiveRulesSkipsInactiveIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)

	userID := sfCreateTestUser(t, pool)
	account := mkAccount(t, pool, userID, "Checking")
	cat := mkCategory(t, pool, userID, "Coffee")

	txnID := mkTxn(t, pool, userID, account, -500, "2026-03-01", "STARBUCKS")

	rule := mkRule(t, svc, userID, "Coffee (disabled)", 0, false,
		[]domain.RuleCondition{cond(FieldDescription, OpContains, "starbucks")},
		[]domain.RuleAction{{ActionType: ActionSetCategory, Value: cat}},
	)

	updated, err := svc.ApplyActiveRules(ctx, userID, true, nil, nil)
	if err != nil {
		t.Fatalf("ApplyActiveRules: %v", err)
	}
	if updated != 0 {
		t.Errorf("an inactive rule updated %d transactions, want 0", updated)
	}
	if got := categoryOf(t, pool, txnID); got != nil {
		t.Error("an inactive rule categorized a transaction")
	}

	// Enabling it makes the same rule fire.
	rule.IsActive = true
	if err := svc.UpdateRule(ctx, userID, rule); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if updated, err = svc.ApplyActiveRules(ctx, userID, true, nil, nil); err != nil {
		t.Fatalf("ApplyActiveRules after enabling: %v", err)
	}
	if updated != 1 {
		t.Errorf("after enabling, updated = %d, want 1", updated)
	}
}

// TestApplyRuleRejectsInvalidIntegration checks that validation runs on the
// write path, so a rule that would silently never fire cannot be stored.
func TestApplyRuleRejectsInvalidIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)
	cat := mkCategory(t, pool, userID, "Coffee")

	invalid := &domain.Rule{
		Name:       "Legacy",
		Strictness: StrictnessAll,
		// The operator the old UI emitted.
		Conditions: []domain.RuleCondition{cond(FieldDescription, "is", "STARBUCKS")},
		Actions:    []domain.RuleAction{{ActionType: ActionSetCategory, Value: cat}},
	}
	if err := svc.CreateRule(ctx, userID, invalid); err == nil {
		t.Error("expected CreateRule to reject the legacy 'is' operator")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM rules WHERE user_id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if count != 0 {
		t.Errorf("an invalid rule was stored anyway (%d rows)", count)
	}
}
