-- 0034_unique_budget_per_month.up.sql
--
-- One budget per category per month.
--
-- Viewing a month with no budgets copies the previous month's forward
-- (rolloverBudgets). Nothing stopped two requests that arrived together, such
-- as the dashboard and the budgets page on the first visit of a month, from
-- both copying, which left two identical rows per category. The budgets page
-- showed one of them; every total that sums the month counted both.
--
-- Existing duplicates are folded before the key goes on:
--   1. Exact copies (same amount, period type and bucket) are what the race
--      left behind. One is kept, the others are deleted.
--   2. Any remaining pairs differ, e.g. a monthly budget and a one-time one
--      from a category merge. Their amounts are added onto one row (monthly
--      first, then most recently updated) and the rest are deleted, which
--      keeps the month's total what it was.

BEGIN;

DELETE FROM budgets b
USING (
    SELECT id,
           row_number() OVER (
               PARTITION BY user_id, category_id, start_date, amount, period_type, bucket
               ORDER BY updated_at DESC NULLS LAST, id
           ) AS rn
    FROM budgets
) d
WHERE b.id = d.id AND d.rn > 1;

WITH ranked AS (
    SELECT id, user_id, category_id, start_date, amount,
           row_number() OVER (
               PARTITION BY user_id, category_id, start_date
               ORDER BY (period_type = 'one_time'), updated_at DESC NULLS LAST, id
           ) AS rn
    FROM budgets
),
folded AS (
    SELECT user_id, category_id, start_date, SUM(amount) AS amount
    FROM ranked
    GROUP BY user_id, category_id, start_date
    HAVING COUNT(*) > 1
),
kept AS (
    UPDATE budgets b SET amount = f.amount, updated_at = NOW()
    FROM ranked r
    JOIN folded f USING (user_id, category_id, start_date)
    WHERE b.id = r.id AND r.rn = 1
    RETURNING b.id
)
DELETE FROM budgets b
USING ranked r
JOIN folded f USING (user_id, category_id, start_date)
WHERE b.id = r.id AND r.rn > 1;

ALTER TABLE budgets
    ADD CONSTRAINT budgets_user_category_month_key UNIQUE (user_id, category_id, start_date);

COMMIT;
