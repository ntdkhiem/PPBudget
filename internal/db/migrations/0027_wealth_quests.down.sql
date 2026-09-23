-- 0027_wealth_quests.down.sql
--
-- Drops generated actions and their event history.
--
-- Action instances regenerate from the profile, so losing wealth_quests costs
-- little. The event log is the real loss: completion timestamps and the
-- manual-versus-auto distinction cannot be reconstructed, and any duration
-- condition ("90 consecutive days under the cap") restarts its clock from zero
-- on the next generation.

BEGIN;

DROP TABLE IF EXISTS wealth_quest_events;
DROP TABLE IF EXISTS wealth_quests;

COMMIT;
