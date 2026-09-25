"use client";

import { useCallback, useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch, getStoredToken } from "@/lib/api";

/**
 * Data layer for the Wealth Strategy pages.
 *
 * Unlike planning-data.ts and retirement-data.ts, which this replaces, nothing
 * here parses a JSON blob out of a free-form settings column. The server owns
 * the profile in typed, constrained columns and owns the action list outright,
 * so the client's job is to render what it is given rather than to recompute
 * it. No zod schema is needed to defend against a malformed blob, because the
 * only writer is an endpoint that validates.
 *
 * Money is integer cents throughout, matching the Go side.
 */

// ---------------------------------------------------------------- types

/**
 * Where an answer came from. The distinction is load-bearing rather than
 * cosmetic: the generator refuses to run high-stakes branches (the backdoor
 * Roth decision above all) off a value the user never actually looked at, so
 * confirming a derived figure is a real action with real consequences.
 */
export type FieldSource = "derived" | "confirmed" | "entered" | "default";

export interface ProfileField {
  field_key: string;
  source: FieldSource;
  answered_at: string;
}

/**
 * Every answer is optional. Absent means "not asked yet", which is what drives
 * progressive intake -- it is never the same as zero.
 */
export interface WealthProfile {
  user_id: string;

  date_of_birth?: string;
  marital_status?: string;
  filing_status?: string;
  resident_state?: string;
  spouse_gross_annual?: number;
  spouse_has_workplace_plan?: boolean;
  target_independence_age?: number;

  gross_annual_income?: number;

  deferral_pct?: number;
  deferral_roth_share?: number;
  match_pct?: number;
  match_limit_pct?: number;
  match_per_paycheck?: boolean;
  match_has_true_up?: boolean;
  plan_allows_after_tax?: boolean;
  plan_allows_in_service?: boolean;
  plan_accepts_rollovers?: boolean;

  hdhp_enrolled?: boolean;
  hsa_coverage_tier?: string;
  hsa_employer_contribution?: number;

  prior_year_tax_liability?: number;
  prior_year_agi?: number;
  annual_bonus?: number;
  bonus_month?: number;

  emergency_fund_target_months?: number;

  /** How hard the user wants to save; sets the target rate comparisons use. */
  strategy?: "aggressive" | "balanced" | "flexible";

  /**
   * The user telling us our arithmetic is wrong about them. Kept apart from the
   * derived figure rather than replacing it, so the app can still say which
   * numbers they corrected. Absent means no override, never an override of zero.
   * Which accounts count as cash is each account's role, not an override.
   */
  override_monthly_income?: number;
  override_essential_expenses?: number;

  traditional_ira_balance?: number;
  taxable_brokerage_value?: number;
  taxable_unrealized_gain?: number;
  taxable_employer_stock?: number;

  employer_is_public?: boolean;
  employer_ticker?: string;
  has_stock_options?: boolean;

  ltd_replacement_pct?: number;
  ltd_monthly_cap?: number;
  ltd_premium_pretax?: boolean;
  life_death_benefit?: number;

  housing_tenure?: string;
  mortgage_apr?: number;
  mortgage_balance?: number;
  pays_pmi?: boolean;
  student_loan_kind?: string;
  student_loan_idr?: boolean;
  student_loan_pslf?: boolean;

  expected_return_apr?: number;
  inflation_apr?: number;
  withdrawal_rate?: number;
  target_annual_spend?: number;

  /** Provenance per answered field. Absence of a key means unanswered. */
  fields?: Record<string, ProfileField>;
}

export type QuestStatus =
  | "locked"
  | "blocked"
  | "available"
  | "complete"
  | "skipped"
  | "not_applicable";

