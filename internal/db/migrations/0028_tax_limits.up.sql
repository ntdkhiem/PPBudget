-- 0028_tax_limits.up.sql
--
-- Annually-indexed statutory figures, as data rather than as Go constants.
--
-- Nearly every number the action engine needs to size a contribution moves each
-- year, and several are mid-phase-in under SECURE 2.0. Hardcoding any of them
-- produces an engine that is quietly wrong from 1 January until someone
-- notices, so every rule resolves its limits through this table by tax year.
--
-- The corollary matters just as much: the engine stores the user YEAR-TO-DATE
-- CONTRIBUTIONS (paystub_ytd, 0025) and computes headroom at evaluation time.
-- It never stores "their limit". retirement-math.ts currently does the opposite
-- -- RetirementAccount.annual_limit_cents is documented as "user-maintained:
-- IRS limits change annually and are not hardcoded" -- which is the right
-- instinct solved the wrong way, since it makes every user responsible for
-- tracking the IRS. 0024 deliberately drops that column.
--
-- ---------------------------------------------------------------------------
-- VERIFY THE SEED BEFORE RELYING ON IT.
--
-- The rows below are a starting point, not an authority. Confirm each figure
-- against the IRS notice for that year (Notice 2024-80 and its successors for
-- retirement limits, Rev. Proc. for HSA and the deduction/bracket tables)
-- before any of this reaches a real plan. Figures that are statutory rather
-- than indexed -- the section 423 ESPP cap, the supplemental-wage threshold,
-- the FICA rates -- are stable; the indexed ones are the ones to check.
--
-- Adding a year is an INSERT, not a migration. Operators should expect to add
-- a row set each autumn when the IRS publishes.
-- ---------------------------------------------------------------------------

BEGIN;

CREATE TABLE tax_limits (
    tax_year   INT  NOT NULL CHECK (tax_year BETWEEN 2000 AND 2200),
    key        TEXT NOT NULL,
    -- Exactly one of these is set. Splitting them keeps money in integer cents
    -- (consistent with pkg/money) and rates as exact decimals, rather than
    -- overloading one column and discriminating on a unit string at every
    -- read site.
    amount     BIGINT,        -- cents
    rate       NUMERIC(8,6),  -- fraction, e.g. 0.220000 for 22%
    note       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tax_year, key),
    CHECK ((amount IS NULL) <> (rate IS NULL))
);

-- Statutory and rate constants. These are not indexed annually, but they are
-- seeded per year anyway so that a single lookup path serves every rule and a
-- future change is one INSERT rather than a code edit.
INSERT INTO tax_limits (tax_year, key, amount, rate, note) VALUES
    (2025, 'espp_423_annual_cap',            2500000, NULL, 'Section 423(b)(8), grant-date FMV. Statutory.'),
    (2025, 'supplemental_wage_threshold',  100000000, NULL, 'Aggregate supplemental wages above which the 37% rate applies. Statutory.'),
    (2025, 'supplemental_rate_low',              NULL, 0.220000, 'Flat federal supplemental withholding below the threshold.'),
    (2025, 'supplemental_rate_high',             NULL, 0.370000, 'Mandatory rate above the threshold.'),
    (2025, 'fica_social_security_rate',          NULL, 0.062000, 'Employee share, to the wage base.'),
    (2025, 'fica_medicare_rate',                 NULL, 0.014500, 'Employee share, uncapped.'),
    (2025, 'fica_additional_medicare_rate',      NULL, 0.009000, 'Additional Medicare surtax above the threshold.'),
    (2025, 'safe_harbor_rate_standard',          NULL, 1.000000, 'Share of prior-year tax that satisfies safe harbour.'),
    (2025, 'safe_harbor_rate_high_agi',          NULL, 1.100000, 'Applies when prior-year AGI exceeds the threshold.'),
    (2026, 'espp_423_annual_cap',            2500000, NULL, 'Section 423(b)(8), grant-date FMV. Statutory.'),
    (2026, 'supplemental_wage_threshold',  100000000, NULL, 'Aggregate supplemental wages above which the 37% rate applies. Statutory.'),
    (2026, 'supplemental_rate_low',              NULL, 0.220000, 'Flat federal supplemental withholding below the threshold.'),
    (2026, 'supplemental_rate_high',             NULL, 0.370000, 'Mandatory rate above the threshold.'),
    (2026, 'fica_social_security_rate',          NULL, 0.062000, 'Employee share, to the wage base.'),
    (2026, 'fica_medicare_rate',                 NULL, 0.014500, 'Employee share, uncapped.'),
    (2026, 'fica_additional_medicare_rate',      NULL, 0.009000, 'Additional Medicare surtax above the threshold.'),
    (2026, 'safe_harbor_rate_standard',          NULL, 1.000000, 'Share of prior-year tax that satisfies safe harbour.'),
    (2026, 'safe_harbor_rate_high_agi',          NULL, 1.100000, 'Applies when prior-year AGI exceeds the threshold.');

