"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ArrowRight, Check } from "lucide-react";
import { apiFetch, getStoredToken, type Account } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Slider } from "@/components/ui/slider";
import { Choice, Field, toCents, toFraction, toInputValue } from "./form-bits";
import { useFieldRegistry, useSaveProfile, usd, type WealthProfile } from "./wealth-data";

/**
 * The eight-screen core intake.
 *
 * This is the whole of what a user has to answer before the engine can produce
 * a real action list. Everything else is asked later, attached to the specific
 * action it unlocks, because a question is far easier to answer when its price
 * is visible than when it is the nineteenth field on a form.
 *
 * Nothing here asks for spending. Housing, utilities, groceries, transport and
 * subscriptions are all in the transaction data already, and recalled figures
 * run systematically low -- a user who guesses $400 for transport when the real
 * number is $680 corrupts the emergency fund target, the independence number
 * and the surplus at once, silently. Those are derived and confirmed on a
 * separate screen instead.
 *
 * Each step SAVES AS IT GOES rather than batching at the end. Someone who stops
 * after the fifth screen should keep what they answered and get the part of the
 * plan it supports, not lose the lot.
 */

type StepId =
  | "birth"
  | "filing"
  | "household"
  | "paystub"
  | "retirement"
  | "debts"
  | "equity"
  | "cushion";

const STEPS: Array<{ id: StepId; title: string; blurb: string }> = [
  {
    id: "birth",
    title: "When were you born?",
    blurb:
      "This decides which contribution catch-ups you qualify for and when your retirement accounts open without penalty. One answer, six rules.",
  },
  {
    id: "filing",
    title: "How do you file?",
    blurb: "Filing status and state set the tax brackets everything else is measured against.",
  },
  {
    id: "household",
    title: "Who is in your household?",
    blurb:
      "If you file jointly, your spouse's income counts toward the limits that decide how you can fund a Roth IRA.",
  },
  {
    id: "paystub",
    title: "Open a recent paystub",
    blurb:
      "The year-to-date column. This is the one screen worth fetching a document for: it replaces three questions people cannot reliably recall, and it is what turns advice into exact amounts.",
  },
  {
    id: "retirement",
    title: "Your pay and 401(k)",
    blurb:
      "What you earn, your current contribution rate and what your employer matches. This is where the single highest-return action in the plan comes from.",
  },
  {
    id: "debts",
    title: "What do your balances cost?",
    blurb:
      "Interest rates are the one thing your bank connection does not report, and without them balances cannot be ordered against each other or against investing.",
  },
  {
    id: "equity",
    title: "Do you get company stock?",
    blurb: "Equity compensation changes a large part of the plan, so this routes the rest of it.",
  },
  {
    id: "cushion",
    title: "How much cushion do you want?",
    blurb:
      "In months, not dollars. The dollar figure comes from what your own essentials actually cost.",
  },
];

