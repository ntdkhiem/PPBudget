/**
 * Every number the planning page asserts is computed here.
 *
 * This module is deliberately pure: no React, no fetching, no date-fns, no DOM.
 * That keeps the arithmetic auditable and testable with plain fixtures, and it
 * means a bug in the UI can never be a bug in the math.
 *
 * Money is always integer cents. Ratios are fractions (0.2), not percentages,
 * until the moment they are formatted.
 */

import type { PlanningAccount, PlanningBaseline, PlanningMonth } from "@/lib/api";

// ---------------------------------------------------------------- plan shape

export type Strategy = "aggressive" | "balanced" | "flexible";

export interface Goal {
  id: string;
  name: string;
  target_cents: number;
  target_date?: string; // yyyy-MM-dd
  linked_account_id?: string;
  current_cents?: number;
  priority: number;
}

export interface DebtInfo {
  account_id: string;
  apr: number; // annual rate as a fraction, e.g. 0.219
  min_payment_cents: number;
}

export interface FinancialPlan {
  version: 1;
  strategy: Strategy;
  overrides: {
    monthly_income_cents?: number;
    essential_expenses_cents?: number;
    liquid_account_ids?: string[];
  };
  emergency_fund: { target_months: number };
  goals: Goal[];
  debts: DebtInfo[];
  assumptions: { invest_return_apr: number };
}

export const DEFAULT_PLAN: FinancialPlan = {
  version: 1,
  strategy: "balanced",
  overrides: {},
  emergency_fund: { target_months: 6 },
  goals: [],
  debts: [],
  assumptions: { invest_return_apr: 0 },
};

/**
 * A strategy sets the savings rate you are aiming for. That single number
 * does the work a three-way percentage split used to: it sizes the waterfall's
 * target pool and gives the savings-rate trend something to be judged against.
 * The actual needs/wants breakdown already lives in Budgets, with real pacing.
 */
export const STRATEGY_PROFILES: Record<
  Strategy,
  { label: string; blurb: string; targetSavingsRate: number }
> = {
  aggressive: {
    label: "Aggressive",
    blurb: "Bank 30% of income. Wants get squeezed first.",
    targetSavingsRate: 0.3,
  },
  balanced: {
    label: "Balanced",
    blurb: "Bank 20%. The classic middle path.",
    targetSavingsRate: 0.2,
  },
  flexible: {
    label: "Flexible",
    blurb: "Bank 15%. More breathing room, slower build.",
    targetSavingsRate: 0.15,
  },
};

/** APR at or above this is paid down before funding the rest of the EF. */
export const HIGH_APR_THRESHOLD = 0.07;

// ------------------------------------------------------------------ baseline

export interface Baseline {
  /** Average monthly income across the window, excluding months with no data. */
  monthlyIncome: number;
  /** Average monthly total outflow. */
  monthlyOutflow: number;
  /** Average monthly 'needs' spend — the emergency-fund denominator. */
  essentialMonthly: number;
  monthlyWants: number;
  monthlySavingsSpend: number;
  monthlyUnbucketed: number;
  /** monthlyIncome - monthlyOutflow. May be negative. */
  monthlySurplus: number;
  liquidAssets: number;
  totalLiabilities: number; // negative
  netWorth: number;
  /** Share of outflow attributed to a bucket, 0..1. 1 means fully bucketed. */
  bucketCoverage: number;
  /** True when any override changed a figure away from its computed value. */
  usedOverrides: { income: boolean; essentials: boolean; liquid: boolean };
  monthsOfData: number;
}

function mean(values: number[]): number {
  if (values.length === 0) return 0;
  return Math.round(values.reduce((a, b) => a + b, 0) / values.length);
}

/**
 * Months with no transactions at all are excluded from the averages: a month
 * that has not happened yet (or predates the data) would otherwise drag every
 * average toward zero and overstate the emergency fund.
 */
export function activeMonths(months: PlanningMonth[]): PlanningMonth[] {
  return months.filter((m) => m.income !== 0 || m.outflow !== 0);
}

