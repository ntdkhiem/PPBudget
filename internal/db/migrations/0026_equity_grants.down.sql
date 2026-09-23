-- 0026_equity_grants.down.sql
--
-- Drops equity compensation records. Nothing upstream holds this data: grants,
-- strike prices and vesting anchors come from the user or their broker, never
-- from the transaction feed.

DROP TABLE IF EXISTS equity_grants;
