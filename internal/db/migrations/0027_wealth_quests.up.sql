-- 0027_wealth_quests.up.sql
--
-- Generated actions and their history.
--
-- The catalog itself -- which actions exist, their conditions, their ordering,
-- what unlocks a phase -- lives in Go (internal/service/quests_catalog.go),
-- not here, for the same reason internal/service/rules_vocabulary.go does: a
-- financial rule must be versioned, diffable and unit-testable, and a
-- user-editable tax rule is a liability. These tables hold only INSTANCES: the
-- actions generated for one user, with their interpolated text and status.
--
-- This replaces a purely-derived model. computeWaterfall() in planning-math.ts
-- recomputes stage state from current balances on every render and stores
-- nothing, which means an action silently un-completes when a balance dips and
-- there is no record that it was ever done. Persisting instances plus an
-- append-only event log is what makes completion durable and makes duration
-- conditions -- "90 consecutive days under the discretionary cap" -- answerable
-- at all.

BEGIN;

CREATE TABLE wealth_quests (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Stable identifier from the Go catalog, e.g. 'capture_employer_match'.
    catalog_key TEXT NOT NULL,
    phase       INT NOT NULL CHECK (phase BETWEEN 0 AND 5),
    -- Lower runs first. Phase 1 gives capture_employer_match priority 0: a 50%
    -- match is an instant 50% return and outranks even a 20% APR balance.
    priority    INT NOT NULL DEFAULT 100,

    status TEXT NOT NULL CHECK (status IN (
        'locked',          -- an earlier phase is unfinished
        'blocked',         -- unlockable, but an input is missing
        'available',       -- ready to work on
        'complete',
        'skipped',
        'not_applicable'   -- a condition rules it out entirely (no HDHP, renter, no equity)
    )),

    -- Interpolated at generation from real figures, so the row carries the
    -- dollar amounts the user sees without re-running the engine to render it.
    title  TEXT NOT NULL,
    detail TEXT,

    target_amount BIGINT,
    -- The gating escape hatch. An action with a due_date surfaces regardless of
    -- whether its phase is unlocked, because deadlines do not wait for phases:
    -- 401(k) and payroll-HSA close on 31 December with no do-over, IRA and
    -- direct HSA on the filing deadline, open enrollment inside a two-week
    -- window, and equity vests whenever the grant says so.
    due_date      DATE,

    verification TEXT NOT NULL DEFAULT 'manual' CHECK (verification IN ('auto','manual')),
    -- Recorded from the first release but not honoured until the
    -- auto-verification pass lands; until then every action completes by hand.
    -- Storing it now means the catalog declares intent once rather than being
    -- revisited later.

    -- Set when a computed input is past its confirmation cadence. The action
    -- still renders, flagged, rather than disappearing -- a plan built on last
    -- year salary figures should be visible, not silently absent.
    stale BOOLEAN NOT NULL DEFAULT FALSE,

    -- Field keys whose answers would move this action out of 'blocked'. This is
    -- what lets the UI say "answer 2 questions to unlock" against the action
    -- dollar value, instead of presenting the remaining questions as a second
    -- wall.
    missing_fields TEXT[] NOT NULL DEFAULT '{}',

    completed_at TIMESTAMPTZ,
    -- Who closed this action, and therefore whether regeneration may reopen it.
    --
    -- A MANUAL completion is a claim by the user ("I set up the 10b5-1") and
    -- must survive regeneration: the app cannot observe the thing they did, so
    -- it has no standing to contradict them. An AUTO completion is a derived
    -- fact ("your cushion covers a month"), and a derived fact has to track the
    -- data it came from -- otherwise a completion computed from a balance that
    -- has since fallen, or from spending history that was never there, outlives
    -- its own justification and the plan reports success it cannot support.
    completed_source TEXT CHECK (completed_source IN ('auto','manual','system')),
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One live instance per catalog entry per user. Regeneration updates in
    -- place so completion history survives it.
    UNIQUE (user_id, catalog_key),
    CHECK ((status = 'complete') = (completed_at IS NOT NULL))
);

CREATE INDEX idx_wealth_quests_user_phase ON wealth_quests (user_id, phase, priority);
-- Drives the "what is due soon, regardless of phase" sweep.
CREATE INDEX idx_wealth_quests_due ON wealth_quests (user_id, due_date)
    WHERE due_date IS NOT NULL AND status NOT IN ('complete','skipped','not_applicable');

CREATE TRIGGER update_wealth_quests_modtime BEFORE UPDATE ON wealth_quests
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- Append-only. Never updated, never deleted except by user cascade.
--
-- Two things depend on this existing separately from wealth_quests.status:
--   * duration conditions, which are queries over history rather than facts
--     about the present ("under the cap for 90 consecutive days");
--   * an honest record of what the user actually did and when, which a
--     mutable status column cannot provide once regeneration rewrites a row.
CREATE TABLE wealth_quest_events (
    id       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    quest_id UUID NOT NULL REFERENCES wealth_quests(id) ON DELETE CASCADE,
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event    TEXT NOT NULL CHECK (event IN (
        'generated','activated','completed','uncompleted','skipped','unskipped'
    )),
    -- 'auto' means the evaluator observed the condition met; 'manual' means the
    -- user said so. Worth keeping apart: they carry different confidence, and
    -- an auto-completion that later proves wrong should be traceable.
    source     TEXT NOT NULL CHECK (source IN ('auto','manual','system')),
    note       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wealth_quest_events_quest ON wealth_quest_events (quest_id, created_at DESC);
CREATE INDEX idx_wealth_quest_events_user ON wealth_quest_events (user_id, created_at DESC);

COMMIT;
