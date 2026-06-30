ALTER TABLE transactions ADD COLUMN linked_transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL;

-- Restore data (best effort, take the first source if multiple exist)
UPDATE transactions t
SET linked_transaction_id = sub.source_transaction_id
FROM (
    SELECT target_transaction_id, MIN(source_transaction_id) as source_transaction_id
    FROM transaction_links
    GROUP BY target_transaction_id
) sub
WHERE t.id = sub.target_transaction_id;

DROP TABLE transaction_links;
