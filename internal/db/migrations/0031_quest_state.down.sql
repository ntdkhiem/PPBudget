-- 0031_quest_state.down.sql
--
-- Rebuilds the stored action list from state and marks.
--
-- Titles, phases and figures were never kept past this migration, so the rows
-- come back with placeholders; the rolled-back code regenerates all of them on
-- its first read. Statuses, completions, skips and history survive.

BEGIN;

CREATE TABLE wealth_quests (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    catalog_key TEXT NOT NULL,
    phase       INT NOT NULL CHECK (phase BETWEEN 0 AND 5),
    priority    INT NOT NULL DEFAULT 100,
    status TEXT NOT NULL CHECK (status IN (
        'locked', 'blocked', 'available', 'complete', 'skipped', 'not_applicable'
    )),
    title  TEXT NOT NULL,
    detail TEXT,
    target_amount BIGINT,
    due_date      DATE,
    verification TEXT NOT NULL DEFAULT 'manual' CHECK (verification IN ('auto','manual')),
    stale BOOLEAN NOT NULL DEFAULT FALSE,
    missing_fields TEXT[] NOT NULL DEFAULT '{}',
    completed_at TIMESTAMPTZ,
    completed_source TEXT CHECK (completed_source IN ('auto','manual','system')),
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, catalog_key),
    CHECK ((status = 'complete') = (completed_at IS NOT NULL))
);

CREATE INDEX idx_wealth_quests_user_phase ON wealth_quests (user_id, phase, priority);
CREATE INDEX idx_wealth_quests_due ON wealth_quests (user_id, due_date)
    WHERE due_date IS NOT NULL AND status NOT IN ('complete','skipped','not_applicable');

CREATE TRIGGER update_wealth_quests_modtime BEFORE UPDATE ON wealth_quests
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- A mark wins over the engine's last reading, as it did in the stored list.
INSERT INTO wealth_quests
    (user_id, catalog_key, phase, priority, status, title, verification,
     completed_at, completed_source, generated_at)
SELECT user_id, catalog_key, 0, 100,
       COALESCE(m.mark, s.status),
       catalog_key,
       'manual',
       CASE WHEN COALESCE(m.mark, s.status) = 'complete'
            THEN COALESCE(m.created_at, s.achieved_at, s.changed_at) END,
       CASE WHEN m.mark = 'complete' THEN 'manual'
            WHEN m.mark IS NULL AND s.status = 'complete' THEN 'auto' END,
       COALESCE(s.changed_at, m.created_at)
FROM wealth_quest_state s
FULL JOIN wealth_quest_marks m USING (user_id, catalog_key);

ALTER TABLE wealth_quest_events ADD COLUMN quest_id UUID;
UPDATE wealth_quest_events e SET quest_id = q.id
FROM wealth_quests q WHERE q.user_id = e.user_id AND q.catalog_key = e.catalog_key;
-- History for an action with no row left to hang it on cannot be carried back.
DELETE FROM wealth_quest_events WHERE quest_id IS NULL;
ALTER TABLE wealth_quest_events ALTER COLUMN quest_id SET NOT NULL;
ALTER TABLE wealth_quest_events
    ADD CONSTRAINT wealth_quest_events_quest_id_fkey
    FOREIGN KEY (quest_id) REFERENCES wealth_quests(id) ON DELETE CASCADE;
CREATE INDEX idx_wealth_quest_events_quest ON wealth_quest_events (quest_id, created_at DESC);
DROP INDEX idx_wealth_quest_events_key;
ALTER TABLE wealth_quest_events DROP COLUMN catalog_key;

DROP TABLE wealth_quest_state;
DROP TABLE wealth_quest_marks;

COMMIT;