/**
 * The current month is partial, so including it understates spending. It is
 * dropped from averages whenever there is at least one complete month.
 */
export function completeMonths(months: PlanningMonth[]): PlanningMonth[] {
  const active = activeMonths(months);
  if (active.length <= 1) return active;
  return active.slice(0, -1);
}

export function computeBaseline(
  baseline: PlanningBaseline | undefined,
  plan: FinancialPlan,
): Baseline {
  const months = completeMonths(baseline?.months ?? []);

  const computedIncome = mean(months.map((m) => m.income));
  const computedEssentials = mean(months.map((m) => m.needs));
  const monthlyOutflow = mean(months.map((m) => m.outflow));

  const liquidIds = plan.overrides.liquid_account_ids;
  const computedLiquid = baseline?.liquid_assets ?? 0;
  const liquidAssets =
    liquidIds && liquidIds.length > 0
      ? (baseline?.accounts ?? [])
          .filter((a) => a.type === "asset" && liquidIds.includes(a.id))
          .reduce((sum, a) => sum + a.balance, 0)
      : computedLiquid;

  const monthlyIncome = plan.overrides.monthly_income_cents ?? computedIncome;
  const essentialMonthly = plan.overrides.essential_expenses_cents ?? computedEssentials;

  const totalOutflow = months.reduce((s, m) => s + m.outflow, 0);
  const totalUnbucketed = months.reduce((s, m) => s + m.unbucketed, 0);

  return {
    monthlyIncome,
    monthlyOutflow,
    essentialMonthly,
    monthlyWants: mean(months.map((m) => m.wants)),
    monthlySavingsSpend: mean(months.map((m) => m.savings)),
    monthlyUnbucketed: mean(months.map((m) => m.unbucketed)),
    monthlySurplus: monthlyIncome - monthlyOutflow,
    liquidAssets,
    totalLiabilities: baseline?.total_liabilities ?? 0,
    netWorth: baseline?.net_worth ?? 0,
    bucketCoverage: totalOutflow > 0 ? 1 - totalUnbucketed / totalOutflow : 1,
    usedOverrides: {
      income: plan.overrides.monthly_income_cents != null,
      essentials: plan.overrides.essential_expenses_cents != null,
      liquid: Boolean(liquidIds && liquidIds.length > 0),
    },
    monthsOfData: months.length,
  };
}

// ------------------------------------------------------------ health metrics

export type Health = "good" | "warn" | "bad" | "unknown";

export interface Metric {
  value: number | null;
  status: Health;
  /** Plain-English reading of the number. */
  note: string;
}

/** Months of essential spending covered by liquid assets. */
export function emergencyFundMonths(b: Baseline): number | null {
  if (b.essentialMonthly <= 0) return null;
  return b.liquidAssets / b.essentialMonthly;
}

export function emergencyFundMetric(b: Baseline, targetMonths: number): Metric {
  const months = emergencyFundMonths(b);
  if (months === null) {
    return {
      value: null,
      status: "unknown",
      note: "No essential spending recorded yet — budget some categories as 'needs' to unlock this.",
    };
  }
  const status: Health = months >= targetMonths ? "good" : months >= 3 ? "warn" : "bad";
  const note =
    months >= targetMonths
      ? `Fully funded — ${months.toFixed(1)} months covered against a ${targetMonths}-month target.`
      : `${months.toFixed(1)} of ${targetMonths} months covered.`;
  return { value: months, status, note };
}

/** Flags when bucket coverage is too thin to trust the essentials figure. */
export function coverageWarning(b: Baseline): string | null {
  if (b.monthsOfData === 0) return null;
  if (b.bucketCoverage >= 0.8) return null;
  return `Only ${formatPct(b.bucketCoverage)} of your spending sits in a budgeted category, so essentials are probably understated and these numbers flatter you.`;
}

// -------------------------------------------------------------- allocation

export type StageKind = "starter_ef" | "high_apr_debt" | "full_ef" | "invest";

