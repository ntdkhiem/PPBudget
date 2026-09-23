-- 0025_paystub_ytd.up.sql
--
-- Year-to-date payroll figures, read off a paystub.
--
-- This table replaces three intake questions that asked the user to recall
-- per-paycheck figures: exact gross before deductions, exact pre-tax
-- deductions, and total tax withheld. People cannot reliably recall any of
-- those, but they can read all of them off one document in one sitting -- and
-- the YTD column is strictly more useful than a per-paycheck estimate because
-- it supports full-year projection, 402(g) pacing, and headroom directly.
--
-- Two deliberate omissions:
--
--   * FICA is NOT stored. It is mechanical -- 7.65% to the wage base, then
--     1.45% plus the 0.9% additional Medicare surtax above it -- so it is
--     computed from gross and the wage base in tax_limits. The old intake
--     question bundled FICA together with income tax, which made the number
--     useless for every downstream calculation.
--   * There is no net-pay column. Net is derivable, and income already comes
--     from transaction data.
--
-- Rows are DATED rather than overwritten. A user who changes their deferral
-- rate mid-year produces two rows whose deltas reveal the change; a single
-- mutable row would hide it and silently mis-pace the rest of the year.
CREATE TABLE paystub_ytd (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    as_of_date            DATE NOT NULL,
    -- Every figure is year-to-date as printed, in cents.
    gross                 BIGINT NOT NULL CHECK (gross >= 0),
    federal_withheld      BIGINT NOT NULL DEFAULT 0,
    -- Kept separate from federal: the marginal-rate calculation needs the two
    -- apart, and a combined figure cannot be split back out.
    state_withheld        BIGINT NOT NULL DEFAULT 0,
    pretax_401k           BIGINT NOT NULL DEFAULT 0,
    roth_401k             BIGINT NOT NULL DEFAULT 0,
    hsa_contribution      BIGINT NOT NULL DEFAULT 0,
    espp_contribution     BIGINT NOT NULL DEFAULT 0,
    pretax_benefits       BIGINT NOT NULL DEFAULT 0,
    -- Supplemental wages (bonus, RSU vest income) tracked separately because
    -- federal supplemental withholding steps from 22% to 37% once aggregate
    -- supplemental wages cross the statutory threshold. That crossover is a
    -- real, dated event for an equity-compensated earner, and without this
    -- column the engine cannot see it coming.
    supplemental_wages    BIGINT NOT NULL DEFAULT 0,
    -- Paychecks issued so far and expected in the year. Together they give the
    -- "3 paychecks left to capture $4,000 of match" figure, which is the whole
    -- point of tracking pace rather than totals.
    paychecks_ytd         INT CHECK (paychecks_ytd >= 0),
    paychecks_per_year    INT CHECK (paychecks_per_year IN (12, 24, 26, 52)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, as_of_date)
);

CREATE INDEX idx_paystub_ytd_user_date ON paystub_ytd (user_id, as_of_date DESC);
