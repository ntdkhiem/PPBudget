"use client";

import type {
  Baseline,
  FinancialPlan,
  Headline,
  Health,
  Strategy,
  Waterfall,
  WaterfallStage,
} from "../cash/_components/planning-math";
import type { PlanningAccount } from "@/lib/api";
import type { Flag, RetirementProfile } from "../retirement/_components/retirement-math";
import type {
  Goal,
  PlanSummary,
  Quest,
  RetirementAccountTerms,
  WealthProfile,
} from "./wealth-data";

/**
 * Adapters from the server's plan onto the shapes the Cash page already draws.
 *
 * The DECISIONS moved to Go -- which actions exist, their order, what unlocks a
 * phase, the dollar figures. What stayed in the browser is rendering, and the
 * components that do it are fine. So rather than rewrite a thousand lines of
 * working display code to change the shape of its props, this maps one onto the
 * other.
 *
 * The types below therefore survive `computeWaterfall` and `computeHeadline`,
 * but their meaning has changed: they are now view models built from an answer
 * the server gave, not an answer the browser worked out for itself. That
 * distinction is the whole point of the migration -- two implementations of the
 * same sequencing, one in each language, is how a user ends up seeing two
 * different dates for one milestone.
 */

/** Phase 4 in the Go catalog: investment allocation. */
const PHASE_ALLOCATION = 4;

/**
 * The profile, shaped as the plan object the components expect.
 *
 * Only the fields they actually read are populated; the rest carry defaults
 * that keep the types honest without pretending to data we no longer store
 * this way (debts live in account_terms now, and nothing here reads them).
 */
export function toFinancialPlan(
  profile: WealthProfile | undefined,
  goals: Goal[] | undefined,
): FinancialPlan {
  return {
    version: 1,
    strategy: (profile?.strategy as Strategy) ?? "balanced",
    overrides: {
      monthly_income_cents: profile?.override_monthly_income,
      essential_expenses_cents: profile?.override_essential_expenses,
    },
    emergency_fund: { target_months: profile?.emergency_fund_target_months ?? 6 },
    goals: (goals ?? []).map((g) => ({
      id: g.id,
      name: g.name,
      target_cents: g.target_amount,
      target_date: g.target_date ? g.target_date.slice(0, 10) : undefined,
      linked_account_id: g.linked_account_id,
      current_cents: g.current_amount,
      priority: g.priority,
    })),
    debts: [],
    assumptions: { invest_return_apr: profile?.expected_return_apr ?? 0 },
  };
}

/** The summary, shaped as the baseline the components expect. */
export function toBaseline(
  summary: PlanSummary | undefined,
  profile: WealthProfile | undefined,
): Baseline {
  const s = summary;
  return {
    monthlyIncome: s?.monthly_income ?? 0,
    monthlyOutflow: s?.monthly_outflow ?? 0,
    essentialMonthly: s?.essential_monthly ?? 0,
    monthlyWants: s?.monthly_wants ?? 0,
    monthlySavingsSpend: s?.monthly_savings ?? 0,
    monthlyUnbucketed: s?.monthly_unbucketed ?? 0,
    monthlySurplus: s?.monthly_surplus ?? 0,
    liquidAssets: s?.liquid_assets ?? 0,
    totalLiabilities: s?.total_liabilities ?? 0,
    netWorth: s?.net_worth ?? 0,
    bucketCoverage: s?.bucket_coverage ?? 1,
    usedOverrides: {
      income: profile?.override_monthly_income != null,
      essentials: profile?.override_essential_expenses != null,
    },
    monthsOfData: s?.months_of_data ?? 0,
  };
}

/**
 * Phase 1 and the investing stage, shaped as the waterfall the allocation
 * section draws.
 *
 * The schedule comes from the server whole -- which actions the surplus pays
 * for, what each still needs, when each starts -- so the stages here cannot
 * disagree with the crossover beside them. Actions the surplus does not pay
 * for (the employer match, the spending cap, moving idle cash) are left to the
 * action list rather than drawn as stages that take months.
 */