export interface Quest {
  /** The catalog key. Actions are worked out on each read, not stored rows. */
  id: string;
  catalog_key: string;
  phase: number;
  priority: number;
  status: QuestStatus;
  title: string;
  detail?: string;
  /** Which outcome this is, where one action can say different things. Branch on this, not the title. */
  variant?: "over_limit";
  target_amount?: number;
  /** An action with a due date surfaces even while its phase is locked. */
  due_date?: string;
  verification: "auto" | "manual";
  stale: boolean;
  /**
   * Answers that would move this action out of 'blocked'. Mostly profile field
   * keys, but may also name data rather than a question -- `transaction_history`,
   * `paystub_ytd`, `account_apr`, `espp_terms`. Check against the field
   * registry to tell them apart.
   */
  missing_fields: string[];
  completed_at?: string;
  completed_source?: "auto" | "manual" | "system";
  generated_at: string;
}

/** One condition for finishing a phase, stated as what it checks. */
export interface Milestone {
  key: string;
  label: string;
  done: boolean;
  /** False when there was nothing to do at all -- no card, no equity. */
  applies: boolean;
}

export interface Phase {
  number: number;
  name: string;
  /** Whether every earlier phase is finished. */
  unlocked: boolean;
  /** What finishes this phase, and so unlocks the next. */
  milestones: Milestone[];
}

/** What goes into the workplace plan through payroll, and the match it earns. */
export interface WorkplaceSaving {
  rate: number;
  annual_deferral: number;
  match_earned: number;
  match_available: number;
}

/** This tax year's contribution ceilings for the user, from the tax table. */
export interface ContributionLimits {
  tax_year: number;
  workplace: number;
  ira: number;
  /** Absent until the HSA coverage tier is known. */
  hsa?: number;
}

/**
 * One phase 1 action the monthly surplus pays for, in the order it is paid.
 * The last stage ends at `crossover_months`.
 *
 * Worked out month by month on the server, charging a card its interest for as
 * long as it waits, so the months here are not remaining / surplus.
 */
export interface FundingStage {
  quest_id: string;
  /** What the surplus still has to put in; 0 once done or when there is no figure. */
  remaining: number;
  /** Null when the surplus never finishes this stage, or nothing remains. */
  starts_in_months: number | null;
  /** Null means "never at this rate". */
  months_to_complete: number | null;
  /** First day of the month it finishes, in UTC; null when it never does or nothing remains. */
  completes_on: string | null;
}

/**
 * The figures every wealth surface needs in common, served alongside the action
 * list so the client stops computing a baseline of its own.
 */
export interface PlanSummary {
  monthly_income: number;
  monthly_outflow: number;
  essential_monthly: number;
  monthly_wants: number;
  monthly_unbucketed: number;
  monthly_savings: number;
  monthly_surplus: number;
  liquid_assets: number;
  total_liabilities: number;
  net_worth: number;
  bucket_coverage: number;
  months_of_data: number;
  target_savings_rate: number;
  /** Months until phase 1 is funded. Null means "never at this rate". */
  crossover_months: number | null;
  /** First day of that month, in UTC; null when never, or when there is nothing to fund. */
  crossover_on: string | null;
  funding: FundingStage[];
  /** The independence number, built on this much spending a year. 0 without spending. */
  fi_target: number;
  fi_annual_spend: number;
  /** What counts toward it: accounts whose role is investment. */
  invested_assets: number;
  /** Null while the contribution rate or pay is unknown. */
  workplace: WorkplaceSaving | null;
  limits: ContributionLimits;
}

/** A user-authored savings target — the only content here nothing re-derives. */
export interface Goal {
  id: string;
  name: string;
  target_amount: number;
  target_date?: string;
  linked_account_id?: string;
  current_amount: number;
  priority: number;
}

/**
 * One grant of employer equity. The next vest date and the ESPP purchase date
 * are what put dated actions on the plan, ahead of the phase order.
 */
