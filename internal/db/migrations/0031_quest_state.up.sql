-- 0031_quest_state.up.sql
--
-- Stop storing the action list; store only what cannot be recomputed.
--
-- wealth_quests held every generated action, and every read of the list
-- rewrote every row, pruned the ones the catalog no longer produced, and
-- appended events. Three things went wrong with that: a GET wrote to the
-- database on every page load; pruning cascaded away the history of any
-- action that stopped being produced -- a card paid to zero lost the record of
-- being paid; and a later phase re-locked whenever a milestone dipped.
--
-- The list is now worked out on every read and kept nowhere. What survives is:
--   * wealth_quest_marks: what the user said -- "done" or "not for me";
--   * wealth_quest_state: the last status the engine saw per action, written
--     only when it changes, plus the first time it was ever achieved;
--   * wealth_quest_events: history, keyed by action rather than by a row that
--     can be deleted out from under it.

BEGIN;

CREATE TABLE wealth_quest_marks (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    catalog_key TEXT NOT NULL,
    mark        TEXT NOT NULL CHECK (mark IN ('complete', 'skipped')),
    note        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, catalog_key)
);

CREATE TABLE wealth_quest_state (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    catalog_key TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN (
        'locked', 'blocked', 'available', 'complete', 'skipped', 'not_applicable'
    )),
    -- The first time this action was complete. Never cleared: phase gating
    -- reads it, so a phase once finished does not re-lock the next one when a
    -- balance dips.
    achieved_at TIMESTAMPTZ,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, catalog_key)
);

-- Carry the stored list over: statuses and achievements into state, and the
-- user's own claims -- skips and manual completions -- into marks.
INSERT INTO wealth_quest_state (user_id, catalog_key, status, achieved_at, changed_at)
SELECT user_id, catalog_key, status, completed_at, updated_at
FROM wealth_quests;

INSERT INTO wealth_quest_marks (user_id, catalog_key, mark, created_at)
SELECT user_id, catalog_key, 'skipped', updated_at
FROM wealth_quests WHERE status = 'skipped'
UNION ALL
SELECT user_id, catalog_key, 'complete', completed_at
FROM wealth_quests WHERE status = 'complete' AND completed_source = 'manual';

-- History keyed by action.
ALTER TABLE wealth_quest_events ADD COLUMN catalog_key TEXT;
UPDATE wealth_quest_events e SET catalog_key = q.catalog_key
FROM wealth_quests q WHERE q.id = e.quest_id;
ALTER TABLE wealth_quest_events ALTER COLUMN catalog_key SET NOT NULL;
ALTER TABLE wealth_quest_events DROP COLUMN quest_id;
CREATE INDEX idx_wealth_quest_events_key ON wealth_quest_events (user_id, catalog_key, created_at DESC);

DROP TABLE wealth_quests;

COMMIT;
