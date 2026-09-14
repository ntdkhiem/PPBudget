ALTER TABLE accounts ADD COLUMN initial_balance BIGINT NOT NULL DEFAULT 0;

UPDATE accounts a
   SET initial_balance = s.balance
  FROM account_balance_snapshots s
 WHERE s.account_id = a.id
   AND s.as_of_date = '-infinity'::date;

-- Restore the 0019 transition trigger.
CREATE OR REPLACE FUNCTION sync_opening_snapshot_from_initial_balance()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
    VALUES (NEW.user_id, NEW.id, '-infinity'::date, NEW.initial_balance, 'opening')
    ON CONFLICT (account_id, as_of_date) DO UPDATE SET balance = EXCLUDED.balance;
    RETURN NEW;
END;
$$;

CREATE TRIGGER sync_opening_snapshot
    AFTER UPDATE OF initial_balance ON accounts
    FOR EACH ROW
    WHEN (OLD.initial_balance IS DISTINCT FROM NEW.initial_balance)
    EXECUTE PROCEDURE sync_opening_snapshot_from_initial_balance();
