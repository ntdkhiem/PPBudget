-- 0032_deadline_reminders.up.sql
--
-- Deadline reminders: an email a week ahead of each dated action.
--
-- One row per action and due date, written when the email goes out. The key
-- is what makes the daily job safe to run twice -- the in-process cron and the
-- scheduled HTTP trigger both call it, and a host that slept through one runs
-- the other. A new due date (next year's ESPP purchase, say) is a new row.

BEGIN;

CREATE TABLE wealth_reminders_sent (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    catalog_key TEXT NOT NULL,
    due_on      DATE NOT NULL,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, catalog_key, due_on)
);

COMMIT;
