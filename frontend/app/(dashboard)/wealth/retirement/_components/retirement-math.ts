/**
 * Retirement arithmetic. Pure: no React, no fetching, no DOM.
 *
 * Everything is computed in TODAY'S DOLLARS using a real return
 * (nominal minus inflation). Projecting in nominal dollars produces large,
 * flattering numbers that buy less than they appear to; keeping one basis
 * means the target and the projection are directly comparable.
 *
 * Money is integer cents throughout. Rates are fractions, not percentages.
 */

import type { PlanningAccount } from "@/lib/api";

// ------------------------------------------------------------------ profile

export type AccountKind = "401k" | "roth_401k" | "ira" | "roth_ira" | "hsa" | "taxable";

export const ACCOUNT_KIND_LABELS: Record<AccountKind, string> = {
  "401k": "401(k), pre-tax",
  roth_401k: "Roth 401(k)",
  ira: "Traditional IRA",
  roth_ira: "Roth IRA",
  hsa: "HSA",
  taxable: "Taxable brokerage",
};

/** Only workplace plans can carry an employer match. */
export const MATCHABLE_KINDS: AccountKind[] = ["401k", "roth_401k"];

export interface RetirementAccount {
  /** Links to a real PPBudget account, so the balance is never hand-typed. */
  account_id: string;
  kind: AccountKind;
  monthly_contribution_cents: number;
  /** Employer match rate, e.g. 0.5 means 50 cents per dollar you put in. */
  employer_match_pct?: number;
  /** Match applies only up to this share of salary, e.g. 0.06 = 6%. */
  employer_match_limit_pct?: number;
  /** User-maintained: IRS limits change annually and are not hardcoded. */
  annual_limit_cents?: number;
}

export interface RetirementProfile {
  version: 1;
  current_age: number;
  target_retirement_age: number;
  gross_annual_income_cents: number;
  expected_return_apr: number;
  inflation_apr: number;
  withdrawal_rate: number;
  /** Expected annual spending in retirement. Defaults to current essentials. */
  target_annual_spend_cents?: number;
  accounts: RetirementAccount[];
}

export const DEFAULT_PROFILE: RetirementProfile = {
  version: 1,
  current_age: 0,
  target_retirement_age: 65,
  gross_annual_income_cents: 0,
  expected_return_apr: 0.07,
  inflation_apr: 0.03,
  withdrawal_rate: 0.04,
  accounts: [],
};

/** A profile with no age and no income has never been set up. */
export function isConfigured(p: RetirementProfile): boolean {
  return p.current_age > 0 && p.gross_annual_income_cents > 0;
}

// ------------------------------------------------------------------- inputs

export interface RetirementInputs {
  yearsToRetirement: number;
  /** Nominal return less inflation. Can be zero or negative. */
  realReturn: number;
  /** Balance across every linked account. */
  currentAssetsCents: number;
  /** Your own contributions, excluding any employer match. */
  ownAnnualContributionCents: number;
  employerMatchAnnualCents: number;
  totalAnnualContributionCents: number;
  targetAnnualSpendCents: number;
  /** The nest egg the withdrawal rate implies. */
  targetNestEggCents: number;
  /** Own contributions as a share of gross pay. */
  grossSavingsRate: number;
  /** Including employer match — the basis most 15% guidance uses. */
  grossSavingsRateWithMatch: number;
}

/**
 * Employer match, capped twice over: by the employer's stated limit as a share
 * of salary, and by what you actually contribute. Contributing less than the
 * limit forfeits the difference, which is the point of the match flag below.
 */
export function employerMatchFor(
  account: RetirementAccount,
  grossAnnualIncomeCents: number,
): number {
  if (!MATCHABLE_KINDS.includes(account.kind)) return 0;
  const matchPct = account.employer_match_pct ?? 0;
  const limitPct = account.employer_match_limit_pct ?? 0;
  if (matchPct <= 0 || limitPct <= 0 || grossAnnualIncomeCents <= 0) return 0;

  const ownAnnual = account.monthly_contribution_cents * 12;
  const ownRate = ownAnnual / grossAnnualIncomeCents;
  const matchedRate = Math.min(ownRate, limitPct);
  return Math.round(matchedRate * matchPct * grossAnnualIncomeCents);
}

/** The match you would earn by contributing all the way to the employer's limit. */
export function maxEmployerMatchFor(
  account: RetirementAccount,
  grossAnnualIncomeCents: number,
): number {
  if (!MATCHABLE_KINDS.includes(account.kind)) return 0;
  const matchPct = account.employer_match_pct ?? 0;
  const limitPct = account.employer_match_limit_pct ?? 0;
  if (matchPct <= 0 || limitPct <= 0 || grossAnnualIncomeCents <= 0) return 0;
  return Math.round(limitPct * matchPct * grossAnnualIncomeCents);
}

