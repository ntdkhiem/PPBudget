package service

import (
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

const (
	acctChecking = "11111111-1111-1111-1111-111111111111"
	acctSavings  = "22222222-2222-2222-2222-222222222222"
)

func txn(description string, amountCents int64, accountID string) domain.Transaction {
	return domain.Transaction{
		ID:          "t1",
		AccountID:   accountID,
		Amount:      money.Money(amountCents),
		Date:        time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Description: description,
	}
}

func cond(field, operator, value string) domain.RuleCondition {
	return domain.RuleCondition{Field: field, Operator: operator, Value: value}
}

func rule(strictness string, conditions ...domain.RuleCondition) domain.Rule {
	return domain.Rule{Strictness: strictness, Conditions: conditions}
}

func TestMatchesCondition(t *testing.T) {
	// A -$45.00 coffee run on the checking account.
	coffee := txn("STARBUCKS STORE #4412", -4500, acctChecking)

	tests := []struct {
		name string
		cond domain.RuleCondition
		txn  domain.Transaction
		want bool
	}{
		// --- description ---
		{"contains matches", cond(FieldDescription, OpContains, "starbucks"), coffee, true},
		{"contains is case-insensitive both ways", cond(FieldDescription, OpContains, "StArBuCkS"), coffee, true},
		{"contains non-match", cond(FieldDescription, OpContains, "dunkin"), coffee, false},
		{"not_contains inverts", cond(FieldDescription, OpNotContains, "dunkin"), coffee, true},
		{"not_contains on a match", cond(FieldDescription, OpNotContains, "starbucks"), coffee, false},

		// is_exactly is the operator the UI emitted as "is", so it never fired.
		{"is_exactly matches whole string", cond(FieldDescription, OpIsExactly, "STARBUCKS STORE #4412"), coffee, true},
		{"is_exactly is case-insensitive", cond(FieldDescription, OpIsExactly, "starbucks store #4412"), coffee, true},
		{"is_exactly rejects a substring", cond(FieldDescription, OpIsExactly, "STARBUCKS"), coffee, false},

		{"starts_with matches", cond(FieldDescription, OpStartsWith, "STARBUCKS"), coffee, true},
		{"starts_with is case-insensitive", cond(FieldDescription, OpStartsWith, "starbucks"), coffee, true},
		{"starts_with non-match", cond(FieldDescription, OpStartsWith, "STORE"), coffee, false},
		{"ends_with matches", cond(FieldDescription, OpEndsWith, "#4412"), coffee, true},
		{"ends_with is case-insensitive", cond(FieldDescription, OpEndsWith, "store #4412"), coffee, true},
		{"ends_with non-match", cond(FieldDescription, OpEndsWith, "STARBUCKS"), coffee, false},

		// --- amount (magnitude) ---
		{"greater_than matches a large expense", cond(FieldAmount, OpGreaterThan, "40.00"), coffee, true},
		{"greater_than matches a large deposit too", cond(FieldAmount, OpGreaterThan, "40.00"), txn("PAYCHECK", 4500, acctChecking), true},
		{"greater_than non-match", cond(FieldAmount, OpGreaterThan, "50.00"), coffee, false},
		{"less_than matches", cond(FieldAmount, OpLessThan, "50.00"), coffee, true},
		{"less_than non-match", cond(FieldAmount, OpLessThan, "40.00"), coffee, false},
		{"is_exactly on amount", cond(FieldAmount, OpIsExactly, "45.00"), coffee, true},
		{"is_exactly on amount ignores sign", cond(FieldAmount, OpIsExactly, "-45.00"), coffee, true},
		{"is_exactly on amount non-match", cond(FieldAmount, OpIsExactly, "45.01"), coffee, false},
		{"bare integer parses as dollars", cond(FieldAmount, OpIsExactly, "45"), coffee, true},
		{"unparseable amount never matches", cond(FieldAmount, OpGreaterThan, "forty"), coffee, false},

		// --- direction ---
		{"outflow matches a negative amount", cond(FieldDirection, OpIsExactly, DirectionOutflow), coffee, true},
		{"inflow does not match a negative amount", cond(FieldDirection, OpIsExactly, DirectionInflow), coffee, false},
		{"inflow matches a positive amount", cond(FieldDirection, OpIsExactly, DirectionInflow), txn("PAYCHECK", 250000, acctChecking), true},
		{"direction value is case-insensitive", cond(FieldDirection, OpIsExactly, "OutFlow"), coffee, true},

		// --- account ---
		{"account matches account_id", cond(FieldAccount, OpIsExactly, acctChecking), coffee, true},
		{"account non-match", cond(FieldAccount, OpIsExactly, acctSavings), coffee, false},
		{"account not_equals", cond(FieldAccount, OpNotEquals, acctSavings), coffee, true},

		// --- regex ---
		{"regex anchored match", cond(FieldDescription, OpMatchesRegex, "^STARBUCKS"), coffee, true},
		{"regex is case-insensitive", cond(FieldDescription, OpMatchesRegex, "^starbucks"), coffee, true},
		{"regex character class", cond(FieldDescription, OpMatchesRegex, `#\d{4}$`), coffee, true},
		{"regex non-match", cond(FieldDescription, OpMatchesRegex, "^DUNKIN"), coffee, false},
		{"uncompilable regex never matches", cond(FieldDescription, OpMatchesRegex, "^(unclosed"), coffee, false},

		// --- vocabulary guards ---
		{"unknown field never matches", cond("merchant", OpContains, "STARBUCKS"), coffee, false},
		{"unknown operator never matches", cond(FieldDescription, "sounds_like", "STARBUCKS"), coffee, false},
		{"operator invalid for field never matches", cond(FieldAmount, OpContains, "45"), coffee, false},
		// The exact pair that was dead before: the UI's "is" against the engine.
		{"legacy 'is' operator never matches", cond(FieldDescription, "is", "STARBUCKS STORE #4412"), coffee, false},
		// The old backend field name is no longer honored.
		{"legacy 'source_account' field never matches", cond("source_account", OpIsExactly, acctChecking), coffee, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesCondition(tt.cond, tt.txn, NewRuleResolver())
			if got != tt.want {
				t.Errorf("matchesCondition(%+v) = %v, want %v", tt.cond, got, tt.want)
			}
		})
	}
}

