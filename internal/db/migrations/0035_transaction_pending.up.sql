-- 0035_transaction_pending.up.sql
--
-- Whether an imported transaction is still pending at the bank.
--
-- With "Import pending" on, SimpleFIN returns transactions that haven't posted
-- yet. Their amount, date and description can still change, and the bank may
-- give the posted version a new id. A pending row is updated in place when it
-- posts under the same id, and removed by the next sync that no longer
-- reports it. Posted rows are never rewritten by an import.

BEGIN;

ALTER TABLE transactions ADD COLUMN is_pending BOOLEAN NOT NULL DEFAULT false;

-- Each sync looks up an account's pending rows; there are only ever a few.
CREATE INDEX idx_transactions_pending ON transactions (account_id) WHERE is_pending;

COMMIT;