export interface WaterfallStage {
  kind: StageKind;
  label: string;
  description: string;
  /** Cents per month flowing here while the stage is the active one. */
  monthlyCents: number;
  /** Total still needed to complete this stage, or null if open-ended. */
  remainingCents: number | null;
  /** Months of funding this stage itself needs, or null if it can never finish. */
  monthsToComplete: number | null;
  /** Months from today until this stage starts receiving money. */
  startsInMonths: number | null;
  /** True when this is the stage your money is going to right now. */
  active: boolean;
  /** True when the stage is already satisfied and takes no money. */
  complete: boolean;
  /** Set when the stage cannot be evaluated (e.g. no APR data). */
  blockedReason?: string;
}

export interface Waterfall {
  /** The pool being allocated each month. */
  poolCents: number;
  /** What the chosen strategy says the pool should be, for comparison. */
  targetPoolCents: number;
  stages: WaterfallStage[];
  /** Months until every funding stage completes and everything flows to investing. */
  crossoverMonths: number | null;
}

function monthsFor(remaining: number, monthly: number): number | null {
  if (remaining <= 0) return 0;
  if (monthly <= 0) return null;
  return Math.ceil(remaining / monthly);
}

/**
 * Routes the monthly surplus through funding priorities in strict order.
 *
 * A waterfall directs the whole flow at the topmost unfilled stage rather than
 * splitting it, so each stage's timeline starts only once the stages above it
 * finish. That sequencing is the point: it produces the crossover month, when
 * the cushion is done and the same dollars start being invested instead.
 *
 * The pool is actual surplus, not the strategy's target share — allocating
 * money you do not have would make every date below a fiction. The target is
 * returned alongside so the UI can show the gap.
 */
export function computeWaterfall(
  b: Baseline,
  plan: FinancialPlan,
  accounts: PlanningAccount[],
): Waterfall {
  const targetPoolCents = Math.round(
    b.monthlyIncome * STRATEGY_PROFILES[plan.strategy].targetSavingsRate,
  );
  const poolCents = Math.max(0, b.monthlySurplus);

  const starterTarget = b.essentialMonthly;
  const fullTarget = b.essentialMonthly * plan.emergency_fund.target_months;

  // The two EF stages partition one shortfall, so they never double-count.
  const starterGap = Math.max(0, starterTarget - b.liquidAssets);
  const totalEfGap = Math.max(0, fullTarget - b.liquidAssets);
  const fullGap = Math.max(0, totalEfGap - starterGap);

  const liabilityTotal = accounts
    .filter((a) => a.type === "liability")
    .reduce((s, a) => s + Math.abs(a.balance), 0);
  const highAprBalance = plan.debts
    .filter((d) => d.apr >= HIGH_APR_THRESHOLD)
    .reduce((sum, d) => {
      const acct = accounts.find((a) => a.id === d.account_id);
      return sum + (acct ? Math.abs(acct.balance) : 0);
    }, 0);

  const specs: Array<Omit<WaterfallStage, "monthlyCents" | "monthsToComplete" | "startsInMonths" | "active" | "complete">> = [
    {
      kind: "starter_ef",
      label: "Starter emergency fund",
      description: "One month of essentials, in cash",
      remainingCents: starterGap,
    },
    {
      kind: "high_apr_debt",
      label: "High-interest debt",
      description: `Balances above ${formatPct(HIGH_APR_THRESHOLD)} APR`,
      remainingCents: highAprBalance,
      blockedReason:
        plan.debts.length === 0 && liabilityTotal > 0
          ? "You have liabilities but no interest rates entered, so this stage is skipped."
          : undefined,
    },
    {
      kind: "full_ef",
      label: `Emergency fund to ${plan.emergency_fund.target_months} months`,
      description: "Topping the cushion up to your target",
      remainingCents: fullGap,
    },
  ];

  let elapsed: number | null = 0;
  let activeAssigned = false;

  const stages: WaterfallStage[] = specs.map((spec) => {
    const remaining = spec.remainingCents ?? 0;
    const complete = remaining <= 0;
    const months = monthsFor(remaining, poolCents);

    const startsInMonths = complete ? elapsed : elapsed;
    const active = !complete && !activeAssigned;
    if (active) activeAssigned = true;

    if (!complete) {
      elapsed = elapsed === null || months === null ? null : elapsed + months;
    }

    return {
      ...spec,
      monthlyCents: complete ? 0 : poolCents,
      monthsToComplete: complete ? 0 : months,
      startsInMonths,
      active,
      complete,
    };
  });

  const crossoverMonths = elapsed;

  stages.push({
    kind: "invest",
    label: "Invest the remainder",
    description: "Long-term money, once the cushion is in place",
    monthlyCents: poolCents,
    remainingCents: null,
    monthsToComplete: null,
    startsInMonths: crossoverMonths,
    active: !activeAssigned,
    complete: false,
  });

  return { poolCents, targetPoolCents, stages, crossoverMonths };
}

