-- 0029_wealth_goals_and_overrides.up.sql
--
-- The last of what the 'financial_plan' blob still held.
--
-- Migrations 0024-0028 rehoused the facts: rates, terms, equity, limits. Four
-- things stayed behind, and without them the blob cannot be retired:
--
--   * strategy -- how hard the user wants to save, which sets the target rate
--     every "you are saving X against a target of Y" line is measured against;
--   * three OVERRIDES, where the user has told us the computed figure is wrong;
--   * their GOALS, which are the only genuinely user-authored content in the
--     whole feature -- named targets with dates that nothing can re-derive.
--
-- Goals get a table rather than a column because they are a list the user
-- edits, ordered, and joined to accounts. Everything else is a scalar and joins
-- the profile.

BEGIN;

-- Strategy and overrides -----------------------------------------------------
--
-- An override is a claim that our arithmetic is wrong about this user -- their
-- income is lumpy, or half their "needs" spending is really discretionary. It
-- is deliberately separate from the derived value rather than replacing it, so
-- the engine can still say which figures the user has corrected. NULL means
-- "no override", which is not the same as an override of zero.
ALTER TABLE wealth_profiles
    ADD COLUMN strategy TEXT CHECK (strategy IN ('aggressive', 'balanced', 'flexible')),
    ADD COLUMN override_monthly_income BIGINT,
    ADD COLUMN override_essential_expenses BIGINT,
    -- Which accounts count as the emergency fund. An account later deleted just
    -- stops matching, which is why this is an array rather than a constrained
    -- join: a stale id is harmless and a cascade here would be surprising.
    ADD COLUMN override_liquid_account_ids UUID[];

-- Goals ----------------------------------------------------------------------
--
-- Funded sequentially, like the action phases: each goal takes the whole
-- remaining surplus until it is met, which is what produces an honest start
-- date for the one after it. Splitting the surplus across goals instead would
-- make every date optimistic and none of them wrong in a way the user could
-- see.
CREATE TABLE wealth_goals (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    target_amount BIGINT NOT NULL CHECK (target_amount > 0),
    target_date   DATE,
    -- Optional: when a goal is backed by a real account, progress comes from
    -- that balance rather than from a number the user maintains by hand.
    linked_account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
    -- Used only when there is no linked account.
    current_amount BIGINT NOT NULL DEFAULT 0,
    -- Lower funds first. The order is the user's, not ours.
    priority   INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wealth_goals_user ON wealth_goals (user_id, priority);

CREATE TRIGGER update_wealth_goals_modtime BEFORE UPDATE ON wealth_goals
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

COMMIT;