export interface EquityGrant {
  id: string;
  kind: "rsu" | "espp" | "iso" | "nso";
  label?: string;
  grant_date?: string;
  total_shares?: number;
  shares_vested?: number;
  next_vest_date?: string;
  vest_frequency?: "monthly" | "quarterly" | "semiannual" | "annual" | "cliff";
  /** Fraction of the grant per tranche, e.g. 0.0625. */
  vest_share?: number;
  espp_discount_pct?: number;
  espp_has_lookback?: boolean;
  espp_contribution_pct?: number;
  espp_plan_max_pct?: number;
  espp_offering_start?: string;
  espp_purchase_date?: string;
  has_10b5_1?: boolean;
  blackout_policy?: "none" | "quarterly" | "event_based" | "unknown";
  /** Overrides the statutory supplemental rate; absent means the default. */
  supplemental_withholding_pct?: number;
}

/** An account's tax treatment and monthly contribution. */
export interface RetirementAccountTerms {
  account_id: string;
  kind: "401k" | "roth_401k" | "ira" | "roth_ira" | "hsa" | "taxable";
  monthly_contribution: number;
}

export interface DerivedValue {
  field_key: string;
  amount?: number;
  number?: number;
  text?: string;
  /** Why the figure is what it is, so the screen can show its working. */
  basis: string;
  already_answered: boolean;
}

export interface DerivedProfile {
  values: DerivedValue[];
  breakdown: Array<{ category_id: string | null; name: string; total_spent: number }>;
  months_of_data: number;
  bucket_coverage: number;
  /** Too little bucketed spending for the derived figures to be asserted. */
  low_confidence: boolean;
  /** No complete months to average at all — a different ask from low coverage. */
  no_spending_data: boolean;
  unanswered: string[];
  stale: string[];
}

// ---------------------------------------------------------------- queries

const PROFILE_KEY = ["wealth", "profile"] as const;
const QUESTS_KEY = ["wealth", "quests"] as const;
const DERIVED_KEY = ["wealth", "derived"] as const;
const FIELDS_KEY = ["wealth", "fields"] as const;
const GOALS_KEY = ["wealth", "goals"] as const;
const RETIREMENT_ACCOUNTS_KEY = ["wealth", "retirement-accounts"] as const;
const GRANTS_KEY = ["wealth", "equity-grants"] as const;

export function useWealthProfile() {
  return useQuery<WealthProfile>({
    queryKey: PROFILE_KEY,
    queryFn: () => apiFetch<WealthProfile>("/wealth/profile", {}, getStoredToken()),
  });
}

export function useDerivedProfile() {
  return useQuery<DerivedProfile>({
    queryKey: DERIVED_KEY,
    queryFn: () => apiFetch<DerivedProfile>("/wealth/profile/derived", {}, getStoredToken()),
  });
}

/** How a profile question is asked. Money is cents; percent is a fraction. */
export type FieldKind = "money" | "percent" | "integer" | "boolean" | "date" | "text" | "choice";

/** One profile question, as the server specifies it. */
export interface FieldInfo {
  key: string;
  label: string;
  kind: FieldKind;
  choices?: Array<{ value: string; label: string }>;
  /** An override of a derived figure rather than a question. */
  correction?: boolean;
}

export interface FieldRegistry {
  fields: FieldInfo[];
  /** Labels for every field and pseudo-field, keyed by key. */
  labels: Record<string, string>;
}

/**
 * Every profile question with its label, kind and choices.
 *
 * Served rather than duplicated here: the unlock prompt beside a blocked
 * action and the sentence the engine writes into its own detail text have to
 * name the same thing, and a question's input has to match what the server
 * stores. Inferring a type from a key's name got that wrong.
 *
 * `labels` covers pseudo-keys too — `transaction_history`, `paystub_ytd` —
 * which name data the app lacks rather than a question for the user. They are
 * absent from `fields`, which is how the two are told apart.
 */
export function useFieldRegistry() {
  return useQuery<FieldRegistry>({
    queryKey: FIELDS_KEY,
    queryFn: () => apiFetch<FieldRegistry>("/wealth/fields", {}, getStoredToken()),
    staleTime: Infinity, // the registry only changes when the app is redeployed
  });
}

export function useQuests() {
  return useQuery<{ quests: Quest[]; phases: Phase[]; summary: PlanSummary }>({
    queryKey: QUESTS_KEY,
    queryFn: () =>
      apiFetch<{ quests: Quest[]; phases: Phase[]; summary: PlanSummary }>(
        "/wealth/quests",
        {},
        getStoredToken(),
      ),
  });
}