// ----------------------------------------------------------------- goals

export interface GoalProgress {
  id: string;
  name: string;
  currentCents: number;
  targetCents: number;
  /** 0..1, clamped. */
  fundedShare: number;
  /** Contribution per month needed to hit target_date, null when undated. */
  requiredMonthly: number | null;
  /** Months to completion at the planned contribution, null when unfunded. */
  monthsAtCurrent: number | null;
  /** True when the dated target cannot be met at the planned contribution. */
  offTrack: boolean;
  /** Months until this goal starts receiving money. Set by computeGoalPlan. */
  startsInMonths?: number | null;
  targetDate?: string;
  isEmergencyFund?: boolean;
}

/** Whole months from today until an ISO date. Negative when in the past. */
export function monthsUntil(isoDate: string, from: Date): number | null {
  const target = new Date(`${isoDate}T00:00:00Z`);
  if (Number.isNaN(target.getTime())) return null;
  const years = target.getUTCFullYear() - from.getUTCFullYear();
  const months = target.getUTCMonth() - from.getUTCMonth();
  return years * 12 + months;
}

export function computeGoalProgress(
  goal: Goal,
  accounts: PlanningAccount[],
  plannedMonthly: number,
  now: Date,
): GoalProgress {
  const linked = goal.linked_account_id
    ? accounts.find((a) => a.id === goal.linked_account_id)
    : undefined;
  const currentCents = linked ? linked.balance : (goal.current_cents ?? 0);
  const remaining = Math.max(0, goal.target_cents - currentCents);

  const monthsLeft = goal.target_date ? monthsUntil(goal.target_date, now) : null;
  const requiredMonthly =
    monthsLeft !== null && monthsLeft > 0 ? Math.ceil(remaining / monthsLeft) : null;
  const monthsAtCurrent = monthsFor(remaining, plannedMonthly);

  return {
    id: goal.id,
    name: goal.name,
    currentCents,
    targetCents: goal.target_cents,
    fundedShare: goal.target_cents > 0 ? Math.min(1, currentCents / goal.target_cents) : 0,
    requiredMonthly,
    monthsAtCurrent,
    offTrack:
      requiredMonthly !== null && remaining > 0 && (plannedMonthly <= 0 || plannedMonthly < requiredMonthly),
    targetDate: goal.target_date,
  };
}

/** The emergency fund is always present and always derived, never hand-entered. */
export function emergencyFundGoal(b: Baseline, plan: FinancialPlan): Goal {
  return {
    id: "__emergency_fund__",
    name: `Emergency fund (${plan.emergency_fund.target_months} months)`,
    target_cents: b.essentialMonthly * plan.emergency_fund.target_months,
    current_cents: b.liquidAssets,
    priority: 0,
  };
}

/**
 * Funds goals the same way the waterfall funds priorities: in order, each one
 * taking the whole pool once the goal above it completes.
 *
 * Splitting the pool evenly across goals — the obvious alternative — finishes
 * nothing for a long time and implies a schedule you would not actually
 * follow. Sequencing gives each goal a real start month.
 */
