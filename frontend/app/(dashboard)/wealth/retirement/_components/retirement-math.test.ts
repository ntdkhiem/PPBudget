import { describe, expect, it } from "vitest";
import {
  DEFAULT_PROFILE,
  centsToDollars,
  computeInputs,
  formatPct,
  isConfigured,
  isMatchable,
  project,
  type RetirementProfile,
} from "./retirement-math";

const profile = (overrides: Partial<RetirementProfile> = {}): RetirementProfile => ({
  ...DEFAULT_PROFILE,
  current_age: 35,
  target_retirement_age: 65,
  gross_annual_income_cents: 16_500_000,
  ...overrides,
});

const summary = {
  fi_target: 120_000_000,
  fi_annual_spend: 4_800_000,
  invested_assets: 4_000_000,
  workplace: { rate: 0.02, annual_deferral: 330_000, match_earned: 165_000, match_available: 495_000 },
};

describe("isMatchable / isConfigured", () => {
  it("allows a match only on workplace plans", () => {
    expect(isMatchable("401k")).toBe(true);
    expect(isMatchable("roth_401k")).toBe(true);
    expect(isMatchable("ira")).toBe(false);
    expect(isMatchable(undefined)).toBe(false);
  });

  it("treats a profile with no age or income as never set up", () => {
    expect(isConfigured(DEFAULT_PROFILE)).toBe(false);
    expect(isConfigured(profile())).toBe(true);
  });
});

describe("computeInputs", () => {
  it("takes payroll saving from the server and adds only what is saved outside it", () => {
    const inputs = computeInputs(
      profile({
        accounts: [
          // Funded by the payroll rate, so its monthly figure is not added again.
          { account_id: "a", kind: "401k", monthly_contribution_cents: 50_000 },
          { account_id: "b", kind: "roth_ira", monthly_contribution_cents: 50_000 },
        ],
      }),
      summary,
    );

    expect(inputs.yearsToRetirement).toBe(30);
    expect(inputs.realReturn).toBeCloseTo(0.04, 12);
    expect(inputs.currentAssetsCents).toBe(4_000_000);
    expect(inputs.ownAnnualContributionCents).toBe(600_000 + 330_000);
    expect(inputs.employerMatchAnnualCents).toBe(165_000);
    expect(inputs.totalAnnualContributionCents).toBe(1_095_000);
    expect(inputs.targetNestEggCents).toBe(120_000_000);
    expect(inputs.targetAnnualSpendCents).toBe(4_800_000);
    expect(inputs.grossSavingsRate).toBeCloseTo(930_000 / 16_500_000, 12);
    expect(inputs.grossSavingsRateWithMatch).toBeCloseTo(1_095_000 / 16_500_000, 12);
  });

  it("is all zeros before the summary arrives", () => {
    const inputs = computeInputs(profile(), undefined);
    expect(inputs.currentAssetsCents).toBe(0);
    expect(inputs.totalAnnualContributionCents).toBe(0);
    expect(inputs.targetNestEggCents).toBe(0);
  });
});

describe("project", () => {
  it("adds contributions straight on at a zero real return", () => {
    const p = profile({ current_age: 55, expected_return_apr: 0.03, inflation_apr: 0.03 });
    const inputs = {
      ...computeInputs(p, undefined),
      currentAssetsCents: 1_000_000,
      ownAnnualContributionCents: 100_000,
      totalAnnualContributionCents: 100_000,
      targetNestEggCents: 3_000_000,
    };
    const result = project(p, inputs);

    expect(result.years).toHaveLength(11);
    expect(result.years[0]).toEqual({ year: 0, age: 55, balanceCents: 1_000_000 });
    expect(result.projectedCents).toBe(2_000_000);
    expect(result.gapCents).toBe(1_000_000);
    expect(result.onTrack).toBe(false);
    expect(result.fundedShare).toBeCloseTo(2 / 3, 12);
    // $10,000 short over ten years is $1,000 more a year on top of the $1,000.
    expect(result.requiredMonthlyCents).toBe(Math.ceil(200_000 / 12));
  });

  it("compounds at the real return", () => {
    const p = profile({ current_age: 45 }); // 20 years at 7% - 3%
    const inputs = {
      ...computeInputs(p, undefined),
      currentAssetsCents: 5_000_000,
      totalAnnualContributionCents: 1_200_000,
      targetNestEggCents: 1,
    };
    const r = 0.04;
    const n = 20;
    const closedForm = 5_000_000 * (1 + r) ** n + (1_200_000 * ((1 + r) ** n - 1)) / r;

    const result = project(p, inputs);
    expect(Math.abs(result.projectedCents - closedForm)).toBeLessThanOrEqual(1);
    expect(result.onTrack).toBe(true);
    expect(result.requiredMonthlyCents).toBeNull();
  });

  it("is never on track against no target", () => {
    const result = project(profile(), { ...computeInputs(profile(), undefined), targetNestEggCents: 0 });
    expect(result.onTrack).toBe(false);
    expect(result.fundedShare).toBe(0);
  });
});

describe("formatting", () => {
  it("formats percent and whole dollars", () => {
    expect(formatPct(0.2249, 2)).toBe("22.49%");
    expect(formatPct(0.06)).toBe("6%");
    expect(centsToDollars(123_456)).toBe("$1,235");
  });
});