-- Indexed figures. VERIFY THESE -- see the header.
INSERT INTO tax_limits (tax_year, key, amount, rate, note) VALUES
    -- 2025
    (2025, 'elective_deferral_402g',         2350000, NULL, 'VERIFY against the IRS notice.'),
    (2025, 'catchup_50_plus',                 750000, NULL, 'VERIFY.'),
    (2025, 'catchup_60_to_63',               1125000, NULL, 'SECURE 2.0 super catch-up, first effective 2025. VERIFY.'),
    (2025, 'total_additions_415c',           7000000, NULL, 'Mega backdoor headroom is this less deferrals and match. VERIFY.'),
    (2025, 'ira_contribution',                700000, NULL, 'VERIFY.'),
    (2025, 'ira_catchup_50_plus',             100000, NULL, 'VERIFY.'),
    (2025, 'hsa_self_only',                   430000, NULL, 'VERIFY.'),
    (2025, 'hsa_family',                      855000, NULL, 'VERIFY.'),
    (2025, 'hsa_catchup_55_plus',             100000, NULL, 'VERIFY.'),
    (2025, 'social_security_wage_base',      17610000, NULL, 'VERIFY.'),
    (2025, 'roth_magi_phaseout_single_start', 15000000, NULL, 'VERIFY.'),
    (2025, 'roth_magi_phaseout_single_end',   16500000, NULL, 'VERIFY.'),
    (2025, 'roth_magi_phaseout_mfj_start',    23600000, NULL, 'VERIFY.'),
    (2025, 'roth_magi_phaseout_mfj_end',      24600000, NULL, 'VERIFY.'),
    -- Married filing separately phases out from zero to $10,000 and is NOT
    -- indexed. Seeded rather than special-cased in code so every filing status
    -- resolves through one path; an MFS filer who lived with their spouse is
    -- effectively ineligible above $10,000, which the engine must be able to say.
    (2025, 'roth_magi_phaseout_mfs_start',           0, NULL, 'Statutory, not indexed.'),
    (2025, 'roth_magi_phaseout_mfs_end',       1000000, NULL, 'Statutory, not indexed.'),
    (2025, 'safe_harbor_agi_threshold',       15000000, NULL, 'Above this, safe harbour requires 110% of prior-year tax. VERIFY.'),
    (2025, 'additional_medicare_threshold_single', 20000000, NULL, 'Not indexed. VERIFY.'),
    (2025, 'additional_medicare_threshold_mfj',    25000000, NULL, 'Not indexed. VERIFY.'),
    (2025, 'secure2_roth_catchup_wage_threshold',  14500000, NULL, 'Prior-year FICA wages above which catch-up must be Roth. VERIFY.'),
    (2025, 'dependent_care_fsa_cap',           500000, NULL, 'VERIFY.'),
    -- 2026
    (2026, 'elective_deferral_402g',         2450000, NULL, 'VERIFY against the IRS notice.'),
    (2026, 'catchup_50_plus',                 800000, NULL, 'VERIFY.'),
    (2026, 'catchup_60_to_63',               1125000, NULL, 'SECURE 2.0 super catch-up. VERIFY.'),
    (2026, 'total_additions_415c',           7200000, NULL, 'VERIFY.'),
    (2026, 'ira_contribution',                750000, NULL, 'VERIFY.'),
    (2026, 'ira_catchup_50_plus',             110000, NULL, 'VERIFY.'),
    (2026, 'hsa_self_only',                   440000, NULL, 'VERIFY.'),
    (2026, 'hsa_family',                      875000, NULL, 'VERIFY.'),
    (2026, 'hsa_catchup_55_plus',             100000, NULL, 'VERIFY.'),
    (2026, 'social_security_wage_base',      18450000, NULL, 'VERIFY.'),
    (2026, 'roth_magi_phaseout_single_start', 15300000, NULL, 'VERIFY.'),
    (2026, 'roth_magi_phaseout_single_end',   16800000, NULL, 'VERIFY.'),
    (2026, 'roth_magi_phaseout_mfj_start',    24200000, NULL, 'VERIFY.'),
    (2026, 'roth_magi_phaseout_mfj_end',      25200000, NULL, 'VERIFY.'),
    (2026, 'roth_magi_phaseout_mfs_start',           0, NULL, 'Statutory, not indexed.'),
    (2026, 'roth_magi_phaseout_mfs_end',       1000000, NULL, 'Statutory, not indexed.'),
    (2026, 'safe_harbor_agi_threshold',       15000000, NULL, 'VERIFY.'),
    (2026, 'additional_medicare_threshold_single', 20000000, NULL, 'Not indexed. VERIFY.'),
    (2026, 'additional_medicare_threshold_mfj',    25000000, NULL, 'Not indexed. VERIFY.'),
    (2026, 'secure2_roth_catchup_wage_threshold',  15000000, NULL, 'VERIFY.'),
    (2026, 'dependent_care_fsa_cap',           750000, NULL, 'VERIFY.');

COMMIT;
