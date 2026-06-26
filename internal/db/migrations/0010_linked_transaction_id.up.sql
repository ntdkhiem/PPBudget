ALTER TABLE transactions ADD COLUMN linked_transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL;