export function useGoals() {
  return useQuery<Goal[]>({
    queryKey: GOALS_KEY,
    queryFn: async () => {
      const res = await apiFetch<{ goals: Goal[] | null }>("/wealth/goals", {}, getStoredToken());
      return res.goals ?? [];
    },
  });
}

export function useEquityGrants() {
  return useQuery<EquityGrant[]>({
    queryKey: GRANTS_KEY,
    queryFn: async () => {
      const res = await apiFetch<{ grants: EquityGrant[] | null }>(
        "/wealth/equity-grants",
        {},
        getStoredToken(),
      );
      return res.grants ?? [];
    },
  });
}

export function useRetirementAccounts() {
  return useQuery<RetirementAccountTerms[]>({
    queryKey: RETIREMENT_ACCOUNTS_KEY,
    queryFn: async () => {
      const res = await apiFetch<{ terms: RetirementAccountTerms[] | null }>(
        "/wealth/retirement-accounts",
        {},
        getStoredToken(),
      );
      return res.terms ?? [];
    },
  });
}

// ---------------------------------------------------------------- writes

/**
 * Saves a partial profile.
 *
 * `fields` carries only the keys being set: a key present with a null value
 * clears the answer, while an omitted key leaves it untouched. `source` says
 * how the values arrived, and is not bookkeeping -- passing 'confirmed' for a
 * figure the user never saw would let a high-stakes rule fire on an inference.
 */
export function useSaveProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (args: { fields: Record<string, unknown>; source?: FieldSource }) =>
      apiFetch<WealthProfile>(
        "/wealth/profile",
        {
          method: "PUT",
          body: JSON.stringify({ fields: args.fields, source: args.source ?? "entered" }),
        },
        getStoredToken(),
      ),
    onSuccess: () => {
      // The action list is generated from the profile, so it is stale the
      // moment an answer changes.
      queryClient.invalidateQueries({ queryKey: PROFILE_KEY });
      queryClient.invalidateQueries({ queryKey: QUESTS_KEY });
      queryClient.invalidateQueries({ queryKey: DERIVED_KEY });
    },
  });
}

/**
 * Creates, updates or deletes a goal.
 *
 * Goals are the one thing here the app cannot rebuild from transactions, so a
 * failed write loses something real. The list is refetched rather than patched
 * so a partial failure shows as itself rather than as a phantom success.
 */
export function useSaveGoal() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (goal: Partial<Goal> & { name: string; target_amount: number }) =>
      apiFetch<{ id: string }>(
        goal.id ? `/wealth/goals/${goal.id}` : "/wealth/goals",
        { method: goal.id ? "PUT" : "POST", body: JSON.stringify(goal) },
        getStoredToken(),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: GOALS_KEY }),
  });
}

export function useDeleteGoal() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/wealth/goals/${id}`, { method: "DELETE" }, getStoredToken()),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: GOALS_KEY }),
  });
}

/**
 * Creates or updates a grant. The action list is refetched too: a grant is
 * what turns a vest or purchase date into a dated action.
 */
export function useSaveEquityGrant() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (grant: Partial<EquityGrant> & { kind: EquityGrant["kind"] }) =>
      apiFetch<{ id: string }>(
        grant.id ? `/wealth/equity-grants/${grant.id}` : "/wealth/equity-grants",
        { method: grant.id ? "PUT" : "POST", body: JSON.stringify(grant) },
        getStoredToken(),
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: GRANTS_KEY });
      queryClient.invalidateQueries({ queryKey: QUESTS_KEY });
    },
  });
}

export function useDeleteEquityGrant() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/wealth/equity-grants/${id}`, { method: "DELETE" }, getStoredToken()),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: GRANTS_KEY });
      queryClient.invalidateQueries({ queryKey: QUESTS_KEY });
    },
  });
}

