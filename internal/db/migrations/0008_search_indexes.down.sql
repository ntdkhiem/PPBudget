DROP INDEX IF EXISTS idx_subscriptions_name_trgm;
DROP INDEX IF EXISTS idx_accounts_name_trgm;
DROP INDEX IF EXISTS idx_categories_name_trgm;
DROP INDEX IF EXISTS idx_transactions_search;
ALTER TABLE transactions DROP COLUMN IF EXISTS search_vector;
DROP EXTENSION IF EXISTS pg_trgm;