export function computeGoalPlan(
  goals: Goal[],
  accounts: PlanningAccount[],
  monthlyPool: number,
  /** Months before the pool is free, i.e. the waterfall's crossover. */
  availableInMonths: number | null,
  now: Date,
): GoalProgress[] {
  const ordered = [...goals].sort((a, c) => a.priority - c.priority);
  let elapsed: number | null = availableInMonths;

  return ordered.map((goal) => {
    const progress = computeGoalProgress(goal, accounts, monthlyPool, now);
    const startsInMonths = elapsed;

    // A goal that cannot finish stalls everything behind it, which is the
    // honest answer rather than quietly funding the next one anyway.
    if (progress.monthsAtCurrent === null) {
      elapsed = null;
    } else if (elapsed !== null) {
      elapsed = elapsed + progress.monthsAtCurrent;
    }

    return {
      ...progress,
      startsInMonths,
      // Reaching a dated goal depends on when funding actually starts.
      offTrack:
        progress.requiredMonthly !== null &&
        progress.targetCents > progress.currentCents &&
        (monthlyPool <= 0 ||
          startsInMonths === null ||
          progress.monthsAtCurrent === null ||
          startsInMonths + progress.monthsAtCurrent >
            (goal.target_date ? (monthsUntil(goal.target_date, now) ?? 0) : Infinity)),
    };
  });
}

// -------------------------------------------------------------- projection

export interface ScenarioInput {
  /** Extra or reduced monthly contribution, in cents. */
  monthlyContributionCents: number;
  /** Fractional change to income, e.g. 0.1 for a 10% raise. */
  incomeChange: number;
  /** Fractional cut to discretionary spending, e.g. 0.3 for 30% less. */
  spendingCut: number;
}

export interface ProjectionPoint {
  monthIndex: number;
  baselineNetWorth: number;
  scenarioNetWorth: number;
  baselineEfMonths: number | null;
  scenarioEfMonths: number | null;
}

export interface Projection {
  points: ProjectionPoint[];
  baselineEfFundedMonth: number | null;
  scenarioEfFundedMonth: number | null;
  baselineFinalNetWorth: number;
  scenarioFinalNetWorth: number;
}

/**
 * Projects net worth and emergency-fund coverage forward month by month.
 *
 * Deliberately simple: a monthly surplus added to net worth, compounded at the
 * stated return. It does not model taxes, irregular income, or market variance,
 * which is exactly why the UI prints the assumption next to the chart.
 */
export function project(
  b: Baseline,
  plan: FinancialPlan,
  scenario: ScenarioInput,
  horizonMonths: number,
): Projection {
  const monthlyReturn = plan.assumptions.invest_return_apr / 12;
  const efTarget = b.essentialMonthly * plan.emergency_fund.target_months;

  const scenarioIncome = b.monthlyIncome * (1 + scenario.incomeChange);
  const discretionary = b.monthlyWants + b.monthlyUnbucketed;
  const scenarioOutflow = b.monthlyOutflow - discretionary * scenario.spendingCut;
  const scenarioSurplus =
    scenarioIncome - scenarioOutflow + scenario.monthlyContributionCents;

  let baselineNw = b.netWorth;
  let scenarioNw = b.netWorth;
  let baselineLiquid = b.liquidAssets;
  let scenarioLiquid = b.liquidAssets;

  let baselineEfFundedMonth: number | null = null;
  let scenarioEfFundedMonth: number | null = null;

  const points: ProjectionPoint[] = [];

  for (let i = 0; i <= horizonMonths; i++) {
    if (i > 0) {
      baselineNw = Math.round(baselineNw * (1 + monthlyReturn)) + b.monthlySurplus;
      scenarioNw = Math.round(scenarioNw * (1 + monthlyReturn)) + Math.round(scenarioSurplus);
      baselineLiquid += Math.max(0, b.monthlySurplus);
      scenarioLiquid += Math.max(0, Math.round(scenarioSurplus));
    }

    const baselineEfMonths = b.essentialMonthly > 0 ? baselineLiquid / b.essentialMonthly : null;
    const scenarioEfMonths = b.essentialMonthly > 0 ? scenarioLiquid / b.essentialMonthly : null;

    if (baselineEfFundedMonth === null && efTarget > 0 && baselineLiquid >= efTarget) {
      baselineEfFundedMonth = i;
    }
    if (scenarioEfFundedMonth === null && efTarget > 0 && scenarioLiquid >= efTarget) {
      scenarioEfFundedMonth = i;
    }

    points.push({
      monthIndex: i,
      baselineNetWorth: baselineNw,
      scenarioNetWorth: scenarioNw,
      baselineEfMonths,
      scenarioEfMonths,
    });
  }

  return {
    points,
    baselineEfFundedMonth,
    scenarioEfFundedMonth,
    baselineFinalNetWorth: baselineNw,
    scenarioFinalNetWorth: scenarioNw,
  };
}

