DROP TRIGGER IF EXISTS sync_opening_snapshot ON accounts;
DROP FUNCTION IF EXISTS sync_opening_snapshot_from_initial_balance();
DROP FUNCTION IF EXISTS account_balance_at(UUID, DATE);
DROP INDEX IF EXISTS idx_transactions_account_date_amount_live;
DROP TABLE IF EXISTS account_balance_snapshots;
