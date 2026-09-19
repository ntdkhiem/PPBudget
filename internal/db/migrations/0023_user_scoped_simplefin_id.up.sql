-- 0023_user_scoped_simplefin_id.up.sql
--
-- Scopes SimpleFIN identifiers to their owning user.
--
-- 0001_init declared `simplefin_id TEXT UNIQUE` on both accounts and
-- transactions, and 0014 (multi-tenancy) never revisited it. A SimpleFIN id is
-- only unique within one SimpleFIN connection, so two PPBudget users linking
-- the same bank (a couple sharing an account, say) collide globally.
--
-- The account case is the damaging one. UpsertSimplefinAccount runs
-- `ON CONFLICT (simplefin_id) DO UPDATE ... RETURNING id` with no user filter,
-- so the second user's sync matches the *first* user's row, overwrites its
-- name, and returns that account's id -- after which the second user's
-- transactions are written against an account belonging to someone else.
-- For transactions, `ON CONFLICT DO NOTHING` silently drops the second user's
-- rows instead.
--
-- Both constraints become UNIQUE (user_id, simplefin_id). NULL simplefin_id
-- (manual accounts and transactions) stays unconstrained, since NULLs are
-- never equal in a unique index.

BEGIN;

-- Postgres names these from 0001_init's inline UNIQUE: <table>_<column>_key.
-- Dropped by name with IF EXISTS so a hand-patched database does not fail here.
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_simplefin_id_key;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_simplefin_id_key;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_user_simplefin_id_key UNIQUE (user_id, simplefin_id);

ALTER TABLE transactions
    ADD CONSTRAINT transactions_user_simplefin_id_key UNIQUE (user_id, simplefin_id);

COMMIT;
