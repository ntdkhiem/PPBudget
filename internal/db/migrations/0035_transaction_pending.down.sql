-- 0035_transaction_pending.down.sql
--
-- Pending rows stay, as ordinary transactions.

BEGIN;

DROP INDEX IF EXISTS idx_transactions_pending;
ALTER TABLE transactions DROP COLUMN IF EXISTS is_pending;

COMMIT;
