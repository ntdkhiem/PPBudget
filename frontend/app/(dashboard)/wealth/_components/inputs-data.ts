"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch, getStoredToken } from "@/lib/api";
import { toCents, toFraction, toInputValue } from "./form-bits";

/**
 * The inputs the plan reads that are not profile answers: each account's
 * rates, and the latest paystub.
 *
 * Both could only be entered by re-running the whole setup, and a savings
 * account's rate not at all, so the actions built on them -- moving idle cash,
 * clearing a promotional balance before it expires -- could not appear for
 * anyone. These back the Accounts page, the paystub card and setup alike.
 */

// ---------------------------------------------------------------- rates

/** Per-account terms. Rates are fractions; money is cents. */
export interface AccountTerms {
  account_id: string;
  apy?: number;
  apr?: number;
  promo_apr?: number;
  /** A timestamp; the day is what counts. */
  promo_expires_on?: string;
  statement_balance?: number;
  min_payment?: number;
  due_day?: number;
}

/**
 * A partial save. Only the terms named change, and null clears one -- a save
 * that names the APR leaves a promotional rate recorded elsewhere alone.
 */
export type AccountTermsPatch = Partial<{
  [K in Exclude<keyof AccountTerms, "account_id">]: AccountTerms[K] | null;
}>;

const TERMS_KEY = ["wealth", "account-terms"] as const;

export function useAccountTerms() {
  return useQuery({
    queryKey: TERMS_KEY,
    queryFn: async () => {
      const res = await apiFetch<{ terms: Record<string, AccountTerms> | null }>(
        "/wealth/account-terms",
        {},
        getStoredToken(),
      );
      return res.terms ?? {};
    },
  });
}

export function useSaveAccountTerms() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ accountId, patch }: { accountId: string; patch: AccountTermsPatch }) =>
      apiFetch(
        `/wealth/account-terms/${accountId}`,
        { method: "PUT", body: JSON.stringify(patch) },
        getStoredToken(),
      ),
    // Rates reorder the plan: which balance is paid first, whether cash is idle.
    onSuccess: () => qc.invalidateQueries({ queryKey: ["wealth"] }),
  });
}

/** A rate for a percent input: 0.2249 -> "22.49". */
export const pctIn = (f?: number | null) => (f == null ? "" : toInputValue("percent", f));

/**
 * One rate box's change, if it has one: undefined when it is as stored, null
 * when it was emptied, else the fraction. Unparseable input counts as no
 * change rather than as a clear.
 */
export function rateChange(raw: string, stored?: number | null): number | null | undefined {
  if (raw === pctIn(stored)) return undefined;
  if (raw.trim() === "") return stored == null ? undefined : null;
  const f = toFraction(raw);
  return f === null ? undefined : f;
}

// -------------------------------------------------------------- paystub

/** One dated year-to-date reading. Money is cents. */
export interface Paystub {
  id?: string;
  as_of_date: string;
  gross: number;
  federal_withheld: number;
  state_withheld: number;
  pretax_401k: number;
  roth_401k: number;
  hsa_contribution: number;
  espp_contribution: number;
  pretax_benefits: number;
  supplemental_wages: number;
  paychecks_ytd?: number;
  paychecks_per_year?: number;
}

type PaystubMoneyKey =
  | "gross"
  | "federal_withheld"
  | "state_withheld"
  | "pretax_401k"
  | "roth_401k"
  | "hsa_contribution";

/** The figures asked for, in paystub order, shared by setup and the profile card. */
export const PAYSTUB_FIELDS: Array<{ key: PaystubMoneyKey; label: string }> = [
  { key: "gross", label: "Gross pay, year to date" },
  { key: "federal_withheld", label: "Federal tax withheld, YTD" },
  { key: "state_withheld", label: "State tax withheld, YTD" },
  { key: "pretax_401k", label: "401(k) pre-tax, YTD" },
  { key: "roth_401k", label: "401(k) Roth, YTD" },
  { key: "hsa_contribution", label: "HSA, YTD" },
];

export const PAY_FREQUENCIES = [
  { value: "52", label: "Weekly" },
  { value: "26", label: "Every two weeks" },
  { value: "24", label: "Twice a month" },
  { value: "12", label: "Monthly" },
];

const PAYSTUB_KEY = ["wealth", "paystub"] as const;

export function usePaystub() {
  return useQuery({
    queryKey: PAYSTUB_KEY,
    queryFn: async () => {
      const res = await apiFetch<{ paystub: Paystub | null }>("/wealth/paystub", {}, getStoredToken());
      return res.paystub;
    },
  });
}

export function useSavePaystub() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: Paystub) =>
      apiFetch("/wealth/paystub", { method: "PUT", body: JSON.stringify(p) }, getStoredToken()),
    // Several actions turn from estimates into exact amounts with a paystub.
    onSuccess: () => qc.invalidateQueries({ queryKey: ["wealth"] }),
  });
}

/**
 * A stored reading in its inputs' units: dollars, a yyyy-mm-dd date, counts
 * as text. Keyed by the reading's own field names.
 */
export function paystubDraft(p: Paystub | null | undefined): Record<string, string> {
  if (!p) return {};
  const out: Record<string, string> = { as_of_date: p.as_of_date.slice(0, 10) };
  for (const { key } of PAYSTUB_FIELDS) out[key] = toInputValue("money", p[key]);
  if (p.paychecks_ytd != null) out.paychecks_ytd = String(p.paychecks_ytd);
  if (p.paychecks_per_year != null) out.paychecks_per_year = String(p.paychecks_per_year);
  return out;
}

/**
 * A reading from its inputs. A paystub is saved whole -- it is one dated
 * snapshot -- so the figures not asked for here (ESPP, pre-tax benefits,
 * supplemental wages) carry over from the stored reading instead of being
 * saved as zero, which is what setup used to do.
 */
export function paystubFrom(d: Record<string, string>, stored: Paystub | null | undefined, asOf: string): Paystub {
  const money = (k: PaystubMoneyKey) => toCents(d[k] ?? "") ?? 0;
  const int = (k: string) => {
    const n = Number.parseInt(d[k] ?? "", 10);
    return Number.isFinite(n) ? n : undefined;
  };
  return {
    as_of_date: `${asOf}T00:00:00Z`,
    gross: money("gross"),
    federal_withheld: money("federal_withheld"),
    state_withheld: money("state_withheld"),
    pretax_401k: money("pretax_401k"),
    roth_401k: money("roth_401k"),
    hsa_contribution: money("hsa_contribution"),
    espp_contribution: stored?.espp_contribution ?? 0,
    pretax_benefits: stored?.pretax_benefits ?? 0,
    supplemental_wages: stored?.supplemental_wages ?? 0,
    paychecks_ytd: int("paychecks_ytd"),
    paychecks_per_year: int("paychecks_per_year"),
  };
}

/** Today as a date input's yyyy-mm-dd, in the user's own calendar. */
export function todayInput(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