export function toWaterfall(
  quests: Quest[] | undefined,
  summary: PlanSummary | undefined,
): Waterfall {
  const pool = Math.max(0, summary?.monthly_surplus ?? 0);
  const crossover = summary?.crossover_months ?? null;
  const byId = new Map((quests ?? []).map((q) => [q.id, q]));

  let activeAssigned = false;
  const stages: WaterfallStage[] = [];

  for (const f of summary?.funding ?? []) {
    const q = byId.get(f.quest_id);
    if (!q) continue;

    const complete = q.status === "complete" || q.status === "skipped";
    const active = !complete && q.status === "available" && f.remaining > 0 && !activeAssigned;
    if (active) activeAssigned = true;

    stages.push({
      id: q.id,
      kind: stageKindFor(q.catalog_key),
      label: q.title,
      description: q.detail ?? "",
      monthlyCents: complete ? 0 : pool,
      remainingCents: complete ? 0 : f.remaining,
      monthsToComplete: f.months_to_complete,
      startsInMonths: f.starts_in_months,
      completesOn: f.completes_on ?? null,
      active,
      complete,
      // A blocked action cannot be sized, and saying why is more use than
      // showing it as a stage with no number.
      blockedReason:
        q.status === "blocked"
          ? "Needs an answer before this can be worked out."
          : q.status === "locked"
            ? "Comes after the stages above."
            : undefined,
    });
  }

  stages.push({
    id: "invest",
    kind: "invest",
    label: "Invest the remainder",
    description: "Long-term money, once the cushion is in place",
    monthlyCents: pool,
    remainingCents: null,
    monthsToComplete: null,
    startsInMonths: crossover,
    completesOn: null,
    active: !activeAssigned,
    complete: false,
  });

  return {
    poolCents: pool,
    targetPoolCents: Math.round((summary?.monthly_income ?? 0) * (summary?.target_savings_rate ?? 0)),
    stages,
    crossoverMonths: crossover,
    crossoverOn: summary?.crossover_on ?? null,
  };
}

/**
 * Maps a catalog key onto the stage kind the component styles by.
 *
 * Only the funding actions in the server's schedule arrive here. Anything
 * unrecognised falls back to the emergency-fund styling rather than being
 * dropped -- a stage the user cannot see is worse than one that is the wrong
 * colour.
 */
function stageKindFor(catalogKey: string): WaterfallStage["kind"] {
  const base = catalogKey.split(":")[0];
  switch (base) {
    case "starter_emergency_fund":
      return "starter_ef";
    case "clear_high_apr_balance":
    case "clear_promo_balance_before_expiry":
      return "high_apr_debt";
    case "full_emergency_fund":
      return "full_ef";
    default:
      return "full_ef";
  }
}

/**
 * The single "do this next" sentence, taken from the server's own ordering.
 *
 * Previously resolved in the browser through a seven-case priority ladder that
 * had to be kept in step with the waterfall beside it. Now it is simply the
 * first action the engine put at the top, which cannot disagree with the list
 * below it because it IS the list below it.
 */
export function toHeadline(
  quests: Quest[] | undefined,
  summary: PlanSummary | undefined,
): Headline {
  const live = (quests ?? []).filter(
    (q) => q.status !== "not_applicable" && q.status !== "skipped",
  );

  if (live.length === 0) {
    return {
      kind: "no_essentials",
      action: "Answer a few questions to get your plan",
      detail: "There is nothing to work from yet.",
      tone: "unknown",
      href: "/wealth/profile",
      hrefLabel: "Start",
    };
  }

  // A deadline leads only when it is CLOSE. The engine's own ordering already
  // encodes what matters most -- employer match at priority 0, ahead of even a
  // 22% balance -- and a 31 December date seen in September is not a reason to
  // displace it. Letting any dated action jump the queue put a routine year-end
  // contribution above the single highest-return action in the plan.
  const URGENT_DAYS = 45;
  const dated = live
    .filter((q) => {
      if (!q.due_date || q.status !== "available") return false;
      const days = (new Date(q.due_date).getTime() - Date.now()) / 86_400_000;
      return days <= URGENT_DAYS;
    })
    .sort((a, b) => (a.due_date ?? "").localeCompare(b.due_date ?? ""));

  // Otherwise the engine's order stands: quests arrive sorted by phase then
  // priority, so the first available one is the one it chose.
  const available = live.filter((q) => q.status === "available");
  const blocked = live.filter((q) => q.status === "blocked");

  const top = dated[0] ?? available[0] ?? blocked[0];
  if (!top) {
    return {
      kind: "invest",
      action: "Everything on your list is done",
      detail: "Nothing is outstanding. New actions appear as your situation changes.",
      tone: "good",
    };
  }

  const tone: Health =
    top.status === "blocked" ? "warn" : (summary?.monthly_surplus ?? 0) <= 0 ? "bad" : "good";

  return {
    kind: top.phase === PHASE_ALLOCATION ? "invest" : "starter_ef",
    action: top.title,
    detail: top.detail ?? "",
    tone,
    href: top.status === "blocked" ? "/wealth/profile" : undefined,
    hrefLabel: top.status === "blocked" ? "Answer what it needs" : undefined,
  };
}

