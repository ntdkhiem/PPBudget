-- 0024_wealth_profile.up.sql
--
-- The user-facts half of the Wealth Strategy feature.
--
-- Until now everything beyond balances and transactions lived as two opaque JSON
-- strings in user_settings ('financial_plan', 'retirement_plan'), written through
-- PUT /settings/values/{key}, which accepts any key and any value with no
-- validation. That was survivable while the blobs only drove two read-only
-- pages. It stops being survivable once a deterministic engine reads them to
-- decide what to tell someone to do with their money, so the facts move into
-- typed, constrained columns here.
--
-- EVERY ANSWER COLUMN IS NULLABLE, DELIBERATELY. Intake is progressive: a user
-- answers an eight-screen core, gets a real action list, and answers the rest
-- later, each question attached to the action it unlocks. A NULL means "not
-- asked yet", which the generator reads as "this action stays locked" -- never
-- as zero. Defaults would erase that distinction and let rules fire on numbers
-- nobody supplied.

BEGIN;

-- One row per user. Wide rather than key-value because these are heterogeneous
-- typed facts with real CHECK constraints, and the generator reads nearly all of
-- them on every evaluation.
CREATE TABLE wealth_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

    -- Household and horizon -------------------------------------------------
    -- date_of_birth drives every catch-up tier (50+, the 60-63 super catch-up,
    -- HSA 55+), SECURE 2.0 mandatory-Roth catch-up for high earners, Rule of
    -- 55, and the 59-and-a-half gate. One column, six rules.
    date_of_birth            DATE,
    marital_status           TEXT CHECK (marital_status IN ('single','married','domestic_partner')),
    filing_status            TEXT CHECK (filing_status IN ('single','mfj','mfs','hoh','qss')),
    resident_state           TEXT CHECK (char_length(resident_state) = 2),
    -- Spouse income is not optional colour: filing status alone is actively
    -- dangerous. An MFJ user who supplies only their own income gets told to
    -- contribute directly to a Roth IRA when household MAGI makes them
    -- ineligible -- a 6% excise tax per year until corrected.
    spouse_gross_annual        BIGINT,   -- cents
    spouse_has_workplace_plan  BOOLEAN,  -- governs the traditional IRA deduction phase-out
    target_independence_age    INT CHECK (target_independence_age BETWEEN 30 AND 100),

    -- Income ----------------------------------------------------------------
    -- Gross annual pay. The paystub YTD table (0025) is the richer and more
    -- accurate source, but the employer-match calculation is expressed as a
    -- share of salary and needs a single annual figure, so this stays as the
    -- resolved value: entered directly, annualised from a paystub, or carried
    -- over from the retirement_plan blob during backfill.
    gross_annual_income      BIGINT,

    -- Employer plan mechanics -----------------------------------------------
    -- deferral_pct is the CURRENT rate. YTD contributions give a historical
    -- average, which diverges from the current rate exactly when it matters
    -- (someone who just changed it, or front-loaded). Without this there is no
    -- X in "raise your deferral from X to Y".
    deferral_pct             NUMERIC(6,4) CHECK (deferral_pct BETWEEN 0 AND 1),
    deferral_roth_share      NUMERIC(6,4) CHECK (deferral_roth_share BETWEEN 0 AND 1),
    match_pct                NUMERIC(6,4) CHECK (match_pct >= 0),
    match_limit_pct          NUMERIC(6,4) CHECK (match_limit_pct BETWEEN 0 AND 1),
    -- A plan that funds the match per paycheck with no true-up forfeits the
    -- match for every paycheck after the 402(g) cap is hit. Front-loading to the
    -- limit then costs someone thousands, permanently. This pair is the guard.
    match_per_paycheck       BOOLEAN,
    match_has_true_up        BOOLEAN,
    -- After-tax contributions plus either in-plan conversion or in-service
    -- withdrawal is the mega backdoor Roth -- the largest single action
    -- available to this persona. accepts_rollovers is separately the fix for a
    -- blocked backdoor Roth: roll the pre-tax IRA into the 401(k), zeroing the
    -- pro-rata denominator.
    plan_allows_after_tax    BOOLEAN,
    plan_allows_in_service   BOOLEAN,
    plan_accepts_rollovers   BOOLEAN,

    -- HSA -------------------------------------------------------------------
    -- "Max the HSA" is not computable from hdhp_enrolled alone. Family coverage
    -- is thousands higher than self-only, and an employer seed reduces the
    -- remaining room. Either one wrong produces an excess contribution.
    hdhp_enrolled              BOOLEAN,
    hsa_coverage_tier          TEXT CHECK (hsa_coverage_tier IN ('self_only','family')),
    hsa_employer_contribution  BIGINT,   -- cents per year

    -- Tax -------------------------------------------------------------------
    -- Prior-year liability and AGI are the safe-harbour inputs: 100% of prior
    -- year tax, or 110% above the AGI threshold. They turn "set some money
    -- aside for the vest" into "add $530 to line 4(c) for 7 paychecks".
    prior_year_tax_liability BIGINT,
    prior_year_agi           BIGINT,
    annual_bonus             BIGINT,
    bonus_month              INT CHECK (bonus_month BETWEEN 1 AND 12),

    -- Targets ---------------------------------------------------------------
    -- Months, not dollars. The dollar figure is the planner OUTPUT, derived
    -- from computed essentials; asking the user for it asks them to do the job
    -- they came here for.
    emergency_fund_target_months INT CHECK (emergency_fund_target_months BETWEEN 1 AND 24),

    -- Holdings --------------------------------------------------------------
    -- traditional_ira_balance is the pro-rata denominator for a backdoor Roth.
    traditional_ira_balance  BIGINT,
    -- Unrealized gain is a tax budget, not trivia. "Consolidate into an index
    -- core" against a large embedded gain is a recommendation to realize it;
    -- the correct action there is redirect-new-money plus opportunistic
    -- harvesting, which is a different action entirely.
    taxable_brokerage_value  BIGINT,
    taxable_unrealized_gain  BIGINT,
    taxable_employer_stock   BIGINT,

    -- Employer equity context -----------------------------------------------
    -- employer_is_public gates the whole of phase 2. Private-company employees
    -- have double-trigger RSUs, no liquidity and no 10b5-1; handing them
    -- "sell on vest" is handing them an impossible instruction. No ticker also
    -- means no way to price a vest, so the tax-gap action has no dollar figure.
    employer_is_public       BOOLEAN,
    employer_ticker          TEXT,
    has_stock_options        BOOLEAN,

    -- Risk transfer ---------------------------------------------------------
    -- Group LTD typically replaces 60% of BASE salary only, capped, and is
    -- taxable when the employer pays the premium. For someone whose comp is
    -- half equity that is well under a third of real income -- the largest
    -- un-hedged exposure in the household, and nothing else here protects the
    -- stream that funds every other action.
    ltd_replacement_pct      NUMERIC(6,4) CHECK (ltd_replacement_pct BETWEEN 0 AND 1),
    ltd_monthly_cap          BIGINT,
    ltd_premium_pretax       BOOLEAN,
    life_death_benefit       BIGINT,

    -- Housing and debt ------------------------------------------------------
    housing_tenure           TEXT CHECK (housing_tenure IN ('rent','own')),
    mortgage_apr             NUMERIC(6,4) CHECK (mortgage_apr BETWEEN 0 AND 1),
    mortgage_balance         BIGINT,
    pays_pmi                 BOOLEAN,
    -- Without the IDR/PSLF flags the engine recommends extra principal on a
    -- loan headed for forgiveness: a pure, unrecoverable loss, and the worst
    -- single output this catalog can produce.
    student_loan_kind        TEXT CHECK (student_loan_kind IN ('none','federal','private','both')),
    student_loan_idr         BOOLEAN,
    student_loan_pslf        BOOLEAN,

    -- Projection assumptions (migrated from the retirement_plan blob) --------
    expected_return_apr      NUMERIC(6,4) CHECK (expected_return_apr BETWEEN -1 AND 1),
    inflation_apr            NUMERIC(6,4) CHECK (inflation_apr BETWEEN -1 AND 1),
    withdrawal_rate          NUMERIC(6,4) CHECK (withdrawal_rate BETWEEN 0 AND 1),
    target_annual_spend      BIGINT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_wealth_profile_modtime BEFORE UPDATE ON wealth_profiles
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- Per-field provenance and age ----------------------------------------------
--
-- One table doing three jobs that would otherwise need three mechanisms:
--   * absence of a row means UNANSWERED, which is how progressive intake knows
--     what to ask next and which actions stay locked;
--   * source separates a derived pre-fill nobody looked at from one the user
--     actually confirmed -- high-stakes branches (the backdoor Roth decision)
--     require 'confirmed' or 'entered' and must never fire off 'derived';
--   * answered_at drives staleness, so a plan is never quietly built on last
--     year salary figures.
--
-- Provenance lives here rather than as paired columns on wealth_profiles
-- because adding a question then costs one column and one row key rather than
-- three columns, and the question set is expected to keep growing.
CREATE TABLE wealth_profile_fields (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    field_key   TEXT NOT NULL,
    source      TEXT NOT NULL CHECK (source IN ('derived','confirmed','entered','default')),
    answered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, field_key)
);

