package service

import (
	"fmt"
	"regexp"
	"strings"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// DefaultTriggerType matches the rules.trigger_type column default. The column
// is NOT NULL with a default, but CreateRule inserts the field explicitly, so
// Go's zero value would land as an empty string unless normalized here.
const DefaultTriggerType = "store-journal"

// NormalizeRule fills in server-side defaults for fields the client does not
// send. Call it before ValidateRule on the write path.
func NormalizeRule(rule *domain.Rule) {
	rule.Name = strings.TrimSpace(rule.Name)
	if strings.TrimSpace(rule.TriggerType) == "" {
		rule.TriggerType = DefaultTriggerType
	}
	if strings.TrimSpace(rule.Strictness) == "" {
		rule.Strictness = StrictnessAll
	}
	for i := range rule.Conditions {
		if rule.Conditions[i].Field == FieldDirection {
			rule.Conditions[i].Value = strings.ToLower(strings.TrimSpace(rule.Conditions[i].Value))
		}
	}
}

// ValidateRule checks a rule against the canonical vocabulary before it is
// stored. Every returned error wraps apperrors.ErrInvalidInput so handlers can
// map it to 400 while still showing the specific reason.
//
// This exists because the engine silently ignores anything it does not
// recognize: an unknown operator produces a rule that saves, renders, and never
// fires. Rejecting at write time makes that failure loud and immediate.
func ValidateRule(rule *domain.Rule) error {
	if strings.TrimSpace(rule.Name) == "" {
		return fmt.Errorf("%w: rule name is required", apperrors.ErrInvalidInput)
	}

	if rule.Strictness != StrictnessAll && rule.Strictness != StrictnessAny {
		return fmt.Errorf("%w: strictness must be %q or %q", apperrors.ErrInvalidInput, StrictnessAll, StrictnessAny)
	}

	if len(rule.Conditions) == 0 {
		return fmt.Errorf("%w: a rule needs at least one condition", apperrors.ErrInvalidInput)
	}
	if len(rule.Actions) == 0 {
		return fmt.Errorf("%w: a rule needs at least one action", apperrors.ErrInvalidInput)
	}

	for i, c := range rule.Conditions {
		if err := validateCondition(c); err != nil {
			return fmt.Errorf("%w (condition %d)", err, i+1)
		}
	}

	for i, a := range rule.Actions {
		if err := validateAction(a); err != nil {
			return fmt.Errorf("%w (action %d)", err, i+1)
		}
	}

	return nil
}

func validateCondition(c domain.RuleCondition) error {
	if _, ok := fieldOperators[c.Field]; !ok {
		return fmt.Errorf("%w: unknown condition field %q", apperrors.ErrInvalidInput, c.Field)
	}
	if !operatorValidForField(c.Field, c.Operator) {
		return fmt.Errorf("%w: operator %q is not valid for field %q", apperrors.ErrInvalidInput, c.Operator, c.Field)
	}

	// A blank value makes "contains" match every transaction, which is
	// reachable from the UI just by leaving the box empty.
	if strings.TrimSpace(c.Value) == "" {
		return fmt.Errorf("%w: condition value is required", apperrors.ErrInvalidInput)
	}

	switch c.Field {
	case FieldAmount:
		if _, err := money.NewFromString(c.Value); err != nil {
			return fmt.Errorf("%w: amount %q is not a valid number", apperrors.ErrInvalidInput, c.Value)
		}
	case FieldDirection:
		v := strings.ToLower(strings.TrimSpace(c.Value))
		if v != DirectionInflow && v != DirectionOutflow {
			return fmt.Errorf("%w: direction must be %q or %q", apperrors.ErrInvalidInput, DirectionInflow, DirectionOutflow)
		}
	case FieldDescription:
		if c.Operator == OpMatchesRegex {
			if len(c.Value) > MaxRegexLength {
				return fmt.Errorf("%w: regex pattern must be %d characters or fewer", apperrors.ErrInvalidInput, MaxRegexLength)
			}
			if _, err := regexp.Compile("(?i)" + c.Value); err != nil {
				return fmt.Errorf("%w: invalid regex pattern: %v", apperrors.ErrInvalidInput, err)
			}
		}
	}

	return nil
}

func validateAction(a domain.RuleAction) error {
	if !validActionTypes[a.ActionType] {
		return fmt.Errorf("%w: unknown action type %q", apperrors.ErrInvalidInput, a.ActionType)
	}
	if strings.TrimSpace(a.Value) == "" {
		return fmt.Errorf("%w: action value is required", apperrors.ErrInvalidInput)
	}
	return nil
}
