CREATE INDEX idx_transaction_links_target ON transaction_links(target_transaction_id);

CREATE OR REPLACE FUNCTION validate_transaction_link_amounts() 
RETURNS TRIGGER AS $$
DECLARE
    source_txn_amount BIGINT;
    source_linked_total BIGINT;
    target_txn_amount BIGINT;
    target_linked_total BIGINT;
BEGIN
    -- 1. Validate Source Transaction
    SELECT ABS(amount) INTO source_txn_amount 
    FROM transactions 
    WHERE id = NEW.source_transaction_id 
    FOR SHARE;
    
    SELECT COALESCE(SUM(amount), 0) INTO source_linked_total 
    FROM transaction_links 
    WHERE source_transaction_id = NEW.source_transaction_id 
      AND id != COALESCE(NEW.id, '00000000-0000-0000-0000-000000000000'::uuid);
      
    IF (source_linked_total + NEW.amount) > source_txn_amount THEN
        RAISE EXCEPTION 'Linked amount exceeds available source transaction amount';
    END IF;

    -- 2. Validate Target Transaction (similar logic)
    SELECT ABS(amount) INTO target_txn_amount 
    FROM transactions 
    WHERE id = NEW.target_transaction_id 
    FOR SHARE;
    
    SELECT COALESCE(SUM(amount), 0) INTO target_linked_total 
    FROM transaction_links 
    WHERE target_transaction_id = NEW.target_transaction_id 
      AND id != COALESCE(NEW.id, '00000000-0000-0000-0000-000000000000'::uuid);
      
    IF (target_linked_total + NEW.amount) > target_txn_amount THEN
        RAISE EXCEPTION 'Linked amount exceeds available target transaction amount';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
