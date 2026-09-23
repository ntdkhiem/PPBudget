-- 0029_wealth_goals_and_overrides.down.sql
--
-- Drops goals and the profile overrides.
--
-- Goals are the only user-authored content in the feature: named targets with
-- dates that nothing can re-derive from transactions. Rolling back loses any
-- created since the backfill read them out of the financial_plan blob, which
-- itself is left intact.

BEGIN;

DROP TABLE IF EXISTS wealth_goals;

ALTER TABLE wealth_profiles
    DROP COLUMN IF EXISTS strategy,
    DROP COLUMN IF EXISTS override_monthly_income,
    DROP COLUMN IF EXISTS override_essential_expenses,
    DROP COLUMN IF EXISTS override_liquid_account_ids;

COMMIT;
