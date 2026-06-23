-- 0006_complex_rules.up.sql
ALTER TABLE rules ADD COLUMN trigger_type TEXT NOT NULL DEFAULT 'store-journal';
ALTER TABLE rules ADD COLUMN strictness TEXT NOT NULL DEFAULT 'all';