// ------------------------------------------------------------------ trends

export type TrendKey = "income" | "essentials" | "discretionary" | "savingsRate";

export interface Trend {
  key: TrendKey;
  label: string;
  /** Latest complete month: cents, or a 0..1 fraction for savingsRate. */
  current: number;
  /** Fractional change between the two halves of the window. */
  change: number;
  direction: "up" | "down" | "flat";
  /** Whether this direction of travel is a good thing for this metric. */
  good: boolean;
  /** Too little history to claim a direction. */
  unknown: boolean;
}

/** Movements smaller than this are noise, not a trend. */
const TREND_FLAT_BAND = 0.02;

/** Fewer than this many complete months and two points would be masquerading as a trend. */
const TREND_MIN_MONTHS = 4;

/**
 * Direction of travel, comparing the first half of the window against the
 * second. Averaging halves rather than differencing the endpoints keeps one
 * unusual month from inventing a trend.
 *
 * This is the signal averaging destroys: a page that only reports a mean
 * cannot tell you your discretionary spending is climbing.
 */
export function computeTrends(months: PlanningMonth[]): Trend[] {
  const complete = completeMonths(months);

  const specs: {
    key: TrendKey;
    label: string;
    /** Higher is better? */
    upIsGood: boolean;
    value: (m: PlanningMonth) => number;
  }[] = [
    { key: "income", label: "Income", upIsGood: true, value: (m) => m.income },
    { key: "essentials", label: "Essentials", upIsGood: false, value: (m) => m.needs },
    {
      key: "discretionary",
      label: "Discretionary",
      upIsGood: false,
      value: (m) => m.wants + m.unbucketed,
    },
    {
      key: "savingsRate",
      label: "Savings rate",
      upIsGood: true,
      value: (m) => (m.income > 0 ? (m.income - m.outflow) / m.income : 0),
    },
  ];

  return specs.map((spec) => {
    const series = complete.map(spec.value);
    const current = series.length > 0 ? series[series.length - 1] : 0;

    if (series.length < TREND_MIN_MONTHS) {
      return {
        key: spec.key,
        label: spec.label,
        current,
        change: 0,
        direction: "flat" as const,
        good: true,
        unknown: true,
      };
    }

    const mid = Math.floor(series.length / 2);
    const first = series.slice(0, mid);
    const second = series.slice(mid);
    const avg = (xs: number[]) => xs.reduce((a, b) => a + b, 0) / xs.length;
    const firstAvg = avg(first);
    const secondAvg = avg(second);

    // A rate can legitimately sit at or below zero, so a plain ratio would
    // blow up or flip sign; compare rates in points instead.
    const change =
      spec.key === "savingsRate"
        ? secondAvg - firstAvg
        : firstAvg !== 0
          ? (secondAvg - firstAvg) / Math.abs(firstAvg)
          : 0;

    const direction =
      Math.abs(change) < TREND_FLAT_BAND ? "flat" : change > 0 ? "up" : "down";

    return {
      key: spec.key,
      label: spec.label,
      current,
      change,
      direction,
      good: direction === "flat" ? true : (direction === "up") === spec.upIsGood,
      unknown: false,
    };
  });
}

// ----------------------------------------------------------------- variance

