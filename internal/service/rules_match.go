package service

import (
	"regexp"
	"strings"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// RuleResolver carries per-apply-run state shared across every transaction a
// rule is evaluated against. Today that is the compiled-regex cache, so a
// matches_regex pattern compiles once per run instead of once per transaction.
//
// The zero value is not usable; call NewRuleResolver. A resolver is not safe
// for concurrent use -- apply runs are sequential.
type RuleResolver struct {
	regexCache map[string]*regexp.Regexp
	// badPatterns records patterns that failed to compile, so the caller can
	// log them once per run rather than once per transaction.
	badPatterns map[string]bool
}

func NewRuleResolver() *RuleResolver {
	return &RuleResolver{
		regexCache:  make(map[string]*regexp.Regexp),
		badPatterns: make(map[string]bool),
	}
}

// BadPatterns returns the regex patterns that failed to compile during this
// run. ValidateRule rejects these at save time, so a non-empty result means a
// rule predating validation (or written straight to the database).
func (r *RuleResolver) BadPatterns() []string {
	out := make([]string, 0, len(r.badPatterns))
	for p := range r.badPatterns {
		out = append(out, p)
	}
	return out
}

// compile returns the cached compiled form of pattern, or nil if it does not
// compile. Patterns are matched case-insensitively, consistent with every other
// string operator.
func (r *RuleResolver) compile(pattern string) *regexp.Regexp {
	if re, ok := r.regexCache[pattern]; ok {
		return re
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		r.regexCache[pattern] = nil
		r.badPatterns[pattern] = true
		return nil
	}
	r.regexCache[pattern] = re
	return re
}

// MatchesRule reports whether t satisfies rule under the rule's strictness.
//
// A rule with no conditions matches nothing. That is deliberate: the previous
// implementation treated "all" with zero conditions as matching everything,
// which combined with a set_category action would recategorize an entire
// ledger in one apply.
func MatchesRule(rule domain.Rule, t domain.Transaction, res *RuleResolver) bool {
	if len(rule.Conditions) == 0 {
		return false
	}

	if rule.Strictness == StrictnessAny {
		for _, c := range rule.Conditions {
			if matchesCondition(c, t, res) {
				return true
			}
		}
		return false
	}

	// Default to "all" so an unset or unrecognized strictness is the strict
	// reading rather than the permissive one.
	for _, c := range rule.Conditions {
		if !matchesCondition(c, t, res) {
			return false
		}
	}
	return true
}

// matchesCondition evaluates a single condition. An unknown field or an
// operator invalid for its field never matches.
func matchesCondition(c domain.RuleCondition, t domain.Transaction, res *RuleResolver) bool {
	if !operatorValidForField(c.Field, c.Operator) {
		return false
	}

	switch c.Field {
	case FieldDescription:
		return matchesText(t.Description, c.Operator, c.Value, res)
	case FieldAmount:
		return matchesAmount(t.Amount, c.Operator, c.Value)
	case FieldDirection:
		return matchesDirection(t.Amount, c.Operator, c.Value)
	case FieldAccount:
		switch c.Operator {
		case OpIsExactly:
			return t.AccountID == c.Value
		case OpNotEquals:
			return t.AccountID != c.Value
		}
	}
	return false
}

// matchesText compares a transaction string field. Every operator here is
// case-insensitive; the old engine lowercased only "contains", which made
// "starts with" behave differently from "contains" for no stated reason.
func matchesText(actual, operator, want string, res *RuleResolver) bool {
	if operator == OpMatchesRegex {
		re := res.compile(want)
		if re == nil {
			return false
		}
		return re.MatchString(actual)
	}

	a := strings.ToLower(actual)
	w := strings.ToLower(want)

	switch operator {
	case OpContains:
		return strings.Contains(a, w)
	case OpNotContains:
		return !strings.Contains(a, w)
	case OpIsExactly:
		return a == w
	case OpStartsWith:
		return strings.HasPrefix(a, w)
	case OpEndsWith:
		return strings.HasSuffix(a, w)
	}
	return false
}

// matchesAmount compares on magnitude, so "greater than 100" reads as "bigger
// than $100" regardless of whether the transaction is an expense (stored
// negative) or income. Pair it with a direction condition to narrow to one.
func matchesAmount(amount money.Money, operator, want string) bool {
	condAmt, err := money.NewFromString(want)
	if err != nil {
		return false
	}

	a := abs64(amount.ToInt64())
	w := abs64(condAmt.ToInt64())

	switch operator {
	case OpGreaterThan:
		return a > w
	case OpLessThan:
		return a < w
	case OpIsExactly:
		return a == w
	}
	return false
}

// matchesDirection derives inflow/outflow from the sign of the amount.
// Zero-amount transactions count as inflow, matching the "not an outflow"
// reading; they are vanishingly rare and not worth a third direction value.
func matchesDirection(amount money.Money, operator, want string) bool {
	if operator != OpIsExactly {
		return false
	}
	direction := DirectionInflow
	if amount.ToInt64() < 0 {
		direction = DirectionOutflow
	}
	return direction == strings.ToLower(strings.TrimSpace(want))
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
