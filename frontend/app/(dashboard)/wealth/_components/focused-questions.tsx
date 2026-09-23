"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Check } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useSaveProfile } from "./wealth-data";

/**
 * The questions one blocked action is waiting on, and nothing else.
 *
 * Reached from an action card's "answer 2 questions to unlock". Landing that
 * promise on a page of forty fields would make it a lie, so this renders
 * exactly the keys the action named and returns the user to where they were.
 *
 * Field types are inferred from the key rather than declared, because the
 * alternative is a second registry in the browser that has to be kept in step
 * with the Go one -- and a mis-typed field silently writes the wrong units,
 * which is worse than not asking at all. The inference is conservative: an
 * unrecognised key is treated as money, which is the commonest case, and the
 * server's CHECK constraints reject anything genuinely wrong.
 */

type Kind = "money" | "percent" | "integer" | "boolean" | "date" | "text" | "choice";

const CHOICES: Record<string, Array<{ value: string; label: string }>> = {
  filing_status: [
    { value: "single", label: "Single" },
    { value: "mfj", label: "Married, jointly" },
    { value: "mfs", label: "Married, separately" },
    { value: "hoh", label: "Head of household" },
    { value: "qss", label: "Surviving spouse" },
  ],
  marital_status: [
    { value: "single", label: "Single" },
    { value: "married", label: "Married" },
    { value: "domestic_partner", label: "Domestic partner" },
  ],
  hsa_coverage_tier: [
    { value: "self_only", label: "Just me" },
    { value: "family", label: "Family" },
  ],
  housing_tenure: [
    { value: "rent", label: "Rent" },
    { value: "own", label: "Own" },
  ],
  student_loan_kind: [
    { value: "none", label: "None" },
    { value: "federal", label: "Federal" },
    { value: "private", label: "Private" },
    { value: "both", label: "Both" },
  ],
  strategy: [
    { value: "aggressive", label: "Aggressive" },
    { value: "balanced", label: "Balanced" },
    { value: "flexible", label: "Flexible" },
  ],
};

const BOOLEANS = new Set([
  "hdhp_enrolled",
  "has_stock_options",
  "employer_is_public",
  "pays_pmi",
  "student_loan_idr",
  "student_loan_pslf",
  "plan_allows_after_tax",
  "plan_allows_in_service",
  "plan_accepts_rollovers",
  "match_per_paycheck",
  "match_has_true_up",
  "ltd_premium_pretax",
  "spouse_has_workplace_plan",
]);

/**
 * Where a pseudo-key sends the user.
 *
 * These name data the app is missing rather than a question it can put to
 * them, so a text box would be the wrong response entirely.
 */
const ELSEWHERE: Record<string, { text: string; href: string; cta: string }> = {
  transaction_history: {
    text: "This needs some categorised spending to work from.",
    href: "/transactions",
    cta: "Go to transactions",
  },
  paystub_ytd: {
    text: "This needs the year-to-date figures from a recent paystub.",
    href: "/wealth/profile",
    cta: "Run setup again",
  },
  account_apr: {
    text: "This needs the interest rate on each balance.",
    href: "/wealth/profile",
    cta: "Run setup again",
  },
  espp_terms: {
    text: "This needs your ESPP discount, contribution rate and plan maximum.",
    href: "/wealth/profile",
    cta: "Run setup again",
  },
};

function kindOf(key: string): Kind {
  if (CHOICES[key]) return "choice";
  if (BOOLEANS.has(key)) return "boolean";
  if (key === "date_of_birth") return "date";
  if (key === "resident_state" || key === "employer_ticker") return "text";
  if (/_(pct|apr|rate|share)$/.test(key)) return "percent";
  if (/_(months|age|month)$/.test(key)) return "integer";
  return "money";
}

