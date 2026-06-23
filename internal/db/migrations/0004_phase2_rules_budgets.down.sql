-- 0004_phase2_rules_budgets.down.sql
DROP TABLE IF EXISTS rule_actions CASCADE;
DROP TABLE IF EXISTS rule_conditions CASCADE;
DROP TABLE IF EXISTS rules CASCADE;
DROP TABLE IF EXISTS budgets CASCADE;

CREATE TABLE rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    match_description TEXT NOT NULL,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TRIGGER update_rule_modtime BEFORE UPDATE ON rules FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

CREATE TABLE budgets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    month DATE NOT NULL,
    limit_cents BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(category_id, month)
);
CREATE TRIGGER update_budget_modtime BEFORE UPDATE ON budgets FOR EACH ROW EXECUTE PROCEDURE update_modified_column();