export function computeInputs(
  profile: RetirementProfile,
  accounts: PlanningAccount[],
  /** Monthly essentials from the cash-plan baseline, used as a spend default. */
  essentialMonthlyCents: number,
): RetirementInputs {
  const yearsToRetirement = Math.max(
    0,
    profile.target_retirement_age - profile.current_age,
  );
  const realReturn = profile.expected_return_apr - profile.inflation_apr;

  const currentAssetsCents = profile.accounts.reduce((sum, ra) => {
    const account = accounts.find((a) => a.id === ra.account_id);
    return sum + (account ? account.balance : 0);
  }, 0);

  const ownAnnualContributionCents = profile.accounts.reduce(
    (sum, ra) => sum + ra.monthly_contribution_cents * 12,
    0,
  );
  const employerMatchAnnualCents = profile.accounts.reduce(
    (sum, ra) => sum + employerMatchFor(ra, profile.gross_annual_income_cents),
    0,
  );

  const targetAnnualSpendCents =
    profile.target_annual_spend_cents ?? essentialMonthlyCents * 12;

  const targetNestEggCents =
    profile.withdrawal_rate > 0
      ? Math.round(targetAnnualSpendCents / profile.withdrawal_rate)
      : 0;

  const gross = profile.gross_annual_income_cents;

  return {
    yearsToRetirement,
    realReturn,
    currentAssetsCents,
    ownAnnualContributionCents,
    employerMatchAnnualCents,
    totalAnnualContributionCents: ownAnnualContributionCents + employerMatchAnnualCents,
    targetAnnualSpendCents,
    targetNestEggCents,
    grossSavingsRate: gross > 0 ? ownAnnualContributionCents / gross : 0,
    grossSavingsRateWithMatch:
      gross > 0 ? (ownAnnualContributionCents + employerMatchAnnualCents) / gross : 0,
  };
}

// --------------------------------------------------------------- projection

export interface ProjectionYear {
  year: number;
  age: number;
  balanceCents: number;
}

export interface RetirementProjection {
  years: ProjectionYear[];
  projectedCents: number;
  targetCents: number;
  /** Positive means short of the target. */
  gapCents: number;
  /** 0..1+, projected over target. */
  fundedShare: number;
  onTrack: boolean;
  /** Own monthly contribution that would close the gap, null if unreachable. */
  requiredMonthlyCents: number | null;
}

export function project(
  profile: RetirementProfile,
  inputs: RetirementInputs,
): RetirementProjection {
  const realReturn = inputs.realReturn;
  const years = Math.max(0, profile.target_retirement_age - profile.current_age);
  const annual = inputs.totalAnnualContributionCents;

  const series: ProjectionYear[] = [];
  let balance = inputs.currentAssetsCents;
  series.push({ year: 0, age: profile.current_age, balanceCents: Math.round(balance) });

  for (let i = 1; i <= years; i++) {
    balance = balance * (1 + realReturn) + annual;
    series.push({
      year: i,
      age: profile.current_age + i,
      balanceCents: Math.round(balance),
    });
  }

  const projectedCents = Math.round(balance);
  const targetCents = inputs.targetNestEggCents;
  const gapCents = targetCents - projectedCents;

  // What own-contribution would close the gap, holding everything else fixed.
  let requiredMonthlyCents: number | null = null;
  if (gapCents > 0 && years > 0) {
    // Future value of 1 cent per year, compounded at realReturn over `years`.
    const annuityFactor =
      Math.abs(realReturn) < 1e-9
        ? years
        : ((1 + realReturn) ** years - 1) / realReturn;
    if (annuityFactor > 0) {
      const extraAnnual = gapCents / annuityFactor;
      requiredMonthlyCents = Math.ceil(
        (inputs.ownAnnualContributionCents + extraAnnual) / 12,
      );
    }
  }

  return {
    years: series,
    projectedCents,
    targetCents,
    gapCents,
    fundedShare: targetCents > 0 ? projectedCents / targetCents : 0,
    onTrack: targetCents > 0 && projectedCents >= targetCents,
    requiredMonthlyCents,
  };
}

// -------------------------------------------------------------------- flags

export type FlagKind =
  | "no_accounts"
  | "unclaimed_match"
  | "over_limit"
  | "retire_age_invalid"
  | "negative_real_return";

export interface Flag {
  kind: FlagKind;
  severity: "bad" | "warn";
  title: string;
  detail: string;
}

// computeFlags() lived here. It was the only check in the old app that caught
// unclaimed employer match -- the thing the source strategy document missed
// entirely -- and that check now lives in the Go catalog, where it is ordered
// against every other action and covered by tests. The Flag type stays as the
// view model; see wealth/_components/plan-adapters.ts.

// ------------------------------------------------------------------ format

export function formatPct(fraction: number, digits = 0): string {
  return `${(fraction * 100).toFixed(digits)}%`;
}

/** Whole dollars, for prose where cents are noise. */
export function centsToDollars(cents: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(Math.round(cents / 100));
}
