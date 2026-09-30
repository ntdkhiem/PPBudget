-- 0033_transaction_budget_month.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_transactions_user_effective_date;
ALTER TABLE transactions DROP COLUMN IF EXISTS effective_date;
ALTER TABLE transactions DROP COLUMN IF EXISTS budget_month;

COMMIT;