export function FocusedQuestions({
  fieldKeys,
  labels,
  knownFields,
  onDone,
}: {
  fieldKeys: string[];
  labels: Record<string, string>;
  /** The canonical registry, used to tell real fields from pseudo-keys. */
  knownFields: string[];
  onDone: () => void;
}) {
  const save = useSaveProfile();
  const [draft, setDraft] = useState<Record<string, string>>({});

  const known = useMemo(() => new Set(knownFields), [knownFields]);
  const answerable = fieldKeys.filter((k) => known.has(k));
  const elsewhere = fieldKeys.filter((k) => !known.has(k));

  const set = (key: string, value: string) => setDraft((d) => ({ ...d, [key]: value }));

  const submit = () => {
    const fields: Record<string, unknown> = {};
    for (const key of answerable) {
      const raw = draft[key];
      if (raw === undefined || raw === "") continue;

      switch (kindOf(key)) {
        case "boolean":
          fields[key] = raw === "yes";
          break;
        case "date":
          fields[key] = `${raw}T00:00:00Z`;
          break;
        case "percent": {
          const n = Number.parseFloat(raw);
          if (Number.isFinite(n)) fields[key] = n / 100;
          break;
        }
        case "integer": {
          const n = Number.parseInt(raw, 10);
          if (Number.isFinite(n)) fields[key] = n;
          break;
        }
        case "money": {
          const n = Number.parseFloat(raw.replace(/[^0-9.\-]/g, ""));
          if (Number.isFinite(n)) fields[key] = Math.round(n * 100);
          break;
        }
        default:
          fields[key] = raw;
      }
    }
    if (Object.keys(fields).length === 0) {
      onDone();
      return;
    }
    save.mutate({ fields, source: "entered" }, { onSuccess: onDone });
  };

  return (
    <div className="mx-auto max-w-2xl">
      <Link
        href="/wealth"
        className="mb-6 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        Back to your plan
      </Link>

      <h2 className="text-2xl font-bold font-heading tracking-tight text-slate-900 dark:text-white">
        {answerable.length === 1 ? "One question" : `${answerable.length} questions`}
      </h2>
      <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
        Answering these unlocks the action you came from.
      </p>

      <div className="mt-8 space-y-6">
        {answerable.map((key) => {
          const kind = kindOf(key);
          const label = labels[key] ?? key.replace(/_/g, " ");

          return (
            <div key={key} className="space-y-2">
              <Label className="capitalize">{label}</Label>

              {kind === "choice" && (
                <div className="flex flex-wrap gap-2">
                  {CHOICES[key].map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => set(key, opt.value)}
                      className={cn(
                        "rounded-xl border px-4 py-2 text-sm font-medium transition-colors",
                        draft[key] === opt.value
                          ? "border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-500/15 dark:text-indigo-300"
                          : "border-slate-200 text-slate-600 hover:border-slate-300 dark:border-slate-700 dark:text-slate-300",
                      )}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              )}

              {kind === "boolean" && (
                <div className="flex gap-2">
                  {[
                    { value: "yes", label: "Yes" },
                    { value: "no", label: "No" },
                  ].map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => set(key, opt.value)}
                      className={cn(
                        "rounded-xl border px-4 py-2 text-sm font-medium transition-colors",
                        draft[key] === opt.value
                          ? "border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-500/15 dark:text-indigo-300"
                          : "border-slate-200 text-slate-600 hover:border-slate-300 dark:border-slate-700 dark:text-slate-300",
                      )}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              )}

              {kind === "date" && (
                <Input
                  type="date"
                  value={draft[key] ?? ""}
                  onChange={(e) => set(key, e.target.value)}
                />
              )}

              {(kind === "money" || kind === "percent" || kind === "integer") && (
                <div className="flex items-center gap-2">
                  <Input
                    inputMode="decimal"
                    className="w-40"
                    value={draft[key] ?? ""}
                    onChange={(e) => set(key, e.target.value)}
                  />
                  {kind === "percent" && <span className="text-sm text-slate-500 dark:text-slate-400">%</span>}
                  {kind === "integer" && key.endsWith("_months") && (
                    <span className="text-sm text-slate-500 dark:text-slate-400">months</span>
                  )}
                </div>
              )}

              {kind === "text" && (
                <Input
                  className="w-40 uppercase"
                  value={draft[key] ?? ""}
                  onChange={(e) => set(key, e.target.value.toUpperCase())}
                />
              )}
            </div>
          );
        })}

        {elsewhere.map((key) => {
          const target = ELSEWHERE[key];
          return (
            <div
              key={key}
              className="rounded-2xl border border-dashed border-slate-300 p-5 dark:border-slate-700"
            >
              <p className="text-sm text-slate-600 dark:text-slate-300">
                {target?.text ?? labels[key] ?? key.replace(/_/g, " ")}
              </p>
              {target && (
                <Link href={target.href}>
                  <Button size="sm" variant="outline" className="mt-3">
                    {target.cta}
                  </Button>
                </Link>
              )}
            </div>
          );
        })}
      </div>

      <div className="mt-10 flex items-center gap-2">
        <Button onClick={submit} disabled={save.isPending || answerable.length === 0}>
          <Check className="h-4 w-4" />
          Save and go back
        </Button>
        <Button variant="ghost" onClick={onDone} disabled={save.isPending}>
          Cancel
        </Button>
      </div>
    </div>
  );
}