// ------------------------------------------------------------- retirement


/**
 * The profile and its investment accounts, shaped as the retirement profile
 * the projection components expect.
 *
 * The accounts are every one whose role is investment, with its tax treatment
 * and contribution where the user has set them. An account with no treatment
 * yet still counts: its balance is invested money whatever it is called.
 *
 * Age is derived from date of birth rather than stored, which is the point of
 * having replaced one with the other: an age is correct for exactly one year
 * and then silently wrong.
 */
export function toRetirementProfile(
  profile: WealthProfile | undefined,
  terms: RetirementAccountTerms[] | undefined,
  accounts: PlanningAccount[] | undefined,
): RetirementProfile {
  const termsFor = new Map((terms ?? []).map((t) => [t.account_id, t]));
  const currentAge = profile?.date_of_birth
    ? ageFrom(profile.date_of_birth)
    : 0;

  return {
    version: 1,
    current_age: currentAge,
    target_retirement_age: profile?.target_independence_age ?? 65,
    gross_annual_income_cents: profile?.gross_annual_income ?? 0,
    expected_return_apr: profile?.expected_return_apr ?? 0.07,
    inflation_apr: profile?.inflation_apr ?? 0.03,
    withdrawal_rate: profile?.withdrawal_rate ?? 0.04,
    target_annual_spend_cents: profile?.target_annual_spend,
    accounts: (accounts ?? [])
      .filter((a) => a.role === "investment")
      .map((a) => {
        const t = termsFor.get(a.id);
        return {
          account_id: a.id,
          kind: t?.kind,
          monthly_contribution_cents: t?.monthly_contribution ?? 0,
        };
      }),
  };
}

/** Whole years since a date of birth. */
function ageFrom(iso: string): number {
  const dob = new Date(iso);
  if (Number.isNaN(dob.getTime())) return 0;
  const now = new Date();
  let age = now.getUTCFullYear() - dob.getUTCFullYear();
  const beforeBirthday =
    now.getUTCMonth() < dob.getUTCMonth() ||
    (now.getUTCMonth() === dob.getUTCMonth() && now.getUTCDate() < dob.getUTCDate());
  if (beforeBirthday) age--;
  return Math.max(0, age);
}

/**
 * Warnings, taken from the engine's own actions rather than recomputed.
 *
 * `computeFlags` used to work this out in the browser and had the distinction
 * of being the only thing in the old app that caught unclaimed employer match
 * — which the source strategy document missed entirely. That check now lives
 * in the catalog, where it is ordered against everything else and tested; this
 * just surfaces the result in the shape this page draws.
 */
export function toRetirementFlags(
  quests: Quest[] | undefined,
  accounts: RetirementProfile["accounts"],
): Flag[] {
  const out: Flag[] = [];
  const live = quests ?? [];

  const isOpen = (key: string) =>
    live.some(
      (q) =>
        q.catalog_key === key && (q.status === "available" || q.status === "blocked"),
    );
  const detailOf = (key: string) =>
    live.find((q) => q.catalog_key === key)?.detail ?? "";

  if (accounts.length === 0) {
    out.push({
      kind: "no_accounts",
      severity: "warn",
      title: "No investment accounts yet",
      detail:
        "Mark your 401(k), IRA, HSA and brokerage accounts as “Investment or retirement” on the Accounts page, so their balances count here.",
    });
  }

  if (isOpen("capture_employer_match")) {
    const q = live.find((x) => x.catalog_key === "capture_employer_match");
    out.push({
      kind: "unclaimed_match",
      severity: "bad",
      title: q?.title ?? "You are leaving employer match behind",
      detail: detailOf("capture_employer_match"),
    });
  }

  const hsa = live.find((q) => q.catalog_key === "max_hsa");
  if (hsa?.variant === "over_limit") {
    out.push({
      kind: "over_limit",
      severity: "warn",
      title: hsa.title,
      detail: hsa.detail ?? "",
    });
  }

  return out;
}
