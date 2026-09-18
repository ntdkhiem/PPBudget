package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

func validRule() domain.Rule {
	return domain.Rule{
		Name:       "Coffee",
		Strictness: StrictnessAll,
		Conditions: []domain.RuleCondition{cond(FieldDescription, OpContains, "starbucks")},
		Actions:    []domain.RuleAction{{ActionType: ActionSetCategory, Value: "cat-1"}},
	}
}

func TestValidateRule(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *domain.Rule)
		wantErr bool
	}{
		{"a well-formed rule", func(r *domain.Rule) {}, false},
		{"strictness any is valid", func(r *domain.Rule) { r.Strictness = StrictnessAny }, false},

		{"blank name", func(r *domain.Rule) { r.Name = "   " }, true},
		{"unknown strictness", func(r *domain.Rule) { r.Strictness = "mostly" }, true},
		{"no conditions", func(r *domain.Rule) { r.Conditions = nil }, true},
		{"no actions", func(r *domain.Rule) { r.Actions = nil }, true},

		{"unknown field", func(r *domain.Rule) { r.Conditions[0].Field = "merchant" }, true},
		{"unknown operator", func(r *domain.Rule) { r.Conditions[0].Operator = "sounds_like" }, true},
		{"operator wrong for field", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldAmount, OpContains, "45")
		}, true},
		// The legacy UI vocabulary must now be rejected outright rather than
		// saving into a rule that silently never fires.
		{"legacy 'is' operator", func(r *domain.Rule) { r.Conditions[0].Operator = "is" }, true},
		{"legacy 'source_account' field", func(r *domain.Rule) { r.Conditions[0].Field = "source_account" }, true},

		// A blank value makes "contains" match every transaction.
		{"blank condition value", func(r *domain.Rule) { r.Conditions[0].Value = "" }, true},
		{"whitespace condition value", func(r *domain.Rule) { r.Conditions[0].Value = "   " }, true},

		{"non-numeric amount", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldAmount, OpGreaterThan, "forty")
		}, true},
		{"numeric amount", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldAmount, OpGreaterThan, "40.00")
		}, false},

		{"bad direction value", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldDirection, OpIsExactly, "sideways")
		}, true},
		{"good direction value", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldDirection, OpIsExactly, DirectionOutflow)
		}, false},

		{"uncompilable regex", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldDescription, OpMatchesRegex, "^(unclosed")
		}, true},
		{"valid regex", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldDescription, OpMatchesRegex, `^AMZN.*\d+$`)
		}, false},
		{"over-long regex", func(r *domain.Rule) {
			r.Conditions[0] = cond(FieldDescription, OpMatchesRegex, strings.Repeat("a", MaxRegexLength+1))
		}, true},

		{"unknown action type", func(r *domain.Rule) { r.Actions[0].ActionType = "add_tag" }, true},
		{"legacy set_budget action", func(r *domain.Rule) { r.Actions[0].ActionType = "set_budget" }, true},
		{"legacy link_as_card_payment action", func(r *domain.Rule) {
			r.Actions[0].ActionType = "link_as_card_payment"
		}, true},
		{"blank action value", func(r *domain.Rule) { r.Actions[0].Value = "" }, true},
		{"set_account is supported", func(r *domain.Rule) {
			r.Actions[0] = domain.RuleAction{ActionType: ActionSetAccount, Value: acctSavings}
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRule()
			tt.mutate(&r)

			err := ValidateRule(&r)
			if tt.wantErr && err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if err != nil && !errors.Is(err, apperrors.ErrInvalidInput) {
				t.Errorf("error must wrap ErrInvalidInput so handlers return 400, got %v", err)
			}
		})
	}
}

// TestVocabularyMatchesFrontend is the guard against the drift this whole
// change exists to fix: the frontend must emit exactly the strings the engine
// accepts. It reads the shared vocabulary module and the rules page, and
// asserts every vocabulary constant appears, and that no retired value has
// crept back in.
//
// A deliberate rename means updating both sides -- which is the point.
func TestVocabularyMatchesFrontend(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "frontend", "lib", "rules-vocabulary.ts"),
		filepath.Join("..", "..", "frontend", "app", "(dashboard)", "settings", "rules", "page.tsx"),
	}

	var sb strings.Builder
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("frontend source not readable at %s: %v", path, err)
		}
		sb.Write(raw)
		sb.WriteByte('\n')
	}
	page := sb.String()

	mustAppear := []string{
		FieldDescription, FieldAmount, FieldDirection, FieldAccount,
		OpContains, OpNotContains, OpIsExactly, OpStartsWith, OpEndsWith,
		OpMatchesRegex, OpGreaterThan, OpLessThan, OpNotEquals,
		ActionSetCategory, ActionSetAccount, ActionLinkToSubscription,
		DirectionInflow, DirectionOutflow,
	}
	for _, v := range mustAppear {
		// Accept a quoted string ("contains") or a bare TS object key
		// (contains:), which is how the label maps are written.
		if !strings.Contains(page, `"`+v+`"`) && !strings.Contains(page, v+":") {
			t.Errorf("frontend does not offer %q; the engine accepts it but users cannot select it", v)
		}
	}

	// Values the engine no longer honors. Quoted to avoid matching substrings
	// of the canonical names (e.g. "is" inside "is_exactly").
	mustNotAppear := []string{
		"source_account", "add_tag", "set_budget", "link_as_card_payment",
	}
	for _, v := range mustNotAppear {
		if strings.Contains(page, `"`+v+`"`) {
			t.Errorf("frontend still offers %q, which the engine ignores -- rules using it would silently never fire", v)
		}
	}

	// The legacy "is" operator, as a complete quoted select value.
	if strings.Contains(page, `value="is"`) || strings.Contains(page, `"operator": "is"`) {
		t.Error(`frontend still emits the legacy "is" operator; the engine expects "is_exactly"`)
	}
}
