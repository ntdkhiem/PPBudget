-- Apply only after the snapshot-aware backend is deployed.

-- 1. Opening snapshots for accounts created by old code after 0019 ran.
INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
SELECT a.user_id, a.id, '-infinity'::date, a.initial_balance, 'opening'
  FROM accounts a
 WHERE NOT EXISTS (
       SELECT 1 FROM account_balance_snapshots s
        WHERE s.account_id = a.id
          AND s.as_of_date = '-infinity'::date
 );

-- 2. The transition trigger from 0019 is no longer needed.
DROP TRIGGER IF EXISTS sync_opening_snapshot ON accounts;
DROP FUNCTION IF EXISTS sync_opening_snapshot_from_initial_balance();

-- 3. Opening snapshots are now the single source of truth.
ALTER TABLE accounts DROP COLUMN initial_balance;
