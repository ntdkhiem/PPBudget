-- 0034_unique_budget_per_month.down.sql
--
-- Drops the key only. Rows folded or deleted on the way up are not restored.

BEGIN;

ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_user_category_month_key;

COMMIT;
