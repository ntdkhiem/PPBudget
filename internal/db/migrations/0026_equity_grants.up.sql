-- 0026_equity_grants.up.sql
--
-- Equity compensation. Entirely new territory: a grep of the Go and SQL tree
-- for rsu|espp|vest|grant|strike returns nothing before this migration, which
-- is why the app could model a $145k salary but not the $80k RSU grant sitting
-- next to it.
--
-- The shape that matters here is the ANCHOR. The previous intake asked for
-- "vesting frequency and percentage schedule", which cannot produce a date: it
-- says how often shares vest but not where in the schedule the user currently
-- is. grant_date plus shares_vested plus next_vest_date is what turns a
-- schedule into a dated action. A schedule without an anchor is not a
-- projection, it is a description.
--
-- Those dates are also why the generator has a gating escape hatch. A vest 41
-- days out and a fixed ESPP purchase date happen on the calendar's schedule,
-- not the plan's, so an action carrying a due_date surfaces even when the
-- phase that owns it is still locked.

BEGIN;

CREATE TABLE equity_grants (
    id       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind     TEXT NOT NULL CHECK (kind IN ('rsu','espp','iso','nso')),
    label    TEXT,

    -- Shared ----------------------------------------------------------------
    grant_date      DATE,
    total_shares    NUMERIC(18,4) CHECK (total_shares >= 0),
    shares_vested   NUMERIC(18,4) CHECK (shares_vested >= 0),
    next_vest_date  DATE,
    vest_frequency  TEXT CHECK (vest_frequency IN ('monthly','quarterly','semiannual','annual','cliff')),
    -- Fraction of the grant vesting per tranche, e.g. 0.0625 for quarterly
    -- over four years.
    vest_share      NUMERIC(6,4) CHECK (vest_share BETWEEN 0 AND 1),

    -- ESPP ------------------------------------------------------------------
    -- discount_pct with lookback is typically the best risk-adjusted return
    -- available to this persona, which is why contribution_pct is here: the
    -- previous intake collected the discount but never the current rate, so
    -- "raise your ESPP contribution" had no X to raise from and no ceiling to
    -- raise to.
    espp_discount_pct     NUMERIC(6,4) CHECK (espp_discount_pct BETWEEN 0 AND 1),
    espp_has_lookback     BOOLEAN,
    espp_contribution_pct NUMERIC(6,4) CHECK (espp_contribution_pct BETWEEN 0 AND 1),
    espp_plan_max_pct     NUMERIC(6,4) CHECK (espp_plan_max_pct BETWEEN 0 AND 1),
    espp_offering_start   DATE,
    espp_purchase_date    DATE,

    -- Options ---------------------------------------------------------------
    -- Recorded from day one even though the ISO/NSO action branch (AMT
    -- crossover, the 90-day post-termination window, the 83(b) election) ships
    -- later. An expired in-the-money option is a permanent total loss, so the
    -- data needs to be present before the rules that watch it exist.
    strike_price    BIGINT,   -- cents per share
    expiration_date DATE,

    -- Liquidity constraints -------------------------------------------------
    -- "Sell within 48 hours of vest" is not executable during a closed trading
    -- window. Without these two the engine issues an instruction the user
    -- cannot follow and has no fallback to offer.
    has_10b5_1      BOOLEAN,
    blackout_policy TEXT CHECK (blackout_policy IN ('none','quarterly','event_based','unknown')),

    -- Overrides the statutory default (22% federal, stepping to 37% above the
    -- supplemental-wage threshold) when an employer withholds at some other
    -- rate. NULL means "use the default", not "zero".
    supplemental_withholding_pct NUMERIC(6,4) CHECK (supplemental_withholding_pct BETWEEN 0 AND 1),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Vesting cannot exceed the grant.
    CHECK (shares_vested IS NULL OR total_shares IS NULL OR shares_vested <= total_shares),
    -- Strike and expiration are meaningful only for options; discount and
    -- purchase dates only for ESPP. Keeping these apart stops a malformed row
    -- from reaching a rule that would silently treat an RSU as an option.
    CHECK (kind IN ('iso','nso') OR (strike_price IS NULL AND expiration_date IS NULL)),
    CHECK (kind = 'espp' OR (espp_discount_pct IS NULL AND espp_purchase_date IS NULL))
);

CREATE INDEX idx_equity_grants_user ON equity_grants (user_id);
-- The dated-action sweep asks "what vests or purchases soon" on every
-- evaluation, across both date columns.
CREATE INDEX idx_equity_grants_next_vest ON equity_grants (user_id, next_vest_date)
    WHERE next_vest_date IS NOT NULL;
CREATE INDEX idx_equity_grants_purchase ON equity_grants (user_id, espp_purchase_date)
    WHERE espp_purchase_date IS NOT NULL;

CREATE TRIGGER update_equity_grants_modtime BEFORE UPDATE ON equity_grants
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

COMMIT;
