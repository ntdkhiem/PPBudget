-- 0028_tax_limits.down.sql
--
-- Drops the statutory limit table, including any year rows an operator added
-- after the initial seed. Those are re-enterable from IRS publications, so this
-- rollback loses configuration rather than user data.

DROP TABLE IF EXISTS tax_limits;
