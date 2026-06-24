CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE transactions ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
  to_tsvector('english', coalesce(description, '') || ' ' || coalesce(notes, ''))
) STORED;

CREATE INDEX idx_transactions_search ON transactions USING GIN (search_vector);

CREATE INDEX idx_categories_name_trgm ON categories USING GIN (name gin_trgm_ops);
CREATE INDEX idx_accounts_name_trgm ON accounts USING GIN (name gin_trgm_ops);
CREATE INDEX idx_subscriptions_name_trgm ON subscriptions USING GIN (name gin_trgm_ops);