export function IntakeCore({
  profile,
  onDone,
}: {
  profile: WealthProfile | undefined;
  onDone: () => void;
}) {
  const save = useSaveProfile();
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [aprs, setAprs] = useState<Record<string, string>>({});

  // Liability accounts, so the rate screen lists real cards rather than asking
  // the user to describe them.
  const { data: accounts } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, getStoredToken()),
  });
  const liabilities = useMemo(
    () => (accounts ?? []).filter((a) => a.type === "liability"),
    [accounts],
  );

  // What is already on file, in the inputs' own units, so running setup again
  // starts from the current answers instead of a row of empty boxes that read
  // as lost. Shown, never saved as is: only what is changed here is written
  // back, so clicking through does not relabel a figure we worked out as one
  // the user told us.
  const { data: registry } = useFieldRegistry();
  const onFile = useMemo(() => {
    const values = profile as unknown as Record<string, unknown> | undefined;
    const out: Record<string, string> = {};
    for (const f of registry?.fields ?? []) {
      const v = values?.[f.key];
      if (v !== undefined && v !== null) out[f.key] = toInputValue(f.kind, v);
    }
    return out;
  }, [profile, registry]);

  const set = (key: string, value: string) => setDraft((d) => ({ ...d, [key]: value }));
  const val = (key: string) => draft[key] ?? onFile[key] ?? "";

  const current = STEPS[step];
  const isLast = step === STEPS.length - 1;

  /** Collects the fields for the current step, skipping anything left blank. */
  const fieldsForStep = (id: StepId): Record<string, unknown> => {
    const out: Record<string, unknown> = {};
    const num = (k: string, f: (v: string) => number | null) => {
      const raw = draft[k];
      if (raw !== undefined && raw !== "") {
        const parsed = f(raw);
        if (parsed !== null) out[k] = parsed;
      }
    };
    const str = (k: string) => {
      if (draft[k]) out[k] = draft[k];
    };
    const bool = (k: string) => {
      if (draft[k] === "yes") out[k] = true;
      if (draft[k] === "no") out[k] = false;
    };

    switch (id) {
      case "birth":
        if (draft.date_of_birth) out.date_of_birth = `${draft.date_of_birth}T00:00:00Z`;
        break;
      case "filing":
        str("filing_status");
        str("resident_state");
        break;
      case "household":
        str("marital_status");
        num("spouse_gross_annual", toCents);
        bool("spouse_has_workplace_plan");
        break;
      case "retirement":
        num("gross_annual_income", toCents);
        num("deferral_pct", toFraction);
        num("match_pct", toFraction);
        num("match_limit_pct", toFraction);
        bool("match_per_paycheck");
        bool("match_has_true_up");
        break;
      case "equity":
        bool("employer_is_public");
        str("employer_ticker");
        bool("has_stock_options");
        break;
      case "cushion":
        num("emergency_fund_target_months", (v) => Number.parseInt(v, 10) || null);
        break;
      default:
        break;
    }
    return out;
  };

  const saveStep = async () => {
    // The paystub and the card rates are separate resources, not profile
    // fields, so they post to their own endpoints.
    if (current.id === "paystub") {
      const money = (k: string) => toCents(draft[k] ?? "") ?? 0;
      const int = (k: string) => {
        const n = Number.parseInt(draft[k] ?? "", 10);
        return Number.isFinite(n) ? n : undefined;
      };
      if (draft.ps_gross) {
        await apiFetch(
          "/wealth/paystub",
          {
            method: "PUT",
            body: JSON.stringify({
              as_of_date: new Date().toISOString(),
              gross: money("ps_gross"),
              federal_withheld: money("ps_federal"),
              state_withheld: money("ps_state"),
              pretax_401k: money("ps_pretax401k"),
              roth_401k: money("ps_roth401k"),
              hsa_contribution: money("ps_hsa"),
              espp_contribution: money("ps_espp"),
              pretax_benefits: money("ps_benefits"),
              supplemental_wages: money("ps_supplemental"),
              paychecks_ytd: int("ps_ytd_count"),
              paychecks_per_year: int("ps_per_year"),
            }),
          },
          getStoredToken(),
        );
      }
    } else if (current.id === "debts") {
      for (const [accountId, raw] of Object.entries(aprs)) {
        const apr = toFraction(raw);
        if (apr === null) continue;
        await apiFetch(
          `/wealth/account-terms/${accountId}`,
          { method: "PUT", body: JSON.stringify({ apr }) },
          getStoredToken(),
        );
      }
    } else {
      const fields = fieldsForStep(current.id);
      if (Object.keys(fields).length > 0) {
        await save.mutateAsync({ fields, source: "entered" });
      }
    }

    if (isLast) onDone();
    else setStep((s) => s + 1);
  };

  return (
    <div className="mx-auto max-w-2xl">
      {/* Progress. Eight is short enough to show honestly. */}
      <div className="mb-8 flex items-center gap-2">
        {STEPS.map((s, i) => (
          <div
            key={s.id}
            className={cn(
              "h-1.5 flex-1 rounded-full transition-colors",
              i < step
                ? "bg-indigo-500"
                : i === step
                  ? "bg-indigo-300 dark:bg-indigo-400/60"
                  : "bg-slate-200 dark:bg-slate-800",
            )}
          />
        ))}
      </div>

      <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-400">
        Step {step + 1} of {STEPS.length}
      </p>
      <h2 className="text-2xl font-bold font-heading tracking-tight text-slate-900 dark:text-white">
        {current.title}
      </h2>
      <p className="mt-2 text-sm leading-relaxed text-slate-500 dark:text-slate-400">
        {current.blurb}
      </p>

      <div className="mt-8 space-y-6">
        {current.id === "birth" && (
          <Field label="Date of birth">
            <Input
              type="date"
              value={val("date_of_birth")}
              onChange={(e) => set("date_of_birth", e.target.value)}
            />
          </Field>
        )}

        {current.id === "filing" && (
          <>
            <Field label="Filing status">
              <Choice
                value={val("filing_status")}
                onChange={(v) => set("filing_status", v)}
                options={[
                  { value: "single", label: "Single" },
                  { value: "mfj", label: "Married, jointly" },
                  { value: "mfs", label: "Married, separately" },
                  { value: "hoh", label: "Head of household" },
                  { value: "qss", label: "Surviving spouse" },
                ]}
              />
            </Field>
            <Field label="State you file in" hint="Two-letter code, e.g. CA.">
              <Input
                maxLength={2}
                placeholder="CA"
                className="w-24 uppercase"
                value={val("resident_state")}
                onChange={(e) => set("resident_state", e.target.value.toUpperCase())}
              />
            </Field>
          </>
        )}

        {current.id === "household" && (
          <>
            <Field label="Marital status">
              <Choice
                value={val("marital_status")}
                onChange={(v) => set("marital_status", v)}
                options={[
                  { value: "single", label: "Single" },
                  { value: "married", label: "Married" },
                  { value: "domestic_partner", label: "Domestic partner" },
                ]}
              />
            </Field>
            {val("marital_status") && val("marital_status") !== "single" && (
              <>
                <Field
                  label="Your spouse's gross annual income"
                  hint="Joint filers share one income limit for Roth contributions, so leaving this out would make the household look eligible when it is not."
                >
                  <Input
                    inputMode="decimal"
                    placeholder="0"
                    value={val("spouse_gross_annual")}
                    onChange={(e) => set("spouse_gross_annual", e.target.value)}
                  />
                </Field>
                <Field label="Do they have a workplace retirement plan?">
                  <Choice
                    value={val("spouse_has_workplace_plan")}
                    onChange={(v) => set("spouse_has_workplace_plan", v)}
                    options={[
                      { value: "yes", label: "Yes" },
                      { value: "no", label: "No" },
                    ]}
                  />
                </Field>
              </>
            )}
          </>
        )}

        {current.id === "paystub" && (
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label="Gross pay, year to date">
              <Input
                inputMode="decimal"
                value={val("ps_gross")}
                onChange={(e) => set("ps_gross", e.target.value)}
              />
            </Field>
            <Field label="Federal tax withheld, YTD">
              <Input
                inputMode="decimal"
                value={val("ps_federal")}
                onChange={(e) => set("ps_federal", e.target.value)}
              />
            </Field>
            <Field label="State tax withheld, YTD">
              <Input
                inputMode="decimal"
                value={val("ps_state")}
                onChange={(e) => set("ps_state", e.target.value)}
              />
            </Field>
            <Field label="401(k) pre-tax, YTD">
              <Input
                inputMode="decimal"
                value={val("ps_pretax401k")}
                onChange={(e) => set("ps_pretax401k", e.target.value)}
              />
            </Field>
            <Field label="401(k) Roth, YTD">
              <Input
                inputMode="decimal"
                value={val("ps_roth401k")}
                onChange={(e) => set("ps_roth401k", e.target.value)}
              />
            </Field>
            <Field label="HSA, YTD">
              <Input
                inputMode="decimal"
                value={val("ps_hsa")}
                onChange={(e) => set("ps_hsa", e.target.value)}
              />
            </Field>
            <Field label="Paychecks so far this year">
              <Input
                inputMode="numeric"
                value={val("ps_ytd_count")}
                onChange={(e) => set("ps_ytd_count", e.target.value)}
              />
            </Field>
            {/* A choice rather than a number box: a "26" placeholder looked
                like an answer and saved nothing. */}
            <div className="sm:col-span-2">
              <Field label="How often are you paid?">
                <Choice
                  value={val("ps_per_year")}
                  onChange={(v) => set("ps_per_year", v)}
                  options={[
                    { value: "52", label: "Weekly" },
                    { value: "26", label: "Every two weeks" },
                    { value: "24", label: "Twice a month" },
                    { value: "12", label: "Monthly" },
                  ]}
                />
              </Field>
            </div>
          </div>
        )}

        {current.id === "retirement" && (
          <>
            {/* Asked directly rather than only annualised from a paystub: the
                paystub screen can be skipped, and without this the match --
                the plan's first action -- stays blocked. */}
            <Field
              label="Gross pay per year"
              hint="Before tax, including bonuses and stock that vests. Your match is worked out from it, and so is how you can fund a Roth IRA, where guessing low is the costly mistake."
            >
              <Input
                inputMode="decimal"
                className="w-40"
                value={val("gross_annual_income")}
                onChange={(e) => set("gross_annual_income", e.target.value)}
              />
            </Field>
            <Field
              label="What percent of pay are you contributing now?"
              hint="The current rate, not the average so far. They differ exactly when it matters."
            >
              <Input
                inputMode="decimal"
                placeholder="6"
                className="w-32"
                value={val("deferral_pct")}
                onChange={(e) => set("deferral_pct", e.target.value)}
              />
            </Field>
            <Field
              label="How much does your employer match?"
              hint="e.g. 50 if they add 50 cents per dollar."
            >
              <Input
                inputMode="decimal"
                placeholder="50"
                className="w-32"
                value={val("match_pct")}
                onChange={(e) => set("match_pct", e.target.value)}
              />
            </Field>
            <Field label="Up to what percent of your salary?" hint="e.g. 6.">
              <Input
                inputMode="decimal"
                placeholder="6"
                className="w-32"
                value={val("match_limit_pct")}
                onChange={(e) => set("match_limit_pct", e.target.value)}
              />
            </Field>
            <Field
              label="Does your plan true up the match at year end?"
              hint="Without a true-up, hitting the annual cap early forfeits the match on every paycheck after it — so this changes the advice, not just the arithmetic."
            >
              <Choice
                value={val("match_has_true_up")}
                onChange={(v) => set("match_has_true_up", v)}
                options={[
                  { value: "yes", label: "Yes" },
                  { value: "no", label: "No" },
                ]}
              />
            </Field>
          </>
        )}

        {current.id === "debts" && (
          <>
            {liabilities.length === 0 ? (
              <p className="rounded-2xl border border-dashed border-slate-200 p-6 text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">
                You have no liability accounts, so there is nothing to rate here.
              </p>
            ) : (
              liabilities.map((acct) => (
                <Field
                  key={acct.id}
                  label={acct.name}
                  hint={`Balance ${usd(Math.abs(acct.current_balance))}. The rate is on your statement.`}
                >
                  <div className="flex items-center gap-2">
                    <Input
                      inputMode="decimal"
                      placeholder="22.49"
                      className="w-32"
                      value={aprs[acct.id] ?? ""}
                      onChange={(e) =>
                        setAprs((a) => ({ ...a, [acct.id]: e.target.value }))
                      }
                    />
                    <span className="text-sm text-slate-500 dark:text-slate-400">% APR</span>
                  </div>
                </Field>
              ))
            )}
          </>
        )}

        {current.id === "equity" && (
          <>
            <Field
              label="Is your employer publicly traded?"
              hint="Private-company equity works differently enough that the plan takes a separate branch."
            >
              <Choice
                value={val("employer_is_public")}
                onChange={(v) => set("employer_is_public", v)}
                options={[
                  { value: "yes", label: "Yes" },
                  { value: "no", label: "No" },
                ]}
              />
            </Field>
            {val("employer_is_public") === "yes" && (
              <>
                <Field label="Ticker" hint="Used to value your vests.">
                  <Input
                    className="w-32 uppercase"
                    value={val("employer_ticker")}
                    onChange={(e) => set("employer_ticker", e.target.value.toUpperCase())}
                  />
                </Field>
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  Once setup is done, add your RSU and ESPP grants under Company stock on the
                  Profile tab. Their vest and purchase dates are what put this part of the plan
                  on the calendar.
                </p>
              </>
            )}
            <Field label="Do you hold stock options?">
              <Choice
                value={val("has_stock_options")}
                onChange={(v) => set("has_stock_options", v)}
                options={[
                  { value: "yes", label: "Yes" },
                  { value: "no", label: "No" },
                ]}
              />
            </Field>
          </>
        )}

        {current.id === "cushion" && (
          <Field
            label={`${val("emergency_fund_target_months") || 6} months of essentials`}
            hint="Six is the usual starting point. A second income argues for less; dependents or variable pay argue for more."
          >
            <Slider
              value={[Number.parseInt(val("emergency_fund_target_months") || "6", 10)]}
              min={1}
              max={12}
              step={1}
              onValueChange={([v]) => set("emergency_fund_target_months", String(v))}
              aria-label="Months of essentials"
            />
          </Field>
        )}
      </div>

      <div className="mt-10 flex items-center justify-between">
        <Button
          variant="ghost"
          onClick={() => setStep((s) => Math.max(0, s - 1))}
          disabled={step === 0 || save.isPending}
        >
          <ArrowLeft className="h-4 w-4" />
          Back
        </Button>

        <div className="flex items-center gap-2">
          {/* Skipping is explicit and harmless: an unanswered question leaves
              one action blocked, not the whole plan broken. */}
          {!isLast && (
            <Button variant="ghost" onClick={() => setStep((s) => s + 1)} disabled={save.isPending}>
              Skip
            </Button>
          )}
          <Button onClick={saveStep} disabled={save.isPending}>
            {isLast ? (
              <>
                <Check className="h-4 w-4" />
                See my plan
              </>
            ) : (
              <>
                Continue
                <ArrowRight className="h-4 w-4" />
              </>
            )}
          </Button>
        </div>
      </div>
    </div>
  );
}
