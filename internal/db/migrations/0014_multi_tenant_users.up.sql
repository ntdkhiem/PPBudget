-- 1. Create the users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TRIGGER update_user_modtime BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- 2. Add user_id to Core Tables
ALTER TABLE accounts ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE categories ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE rules ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE budgets ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE subscriptions ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE transactions ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- 3. Refactor app_settings to user_settings
ALTER TABLE app_settings RENAME TO user_settings;
ALTER TABLE user_settings ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Assign default user for any existing rows before enforcing NOT NULL constraints
DO $$
DECLARE
    default_user_id UUID;
BEGIN
    IF EXISTS (SELECT 1 FROM accounts WHERE user_id IS NULL)
       OR EXISTS (SELECT 1 FROM categories WHERE user_id IS NULL)
       OR EXISTS (SELECT 1 FROM rules WHERE user_id IS NULL)
       OR EXISTS (SELECT 1 FROM budgets WHERE user_id IS NULL)
       OR EXISTS (SELECT 1 FROM subscriptions WHERE user_id IS NULL)
       OR EXISTS (SELECT 1 FROM transactions WHERE user_id IS NULL)
       OR EXISTS (SELECT 1 FROM user_settings WHERE user_id IS NULL) THEN

        INSERT INTO users (id, email, password_hash)
        VALUES ('00000000-0000-0000-0000-000000000001', 'default@example.com', '')
        ON CONFLICT (email) DO NOTHING;

        SELECT id INTO default_user_id FROM users WHERE email = 'default@example.com';

        UPDATE accounts SET user_id = default_user_id WHERE user_id IS NULL;
        UPDATE categories SET user_id = default_user_id WHERE user_id IS NULL;
        UPDATE rules SET user_id = default_user_id WHERE user_id IS NULL;
        UPDATE budgets SET user_id = default_user_id WHERE user_id IS NULL;
        UPDATE subscriptions SET user_id = default_user_id WHERE user_id IS NULL;
        UPDATE transactions SET user_id = default_user_id WHERE user_id IS NULL;
        UPDATE user_settings SET user_id = default_user_id WHERE user_id IS NULL;
    END IF;
END $$;

-- Update constraints to be NOT NULL after assigning existing rows
ALTER TABLE accounts ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE categories ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE rules ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE budgets ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE subscriptions ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE transactions ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_settings ALTER COLUMN user_id SET NOT NULL;

-- Update the primary key to be a composite of (user_id, key)
ALTER TABLE user_settings DROP CONSTRAINT IF EXISTS app_settings_pkey;
ALTER TABLE user_settings DROP CONSTRAINT IF EXISTS user_settings_pkey;
ALTER TABLE user_settings ADD PRIMARY KEY (user_id, key);

-- Indexes for foreign keys
CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_categories_user_id ON categories(user_id);
CREATE INDEX IF NOT EXISTS idx_rules_user_id ON rules(user_id);
CREATE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
