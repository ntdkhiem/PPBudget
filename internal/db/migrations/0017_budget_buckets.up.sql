-- 0017_budget_buckets.up.sql
ALTER TABLE budgets 
ADD COLUMN bucket TEXT NOT NULL DEFAULT 'needs' 
CHECK (bucket IN ('needs', 'wants', 'savings'));