CREATE INDEX idx_wealth_profile_fields_user ON wealth_profile_fields (user_id);

-- Terms the aggregator does not return ---------------------------------------
--
-- APR finally gets a real home: it currently lives in the financial_plan blob
-- as DebtInfo.apr, which is why the cash-plan waterfall has a "you have
-- liabilities but no interest rates entered" blocked state.
--
-- promo_apr / promo_expires_on exist because a 0% promotional balance must be
-- EXCLUDED from the ordinary payoff rule and given a dated action instead:
-- deferred-interest cards retroactively charge all accrued interest if any
-- balance remains at expiry, which inverts the usual "lowest rate last" advice.
CREATE TABLE account_terms (
    account_id        UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    apy               NUMERIC(6,4) CHECK (apy BETWEEN 0 AND 1),
    apr               NUMERIC(6,4) CHECK (apr BETWEEN 0 AND 1),
    promo_apr         NUMERIC(6,4) CHECK (promo_apr BETWEEN 0 AND 1),
    promo_expires_on  DATE,
    statement_balance BIGINT,
    min_payment       BIGINT,
    due_day           INT CHECK (due_day BETWEEN 1 AND 31),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((promo_apr IS NULL) = (promo_expires_on IS NULL))
);

CREATE INDEX idx_account_terms_user ON account_terms (user_id);

