-- 0033_transaction_budget_month.up.sql
--
-- The month a transaction counts toward, when it is not the month it posted.
--
-- Rent paid on 28 September for October posts in September, so September's
-- budget, the planning baseline and every monthly figure counted it there.
-- Moving the date fixed the budgets but made the ledger disagree with the
-- bank. budget_month moves only the counting: the bank date stays as posted,
-- and balances and net worth keep using it.
--
-- effective_date is what spending-by-period queries filter on: the first of
-- the chosen month when one is set, else the posted date. Stored and indexed
-- so those queries swap one column for another and stay fast.

BEGIN;

ALTER TABLE transactions
    ADD COLUMN budget_month DATE
        CHECK (budget_month IS NULL OR EXTRACT(DAY FROM budget_month) = 1),
    ADD COLUMN effective_date DATE
        GENERATED ALWAYS AS (COALESCE(budget_month, date)) STORED;

CREATE INDEX idx_transactions_user_effective_date ON transactions (user_id, effective_date);

COMMIT;
