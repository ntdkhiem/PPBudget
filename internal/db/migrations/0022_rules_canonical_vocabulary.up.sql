-- 0022_rules_canonical_vocabulary.up.sql
--
-- Repairs rules stored with vocabulary the engine never checked.
--
-- The rules UI emitted operator 'is' and field 'account' while the engine
-- matched on 'is_exactly' and 'source_account'. Those rules saved cleanly,
-- rendered correctly on their cards, and silently never fired. This migration
-- rewrites the stored rows onto the canonical vocabulary defined in
-- internal/service/rules_vocabulary.go.
--
-- Deliberately non-destructive: rows for the three action types that were never
-- implemented (add_tag, set_budget, link_as_card_payment) are left in place so
-- they stay visible in the UI rather than disappearing without a trace.

BEGIN;

-- 1. Operator: the UI's 'is' becomes the engine's 'is_exactly'.
UPDATE rule_conditions SET operator = 'is_exactly' WHERE operator = 'is';

-- 2. Field: the engine's old 'source_account' becomes the canonical 'account'.
UPDATE rule_conditions SET field = 'account' WHERE field = 'source_account';

-- 3. Account conditions now hold an accounts.id. Resolve the old values --
--    which were either a SimpleFIN id (the engine's old semantics) or a name
--    typed into the free-text box (what the UI actually offered) -- to real
--    account ids, scoped through the owning rule's user.
--
--    Matching on simplefin_id first: it is an exact identifier, whereas the
--    name match is case-insensitive and could in principle hit two accounts.
UPDATE rule_conditions c
SET value = a.id::text
FROM rules r, accounts a
WHERE c.rule_id = r.id
  AND c.field = 'account'
  AND a.user_id = r.user_id
  AND a.simplefin_id = c.value;

UPDATE rule_conditions c
SET value = a.id::text
FROM rules r, accounts a
WHERE c.rule_id = r.id
  AND c.field = 'account'
  AND a.user_id = r.user_id
  AND lower(a.name) = lower(c.value)
  -- Skip values already rewritten to a uuid by the statement above, and any
  -- name that is ambiguous within the user's accounts.
  AND c.value !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  AND (SELECT count(*) FROM accounts a2 WHERE a2.user_id = r.user_id AND lower(a2.name) = lower(c.value)) = 1;

-- 4. The UI omits trigger_type, so Go's zero value was inserted explicitly over
--    the column default.
UPDATE rules SET trigger_type = 'store-journal' WHERE trigger_type = '';

-- 5. Foreign keys on these tables were never indexed.
CREATE INDEX IF NOT EXISTS idx_rule_conditions_rule_id ON rule_conditions(rule_id);
CREATE INDEX IF NOT EXISTS idx_rule_actions_rule_id ON rule_actions(rule_id);

COMMIT;
