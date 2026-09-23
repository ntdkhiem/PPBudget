-- 0025_paystub_ytd.down.sql
--
-- Drops year-to-date payroll history. Not recoverable from transaction data:
-- the aggregator sees net deposits, never gross, withholding, or the
-- pre-tax deduction split.

DROP TABLE IF EXISTS paystub_ytd;
