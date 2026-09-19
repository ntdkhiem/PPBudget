-- 0023_user_scoped_simplefin_id.down.sql
--
-- Restores the global UNIQUE constraints from 0001_init.
--
-- This can fail, and that failure is correct: if two users have each linked the
-- same SimpleFIN account while 0023 was applied, the rows that coexisted
-- legitimately under UNIQUE (user_id, simplefin_id) violate a global UNIQUE.
-- Rolling back would have to delete one user's data to succeed, so it stops
-- instead and leaves the decision to a human.
--
-- Recovering from that failure (verified against a throwaway database):
--   1. golang-migrate marks the version dirty; `migrate ... version` shows "22 (dirty)".
--   2. Decide which user keeps the shared SimpleFIN link and remove the other
--      side's accounts/transactions rows -- find them with:
--        SELECT user_id, simplefin_id, count(*) FROM accounts
--        WHERE simplefin_id IS NOT NULL GROUP BY 1,2 HAVING count(*) > 1;
--   3. `migrate ... force 23`, then re-run `migrate ... down 1`.

BEGIN;

ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_user_simplefin_id_key;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_user_simplefin_id_key;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_simplefin_id_key UNIQUE (simplefin_id);

ALTER TABLE transactions
    ADD CONSTRAINT transactions_simplefin_id_key UNIQUE (simplefin_id);

COMMIT;
