-- 1. Drop indexes
DROP INDEX IF EXISTS idx_transactions_user_id;
DROP INDEX IF EXISTS idx_subscriptions_user_id;
DROP INDEX IF EXISTS idx_budgets_user_id;
DROP INDEX IF EXISTS idx_rules_user_id;
DROP INDEX IF EXISTS idx_categories_user_id;
DROP INDEX IF EXISTS idx_accounts_user_id;

-- 2. Revert user_settings back to app_settings
ALTER TABLE user_settings DROP CONSTRAINT IF EXISTS user_settings_pkey;
ALTER TABLE user_settings DROP COLUMN IF EXISTS user_id;
ALTER TABLE user_settings ADD PRIMARY KEY (key);
ALTER TABLE user_settings RENAME TO app_settings;

-- 3. Remove user_id from core tables
ALTER TABLE transactions DROP COLUMN IF EXISTS user_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS user_id;
ALTER TABLE budgets DROP COLUMN IF EXISTS user_id;
ALTER TABLE rules DROP COLUMN IF EXISTS user_id;
ALTER TABLE categories DROP COLUMN IF EXISTS user_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS user_id;

-- 4. Drop trigger and users table
DROP TRIGGER IF EXISTS update_user_modtime ON users;
DROP TABLE IF EXISTS users CASCADE;
