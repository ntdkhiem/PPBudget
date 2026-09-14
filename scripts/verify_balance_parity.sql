-- verify_balance_parity.sql
--
-- Compares the legacy balance formula
--     accounts.initial_balance + SUM(non-deleted transactions.amount)
-- against account_balance_at(id, 'infinity') for every account.
-- Prints only mismatching rows, then a final mismatch count (expect 0).
--
-- ONLY VALID BETWEEN MIGRATIONS 0019 AND 0020: it needs both the
-- account_balance_snapshots table / account_balance_at function (0019) and the
-- accounts.initial_balance column (dropped by 0020). Run it right after
-- applying 0019 and before any SimpleFin/manual snapshots are written (those
-- intentionally change balances).
--
-- Usage: psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/verify_balance_parity.sql
-- Read-only.

WITH legacy AS (
    SELECT a.id,
           a.user_id,
           a.name,
           a.type,
           a.initial_balance + COALESCE(SUM(t.amount), 0) AS old_balance
      FROM accounts a
      LEFT JOIN transactions t
             ON t.account_id = a.id
            AND t.deleted_at IS NULL
     GROUP BY a.id, a.user_id, a.name, a.type, a.initial_balance
),
cmp AS (
    SELECT l.*, account_balance_at(l.id, 'infinity'::date) AS new_balance
      FROM legacy l
)
SELECT id, user_id, name, type, old_balance, new_balance,
       new_balance - old_balance AS diff
  FROM cmp
 WHERE old_balance IS DISTINCT FROM new_balance
 ORDER BY user_id, name;

SELECT COUNT(*) AS mismatch_count
  FROM (
    SELECT a.id
      FROM accounts a
      LEFT JOIN transactions t
             ON t.account_id = a.id
            AND t.deleted_at IS NULL
     GROUP BY a.id, a.initial_balance
    HAVING a.initial_balance + COALESCE(SUM(t.amount), 0)
           IS DISTINCT FROM account_balance_at(a.id, 'infinity'::date)
  ) m;