export function useSaveRetirementAccount() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (terms: RetirementAccountTerms) =>
      apiFetch(
        `/wealth/retirement-accounts/${terms.account_id}`,
        { method: "PUT", body: JSON.stringify(terms) },
        getStoredToken(),
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: RETIREMENT_ACCOUNTS_KEY });
      queryClient.invalidateQueries({ queryKey: QUESTS_KEY });
    },
  });
}

export function useDeleteRetirementAccount() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountId: string) =>
      apiFetch(`/wealth/retirement-accounts/${accountId}`, { method: "DELETE" }, getStoredToken()),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: RETIREMENT_ACCOUNTS_KEY });
      queryClient.invalidateQueries({ queryKey: QUESTS_KEY });
    },
  });
}

export function useSetQuestStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    // An action's id is its catalog key, which carries a colon for per-account
    // and per-grant instances.
    mutationFn: (args: { questId: string; status: QuestStatus; note?: string }) =>
      apiFetch<{ status: string }>(
        `/wealth/quests/${encodeURIComponent(args.questId)}`,
        { method: "PUT", body: JSON.stringify({ status: args.status, note: args.note ?? "" }) },
        getStoredToken(),
      ),
    onSuccess: () => {
      // Completing one action can unlock a whole phase, so the list is refetched
      // rather than patched in place.
      queryClient.invalidateQueries({ queryKey: QUESTS_KEY });
    },
  });
}

// ---------------------------------------------------------------- helpers

/** Whole dollars. Cents are noise next to a plan measured in thousands. */
export function usd(cents: number | undefined | null): string {
  if (cents === undefined || cents === null) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(Math.round(cents / 100));
}

export function pct(fraction: number | undefined | null, digits = 0): string {
  if (fraction === undefined || fraction === null) return "—";
  return `${(fraction * 100).toFixed(digits)}%`;
}

export function formatDate(iso: string | undefined): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

/** Days until a date; negative once it has passed. */
export function daysUntil(iso: string | undefined): number | null {
  if (!iso) return null;
  const diff = new Date(iso).getTime() - Date.now();
  return Math.ceil(diff / 86_400_000);
}

/**
 * Groups actions by phase and separates out the dated ones.
 *
 * Dated actions are surfaced together and ahead of everything else because
 * they are the ones with a cost for waiting: payroll closes on 31 December,
 * and a vest lands when the grant says so regardless of which phase the user
 * has reached.
 */
export function useGroupedQuests(quests: Quest[] | undefined, phases: Phase[] | undefined) {
  return useMemo(() => {
    const all = quests ?? [];
    const live = all.filter((q) => q.status !== "not_applicable");

    const dated = live
      .filter((q) => q.due_date && q.status !== "complete" && q.status !== "skipped")
      .sort((a, b) => (a.due_date ?? "").localeCompare(b.due_date ?? ""));

    const byPhase = (phases ?? []).map((phase) => {
      const items = live.filter((q) => q.phase === phase.number);
      const actionable = items.filter((q) => q.status === "available" || q.status === "blocked");
      // Progress is counted in milestones, which are what actually finish the
      // phase, and only the ones with something to do.
      const milestones = phase.milestones.filter((m) => m.applies);
      return {
        phase,
        items,
        milestones,
        actionable: actionable.length,
        done: milestones.filter((m) => m.done).length,
        locked: !phase.unlocked,
      };
    });

    // The current phase is the first open one with a milestone still unmet,
    // falling back to the first with anything to do.
    const current =
      byPhase.find((p) => p.phase.number > 0 && !p.locked && p.phase.milestones.some((m) => !m.done))
        ?.phase.number ??
      byPhase.find((p) => p.phase.number > 0 && p.actionable > 0)?.phase.number ??
      0;

    return { dated, byPhase, current };
  }, [quests, phases]);
}

/** Whether the user has answered enough for the engine to say anything useful. */
export function useIsConfigured(profile: WealthProfile | undefined): boolean {
  return useCallback(() => {
    const answered = profile?.fields ?? {};
    return Object.keys(answered).length > 0;
  }, [profile])();
}
