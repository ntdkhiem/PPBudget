-- Balance snapshots: anchored account balances (opening / SimpleFin / manual).
-- An account's balance at a date is derived from the nearest snapshot plus the
-- transactions between that snapshot and the date (see account_balance_at).
-- Additive migration: accounts.initial_balance is kept until 0020.

CREATE TABLE account_balance_snapshots (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id        UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    as_of_date        DATE NOT NULL,        -- end-of-day balance, UTC; '-infinity' = opening balance
    balance           BIGINT NOT NULL,      -- cents, signed (liabilities negative)
    available_balance BIGINT,               -- cents, nullable (SimpleFin available-balance)
    source            TEXT NOT NULL CHECK (source IN ('opening','simplefin','manual')),
    reported_at       TIMESTAMPTZ,          -- SimpleFin balance-date, or NOW() for manual
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, as_of_date),
    CHECK ((source = 'opening') = (as_of_date = '-infinity'::date))
);

CREATE INDEX idx_balance_snapshots_account_date ON account_balance_snapshots (account_id, as_of_date DESC);
CREATE INDEX idx_balance_snapshots_user ON account_balance_snapshots (user_id);

CREATE TRIGGER update_balance_snapshot_modtime BEFORE UPDATE ON account_balance_snapshots
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- Index-only scans for the range sums in account_balance_at.
CREATE INDEX idx_transactions_account_date_amount_live
    ON transactions (account_id, date) INCLUDE (amount) WHERE deleted_at IS NULL;

-- Seed: every existing account gets an opening snapshot equal to initial_balance.
INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
SELECT user_id, id, '-infinity'::date, initial_balance, 'opening' FROM accounts;

-- Until 0020 drops initial_balance, the previous backend may still edit it.
-- Mirror those edits into the opening snapshot so they aren't lost.
-- Inserts are not mirrored: 0020 re-seeds missing openings, and the new
-- backend inserts accounts without initial_balance.
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

-- Balance of an account at end of day p_as_of (inclusive). Pass 'infinity' for
-- the current balance (includes future-dated transactions).
--   1. Latest snapshot on/before p_as_of + non-deleted txns in (snapshot, p_as_of].
--   2. Else earliest (future) snapshot - non-deleted txns in (p_as_of, snapshot].
--   3. Else sum of non-deleted txns dated <= p_as_of (0 if none).
-- Lookups use the (account_id, as_of_date) snapshot index and the
-- (account_id, date, id) transactions index.
CREATE OR REPLACE FUNCTION account_balance_at(p_account_id UUID, p_as_of DATE)
RETURNS BIGINT
LANGUAGE plpgsql
STABLE
STRICT
AS $$
DECLARE
    v_anchor_date DATE;
    v_anchor_bal  BIGINT;
BEGIN
    -- 1. Nearest anchor on or before p_as_of.
    SELECT s.as_of_date, s.balance
      INTO v_anchor_date, v_anchor_bal
      FROM account_balance_snapshots s
     WHERE s.account_id = p_account_id
       AND s.as_of_date <= p_as_of
     ORDER BY s.as_of_date DESC
     LIMIT 1;

    IF FOUND THEN
        RETURN v_anchor_bal + COALESCE((
            SELECT SUM(t.amount)
              FROM transactions t
             WHERE t.account_id = p_account_id
               AND t.deleted_at IS NULL
               AND t.date >  v_anchor_date
               AND t.date <= p_as_of
        ), 0)::BIGINT;
    END IF;

    -- 2. Only future anchors exist: walk back from the earliest one.
    SELECT s.as_of_date, s.balance
      INTO v_anchor_date, v_anchor_bal
      FROM account_balance_snapshots s
     WHERE s.account_id = p_account_id
     ORDER BY s.as_of_date ASC
     LIMIT 1;

    IF FOUND THEN
        RETURN v_anchor_bal - COALESCE((
            SELECT SUM(t.amount)
              FROM transactions t
             WHERE t.account_id = p_account_id
               AND t.deleted_at IS NULL
               AND t.date >  p_as_of
               AND t.date <= v_anchor_date
        ), 0)::BIGINT;
    END IF;

    -- 3. No snapshots at all.
    RETURN COALESCE((
        SELECT SUM(t.amount)
          FROM transactions t
         WHERE t.account_id = p_account_id
           AND t.deleted_at IS NULL
           AND t.date <= p_as_of
    ), 0)::BIGINT;
END;
$$;