CREATE TRIGGER update_account_terms_modtime BEFORE UPDATE ON account_terms
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- Lifts RetirementProfile.accounts out of the retirement_plan blob unchanged.
-- Deliberately NO annual_limit column: the generator resolves limits from
-- tax_limits (0028) by tax year. Storing a user-maintained "limit" guarantees
-- it is wrong every January, which is the flaw in the blob this replaces.
CREATE TABLE retirement_account_terms (
    account_id           UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    user_id              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind                 TEXT NOT NULL CHECK (kind IN ('401k','roth_401k','ira','roth_ira','hsa','taxable')),
    monthly_contribution BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retirement_account_terms_user ON retirement_account_terms (user_id);

CREATE TRIGGER update_retirement_account_terms_modtime BEFORE UPDATE ON retirement_account_terms
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- Dependents: Dependent Care FSA, 529 funding against the state deduction
-- (resident_state is already known), the life insurance need calculation, and
-- the Child Tax Credit effect on the marginal rate. Birth year rather than age
-- so the row does not silently rot.
CREATE TABLE dependents (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    birth_year INT NOT NULL CHECK (birth_year BETWEEN 1900 AND 2200),
    label      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dependents_user ON dependents (user_id);

-- Near-term earmarked cash, carved out of every sweep and invest rule.
--
-- This is the one table here that exists for safety rather than optimisation.
-- Without it the engine sees idle cash and tells someone with a March closing
-- date to move their down payment into a broad-market index fund.
CREATE TABLE planned_expenses (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    amount      BIGINT NOT NULL CHECK (amount > 0),
    target_date DATE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_planned_expenses_user_date ON planned_expenses (user_id, target_date);

CREATE TRIGGER update_planned_expenses_modtime BEFORE UPDATE ON planned_expenses
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

COMMIT;