export interface Variance {
  /** Highest essential spend in any complete month. */
  worstEssentials: number;
  /** Median essential spend — the typical month. */
  typicalEssentials: number;
  /** What the emergency fund target becomes if sized on the worst month. */
  stressedTargetCents: number;
  /** Months of cover measured against worst-month essentials. */
  stressedMonths: number | null;
  /** Cash still needed to reach the stressed target. */
  shortfallVsStressed: number;
  unknown: boolean;
}

/**
 * An emergency fund sized on an average month is sized for the wrong month —
 * the fund exists for the bad one. This restates the target against the worst
 * month actually observed.
 */
export function computeVariance(
  b: Baseline,
  plan: FinancialPlan,
  months: PlanningMonth[],
): Variance {
  const complete = completeMonths(months);
  const essentials = complete.map((m) => m.needs).filter((n) => n > 0);

  if (essentials.length === 0) {
    return {
      worstEssentials: 0,
      typicalEssentials: 0,
      stressedTargetCents: 0,
      stressedMonths: null,
      shortfallVsStressed: 0,
      unknown: true,
    };
  }

  const sorted = [...essentials].sort((x, y) => x - y);
  const mid = Math.floor(sorted.length / 2);
  const typical =
    sorted.length % 2 === 0 ? Math.round((sorted[mid - 1] + sorted[mid]) / 2) : sorted[mid];
  const worst = sorted[sorted.length - 1];

  const stressedTargetCents = worst * plan.emergency_fund.target_months;

  return {
    worstEssentials: worst,
    typicalEssentials: typical,
    stressedTargetCents,
    stressedMonths: worst > 0 ? b.liquidAssets / worst : null,
    shortfallVsStressed: Math.max(0, stressedTargetCents - b.liquidAssets),
    unknown: essentials.length < 2,
  };
}

// ------------------------------------------------------------ movable money

export interface Movable {
  /** Essentials — spending you cannot realistically move. */
  committedCents: number;
  /** Wants — spending you can. */
  discretionaryCents: number;
  /** Outflow in categories with no budget, so genuinely unknown either way. */
  unclassifiedCents: number;
  /**
   * Outflow already headed into savings or investments. Not committed and not
   * worth cutting — it is the destination, not the leak — but it is part of
   * total outflow, so it must be named or the figures silently fail to add up.
   */
  alreadySavingCents: number;
  outflowCents: number;
  /** Lower bound: wants only. */
  movableShare: number;
  /** Upper bound: wants plus everything unclassified. */
  maxMovableShare: number;
}

/**
 * How much of the monthly outflow is actually yours to redirect.
 *
 * Unclassified spending is deliberately kept as its own third category rather
 * than folded into discretionary: assuming it is all movable would overstate
 * your freedom, and assuming none of it is would understate it. The result is
 * a range whenever that bucket is material.
 */
export function computeMovable(b: Baseline): Movable {
  const outflow = b.monthlyOutflow;
  const discretionary = b.monthlyWants;
  const unclassified = b.monthlyUnbucketed;

  return {
    committedCents: b.essentialMonthly,
    discretionaryCents: discretionary,
    unclassifiedCents: unclassified,
    alreadySavingCents: b.monthlySavingsSpend,
    outflowCents: outflow,
    movableShare: outflow > 0 ? discretionary / outflow : 0,
    maxMovableShare: outflow > 0 ? (discretionary + unclassified) / outflow : 0,
  };
}

// ------------------------------------------------------------------ headline

export type HeadlineKind =
  | "no_essentials"
  | "no_surplus"
  | "missing_apr"
  | "starter_ef"
  | "debt"
  | "full_ef"
  | "invest";

export interface Headline {
  kind: HeadlineKind;
  /** The imperative: what to do next. */
  action: string;
  /** One sentence of justification, carrying the numbers. */
  detail: string;
  tone: Health;
  href?: string;
  hrefLabel?: string;
}

/**
 * The single sentence this page exists to produce.
 *
 * Resolved in priority order, and the data-quality cases come first on
 * purpose: when the page cannot support advice, it should say how to fix that
 * rather than advise anyway. A 22% card the page does not know about outranks
 * every recommendation it would otherwise make.
 */
