CREATE TABLE transaction_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    target_transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(source_transaction_id, target_transaction_id)
);

-- Migrate existing 1-to-many links to the new table
INSERT INTO transaction_links (source_transaction_id, target_transaction_id, amount)
SELECT 
    linked_transaction_id, 
    id, 
    ABS(amount)
FROM transactions 
WHERE linked_transaction_id IS NOT NULL;

-- Drop the old column
ALTER TABLE transactions DROP COLUMN linked_transaction_id;