func TestMatchesRuleStrictness(t *testing.T) {
	coffee := txn("STARBUCKS STORE #4412", -4500, acctChecking)

	matchingCond := cond(FieldDescription, OpContains, "starbucks")
	failingCond := cond(FieldDescription, OpContains, "dunkin")

	tests := []struct {
		name string
		rule domain.Rule
		want bool
	}{
		{"all: every condition true", rule(StrictnessAll, matchingCond, cond(FieldDirection, OpIsExactly, DirectionOutflow)), true},
		{"all: one condition false", rule(StrictnessAll, matchingCond, failingCond), false},
		{"any: one condition true", rule(StrictnessAny, failingCond, matchingCond), true},
		{"any: no condition true", rule(StrictnessAny, failingCond, cond(FieldAccount, OpIsExactly, acctSavings)), false},

		// An empty condition list previously made an "all" rule match every
		// transaction, which combined with set_category would recategorize the
		// whole ledger in one apply.
		{"all with no conditions matches nothing", rule(StrictnessAll), false},
		{"any with no conditions matches nothing", rule(StrictnessAny), false},

		// An unrecognized strictness falls back to the strict reading.
		{"unknown strictness is treated as all", rule("mostly", matchingCond, failingCond), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesRule(tt.rule, coffee, NewRuleResolver()); got != tt.want {
				t.Errorf("MatchesRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAmountWithDirection covers the combination the amount-magnitude change
// exists to enable: "expenses over $100" is two conditions, not a sign trick.
func TestAmountWithDirection(t *testing.T) {
	bigExpense := txn("LAPTOP", -150000, acctChecking)
	bigDeposit := txn("REFUND", 150000, acctChecking)

	r := rule(StrictnessAll,
		cond(FieldAmount, OpGreaterThan, "100.00"),
		cond(FieldDirection, OpIsExactly, DirectionOutflow),
	)

	if !MatchesRule(r, bigExpense, NewRuleResolver()) {
		t.Error("expected a -$1500.00 expense to match 'amount > 100 and outflow'")
	}
	if MatchesRule(r, bigDeposit, NewRuleResolver()) {
		t.Error("expected a +$1500.00 deposit NOT to match 'amount > 100 and outflow'")
	}

	// Without the direction condition, magnitude alone matches both.
	magnitudeOnly := rule(StrictnessAll, cond(FieldAmount, OpGreaterThan, "100.00"))
	if !MatchesRule(magnitudeOnly, bigDeposit, NewRuleResolver()) {
		t.Error("expected magnitude-only rule to match the deposit")
	}
}

// TestResolverCachesRegex checks that a pattern compiles once per run rather
// than once per transaction, and that a bad pattern is reported once.
func TestResolverCachesRegex(t *testing.T) {
	res := NewRuleResolver()
	c := cond(FieldDescription, OpMatchesRegex, "^AMZN")

	for i := 0; i < 5; i++ {
		matchesCondition(c, txn("AMZN MKTP US", -1000, acctChecking), res)
	}
	if len(res.regexCache) != 1 {
		t.Errorf("expected 1 cached pattern, got %d", len(res.regexCache))
	}

	bad := cond(FieldDescription, OpMatchesRegex, "^(unclosed")
	for i := 0; i < 3; i++ {
		if matchesCondition(bad, txn("anything", -1, acctChecking), res) {
			t.Fatal("an uncompilable pattern must never match")
		}
	}
	if got := res.BadPatterns(); len(got) != 1 || got[0] != "^(unclosed" {
		t.Errorf("BadPatterns() = %v, want exactly [^(unclosed]", got)
	}
}