export function computeHeadline(
  b: Baseline,
  plan: FinancialPlan,
  waterfall: Waterfall,
  accounts: PlanningAccount[],
): Headline {
  if (b.essentialMonthly <= 0) {
    return {
      kind: "no_essentials",
      action: "Give your categories budget buckets",
      detail:
        "Nothing here is measurable until some spending is marked as essential. Tag your categories as needs, wants or savings and this page can start advising you.",
      tone: "unknown",
      href: "/budgets",
      hrefLabel: "Open Budgets",
    };
  }

  const liabilities = accounts
    .filter((a) => a.type === "liability")
    .reduce((sum, a) => sum + Math.abs(a.balance), 0);

  if (liabilities > 0 && plan.debts.length === 0) {
    return {
      kind: "missing_apr",
      action: "Enter your interest rates",
      detail: `You owe ${centsToDollars(liabilities)} but PPBudget does not store APRs. A high-rate balance would outrank everything else below, so this plan is incomplete until you add them.`,
      tone: "warn",
    };
  }

  if (b.monthlySurplus <= 0) {
    return {
      kind: "no_surplus",
      action: "Close the gap between income and spending",
      detail: `You are spending ${centsToDollars(Math.abs(b.monthlySurplus))} a month more than you earn. No funding priority below can make progress until that reverses.`,
      tone: "bad",
    };
  }

  const active = waterfall.stages.find((s) => s.active);
  const pool = centsToDollars(waterfall.poolCents);

  switch (active?.kind) {
    case "starter_ef":
      return {
        kind: "starter_ef",
        action: "Build a one-month cushion",
        detail: `Put ${pool} a month toward ${centsToDollars(active.remainingCents ?? 0)} of starter savings — about ${formatMonths(active.monthsToComplete)} away.`,
        tone: "warn",
      };

    case "high_apr_debt": {
      const worst = [...plan.debts]
        .filter((d) => d.apr >= HIGH_APR_THRESHOLD)
        .sort((x, y) => y.apr - x.apr)[0];
      const name = accounts.find((a) => a.id === worst?.account_id)?.name ?? "your high-rate debt";
      return {
        kind: "debt",
        action: `Clear ${name}`,
        detail: `${centsToDollars(active.remainingCents ?? 0)} at ${formatPct(worst?.apr ?? 0, 1)} APR costs you more than savings can earn. At ${pool} a month it is gone in ${formatMonths(active.monthsToComplete)}.`,
        tone: "bad",
      };
    }

    case "full_ef":
      return {
        kind: "full_ef",
        action: `Top up your emergency fund`,
        detail: `${centsToDollars(active.remainingCents ?? 0)} short of ${plan.emergency_fund.target_months} months. At ${pool} a month you get there in ${formatMonths(active.monthsToComplete)}, then this money starts being invested.`,
        tone: "warn",
      };

    default:
      return {
        kind: "invest",
        action: "Put your surplus to work",
        detail: `Your cushion is complete and no high-rate debt is outstanding, so the full ${pool} a month can be invested.`,
        tone: "good",
      };
  }
}

// ------------------------------------------------------------------ format

/** Whole-dollar rendering for prose, where cents would be noise. */
function centsToDollars(cents: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(Math.round(cents / 100));
}

export function formatPct(fraction: number, digits = 0): string {
  return `${(fraction * 100).toFixed(digits)}%`;
}

/** Signed percentage, for trend deltas. */
export function formatSignedPct(fraction: number, digits = 0): string {
  const sign = fraction > 0 ? "+" : "";
  return `${sign}${(fraction * 100).toFixed(digits)}%`;
}

/** "3 months" / "1 yr 2 mo" / "—" */
export function formatMonths(months: number | null): string {
  if (months === null) return "—";
  if (months <= 0) return "now";
  if (months < 12) return `${months} mo`;
  const years = Math.floor(months / 12);
  const rest = months % 12;
  return rest === 0 ? `${years} yr` : `${years} yr ${rest} mo`;
}
