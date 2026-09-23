-- 0024_wealth_profile.down.sql
--
-- Drops the Wealth Strategy profile tables.
--
-- This is destructive and unrecoverable for anything the user typed. The
-- answers here -- match mechanics, equity terms, insurance coverage, prior-year
-- tax figures -- cannot be re-derived from transactions the way spending can.
--
-- The 'financial_plan' and 'retirement_plan' user_settings blobs are left
-- untouched by 0024 (the backfill reads them, never deletes them) precisely so
-- this rollback lands somewhere usable rather than empty. Answers given after
-- the backfill are lost here with no recovery path.

BEGIN;

DROP TABLE IF EXISTS planned_expenses;
DROP TABLE IF EXISTS dependents;
DROP TABLE IF EXISTS retirement_account_terms;
DROP TABLE IF EXISTS account_terms;
DROP TABLE IF EXISTS wealth_profile_fields;
DROP TABLE IF EXISTS wealth_profiles;

COMMIT;
