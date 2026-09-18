package service

// Canonical rules-engine vocabulary.
//
// These constants are the single source of truth for the strings stored in
// rule_conditions.field / .operator and rule_actions.action_type. The rules UI
// (frontend/app/(dashboard)/settings/rules/page.tsx) mirrors them exactly --
// see TestVocabularyMatchesFrontend, which fails if the two drift apart.
//
// They drifted once already: the UI emitted "is" and "account" while the engine
// checked "is_exactly" and "source_account", so those rules saved cleanly,
// rendered correctly, and silently never fired. Migration 0022 repaired the
// stored rows; ValidateRule keeps new ones honest.

// Condition fields.
const (
	FieldDescription = "description"
	FieldAmount      = "amount"
	FieldDirection   = "direction"
	FieldAccount     = "account"
)

// Condition operators.
const (
	OpContains     = "contains"
	OpNotContains  = "not_contains"
	OpIsExactly    = "is_exactly"
	OpStartsWith   = "starts_with"
	OpEndsWith     = "ends_with"
	OpMatchesRegex = "matches_regex"
	OpGreaterThan  = "greater_than"
	OpLessThan     = "less_than"
	OpNotEquals    = "not_equals"
)

// Action types.
const (
	ActionSetCategory        = "set_category"
	ActionSetAccount         = "set_account"
	ActionLinkToSubscription = "link_to_subscription"
)

// Direction values, for FieldDirection conditions.
const (
	DirectionInflow  = "inflow"
	DirectionOutflow = "outflow"
)

// Strictness values.
const (
	StrictnessAll = "all"
	StrictnessAny = "any"
)

// MaxRegexLength caps matches_regex patterns. Go's regexp is RE2 (linear time,
// no catastrophic backtracking), so the cap is about keeping rules readable and
// bounding compile cost, not about protecting against malicious patterns.
const MaxRegexLength = 200

// fieldOperators lists the operators valid for each field. A field/operator
// pair absent from this map is rejected by ValidateRule and never matches.
var fieldOperators = map[string][]string{
	FieldDescription: {OpContains, OpNotContains, OpIsExactly, OpStartsWith, OpEndsWith, OpMatchesRegex},
	FieldAmount:      {OpGreaterThan, OpLessThan, OpIsExactly},
	FieldDirection:   {OpIsExactly},
	FieldAccount:     {OpIsExactly, OpNotEquals},
}

var validActionTypes = map[string]bool{
	ActionSetCategory:        true,
	ActionSetAccount:         true,
	ActionLinkToSubscription: true,
}

// operatorValidForField reports whether operator may be used with field.
func operatorValidForField(field, operator string) bool {
	ops, ok := fieldOperators[field]
	if !ok {
		return false
	}
	for _, op := range ops {
		if op == operator {
			return true
		}
	}
	return false
}
