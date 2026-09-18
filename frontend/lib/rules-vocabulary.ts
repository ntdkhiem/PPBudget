import type { RuleField, RuleOperator, RuleActionType, RuleCondition, RuleAction } from "@/lib/api";

// Mirror of internal/service/rules_vocabulary.go. The Go test
// TestVocabularyMatchesFrontend reads the rules page and fails if the two sides
// drift -- which is how "Is exactly" and every Account rule silently stopped
// working before this file existed.

export const FIELD_LABELS: Record<RuleField, string> = {
  description: "Description",
  amount: "Amount",
  direction: "Direction",
  account: "Account",
};

export const OPERATOR_LABELS: Record<RuleOperator, string> = {
  contains: "Contains",
  not_contains: "Does not contain",
  is_exactly: "Is exactly",
  starts_with: "Starts with",
  ends_with: "Ends with",
  matches_regex: "Matches regex",
  greater_than: "Greater than",
  less_than: "Less than",
  not_equals: "Is not",
};

/** Operators valid for each field. The first entry is the default on field change. */
export const FIELD_OPERATORS: Record<RuleField, RuleOperator[]> = {
  description: ["contains", "not_contains", "is_exactly", "starts_with", "ends_with", "matches_regex"],
  amount: ["greater_than", "less_than", "is_exactly"],
  direction: ["is_exactly"],
  account: ["is_exactly", "not_equals"],
};

export const ACTION_LABELS: Record<RuleActionType, string> = {
  set_category: "Set Category",
  set_account: "Move to Account",
  link_to_subscription: "Link to Subscription",
};

/** Short labels for the compact badges on rule cards. */
export const ACTION_SHORT_LABELS: Record<RuleActionType, string> = {
  set_category: "Category",
  set_account: "Account",
  link_to_subscription: "Subscription",
};

export const DIRECTION_LABELS = {
  inflow: "Inflow (money in)",
  outflow: "Outflow (money out)",
} as const;

export const MAX_REGEX_LENGTH = 200;

export const RULE_FIELDS = Object.keys(FIELD_LABELS) as RuleField[];
export const RULE_ACTION_TYPES = Object.keys(ACTION_LABELS) as RuleActionType[];

export function isSupportedActionType(t: string): t is RuleActionType {
  return t in ACTION_LABELS;
}

export function operatorValidForField(field: RuleField, operator: RuleOperator): boolean {
  return FIELD_OPERATORS[field]?.includes(operator) ?? false;
}

/** Fields whose value is a UUID chosen from a dropdown rather than typed. */
export function valueIsEntityId(field: RuleField): boolean {
  return field === "account";
}

/**
 * Mirrors ValidateRule in internal/service/rules_validate.go so the Save button
 * can explain itself instead of round-tripping a 400. The server remains
 * authoritative -- notably for regex, where RE2 and JavaScript disagree on
 * some syntax.
 */
export function validateRuleDraft(
  name: string,
  conditions: Partial<RuleCondition>[],
  actions: Partial<RuleAction>[]
): string | null {
  if (!name.trim()) return "Give the rule a name.";
  if (conditions.length === 0) return "Add at least one condition.";
  if (actions.length === 0) return "Add at least one action.";

  for (const [i, c] of conditions.entries()) {
    const where = `Condition ${i + 1}`;
    if (!c.field || !c.operator) return `${where}: pick a field and an operator.`;
    if (!operatorValidForField(c.field, c.operator)) {
      return `${where}: "${OPERATOR_LABELS[c.operator]}" cannot be used with ${FIELD_LABELS[c.field]}.`;
    }
    if (!c.value?.trim()) return `${where}: enter a value.`;

    if (c.field === "amount" && Number.isNaN(Number(c.value))) {
      return `${where}: "${c.value}" is not a number.`;
    }
    if (c.field === "direction" && c.value !== "inflow" && c.value !== "outflow") {
      return `${where}: pick inflow or outflow.`;
    }
    if (c.operator === "matches_regex") {
      if (c.value.length > MAX_REGEX_LENGTH) {
        return `${where}: pattern must be ${MAX_REGEX_LENGTH} characters or fewer.`;
      }
      try {
        new RegExp(c.value);
      } catch {
        return `${where}: that is not a valid regular expression.`;
      }
    }
  }

  for (const [i, a] of actions.entries()) {
    const where = `Action ${i + 1}`;
    if (!a.action_type) return `${where}: pick an action.`;
    if (!a.value?.trim()) return `${where}: choose a value.`;
  }

  return null;
}
