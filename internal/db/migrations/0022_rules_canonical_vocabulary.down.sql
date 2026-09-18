-- 0022_rules_canonical_vocabulary.down.sql
--
-- Reverses the vocabulary rename and drops the new indexes.
--
-- The account-value resolution in step 3 of the up migration is NOT reversed:
-- the old values were free text that never matched anything, and the original
-- strings are not recoverable from the account ids they were rewritten to.
-- Rolling back leaves account conditions holding ids, which the old engine
-- compared against simplefin_id -- exactly as broken as before, which is the
-- honest outcome.

BEGIN;

DROP INDEX IF EXISTS idx_rule_actions_rule_id;
DROP INDEX IF EXISTS idx_rule_conditions_rule_id;

UPDATE rule_conditions SET field = 'source_account' WHERE field = 'account';
UPDATE rule_conditions SET operator = 'is' WHERE operator = 'is_exactly';

COMMIT;
