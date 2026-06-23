-- 0006_complex_rules.down.sql
ALTER TABLE rules DROP COLUMN trigger_type;
ALTER TABLE rules DROP COLUMN strictness;
