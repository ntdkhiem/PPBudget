"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Check } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toInputValue } from "./form-bits";
import { useSaveProfile, type FieldInfo, type FieldKind, type WealthProfile } from "./wealth-data";

/**
 * The questions one blocked action is waiting on, and nothing else.
 *
 * Reached from an action card's "answer 2 questions to unlock". Landing that
 * promise on a page of forty fields would make it a lie, so this renders
 * exactly the keys the action named and returns the user to where they were.
 *
 * How each question is asked -- its input, its choices -- comes from the
 * server's field registry. The page used to infer a type from the shape of a
 * key and keep its own copies of the choices, and a mis-typed field silently
 * writes the wrong units, which is worse than not asking at all.
 */

const YES_NO = [
  { value: "yes", label: "Yes" },
  { value: "no", label: "No" },
];

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
    href: "/wealth/profile#equity",
    cta: "Edit your grants",
  },
  account_roles: {
    text: "This needs a role for each account: checking, savings, investment, card or loan.",
    href: "/accounts",
    cta: "Go to accounts",
  },
};

/**
 * "your gross pay" -> "Your gross pay". The registry's labels are written to
 * run on inside a sentence; CSS capitalize made every word a capital and
 * "401(k)" into "401(K)".
 */
function sentence(label: string): string {
  return label.charAt(0).toUpperCase() + label.slice(1);
}

/** The draft value for a field, in the units the server stores. */
function fromInputValue(kind: FieldKind, raw: string): unknown {
  switch (kind) {
    case "boolean":
      return raw === "yes";
    case "date":
      return `${raw}T00:00:00Z`;
    case "percent": {
      const n = Number.parseFloat(raw);
      return Number.isFinite(n) ? n / 100 : undefined;
    }
    case "integer": {
      const n = Number.parseInt(raw, 10);
      return Number.isFinite(n) ? n : undefined;
    }
    case "money": {
      const n = Number.parseFloat(raw.replace(/[^0-9.\-]/g, ""));
      return Number.isFinite(n) ? Math.round(n * 100) : undefined;
    }
    default:
      return raw;
  }
}

function OptionButtons({
  options,
  value,
  onChange,
}: {
  options: Array<{ value: string; label: string }>;
  value: string | undefined;
  onChange: (v: string) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          onClick={() => onChange(opt.value)}
          aria-pressed={value === opt.value}
          className={cn(
            "rounded-xl border px-4 py-2 text-sm font-medium transition-colors",
            value === opt.value
              ? "border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-500/15 dark:text-indigo-300"
              : "border-slate-200 text-slate-600 hover:border-slate-300 dark:border-slate-700 dark:text-slate-300",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

export function FocusedQuestions({
  fieldKeys,
  labels,
  fields,
  profile,
  backTo = "/wealth",
  onDone,
}: {
  fieldKeys: string[];
  labels: Record<string, string>;
  /** The server's field registry: how each question is asked. */
  fields: FieldInfo[];
  /** Current answers, so editing starts from what is already there. */
  profile?: WealthProfile;
  /** Where the back link goes — wherever the user actually came from. */
  backTo?: string;
  onDone: () => void;
}) {
  const save = useSaveProfile();
  const specs = useMemo(() => new Map(fields.map((f) => [f.key, f])), [fields]);
  const kindOf = (key: string): FieldKind => specs.get(key)?.kind ?? "text";

  // What each box starts from, kept so submit can tell an edit from a box left
  // alone. Needs the registry at mount; the profile page waits for it.
  const [initial] = useState<Record<string, string>>(() => {
    const seed: Record<string, string> = {};
    if (!profile) return seed;
    for (const k of fieldKeys) {
      const v = (profile as unknown as Record<string, unknown>)[k];
      const spec = fields.find((f) => f.key === k);
      if (spec && v !== undefined && v !== null) seed[k] = toInputValue(spec.kind, v);
    }
    return seed;
  });
  const [draft, setDraft] = useState(initial);

  const answerable = fieldKeys.filter((k) => specs.has(k));
  const elsewhere = fieldKeys.filter((k) => !specs.has(k));

  // Every requested field already has a value, so this is a correction rather
  // than a first pass. Worth saying, because the two read differently.
  const editing =
    answerable.length > 0 &&
    answerable.every((k) => (profile?.fields ?? {})[k] !== undefined);

  const set = (key: string, value: string) => setDraft((d) => ({ ...d, [key]: value }));

  const submit = () => {
    const out: Record<string, unknown> = {};
    const answeredAlready = profile?.fields ?? {};
    for (const key of answerable) {
      const raw = draft[key];
      // A box left as it was is not an answer. Sending it again would
      // relabel a figure we worked out as one the user typed, and a box that
      // failed to fill in would read as emptied -- and erase the answer.
      if (raw === undefined || raw === initial[key]) continue;
      if (raw === "") {
        // Emptying a field that had a value is a request to REMOVE the answer,
        // and the API treats an explicit null as exactly that. Skipping it
        // would make an answer impossible to take back once given.
        if (answeredAlready[key] !== undefined) out[key] = null;
        continue;
      }
      const value = fromInputValue(kindOf(key), raw);
      if (value !== undefined) out[key] = value;
    }
    if (Object.keys(out).length === 0) {
      onDone();
      return;
    }
    save.mutate({ fields: out, source: "entered" }, { onSuccess: onDone });
  };

  return (
    <div className="mx-auto max-w-2xl">
      <Link
        href={backTo}
        className="mb-6 inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        {backTo === "/wealth/profile" ? "Back to your profile" : "Back to your plan"}
      </Link>

      <h2 className="text-2xl font-bold font-heading tracking-tight text-slate-900 dark:text-white">
        {editing
          ? answerable.length === 1
            ? sentence(labels[answerable[0]] ?? "Change this answer")
            : "Change these answers"
          : answerable.length === 1
            ? "One question"
            : `${answerable.length} questions`}
      </h2>
      <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
        {editing
          ? "Your current answer is filled in. Change it and save, or leave it as it is."
          : "Answering these unlocks the action you came from."}
      </p>

      <div className="mt-8 space-y-6">
        {answerable.map((key) => {
          const spec = specs.get(key)!;
          const label = labels[key] ?? spec.label;

          return (
            <div key={key} className="space-y-2">
              <Label>{sentence(label)}</Label>

              {spec.kind === "choice" && (
                <OptionButtons
                  options={spec.choices ?? []}
                  value={draft[key]}
                  onChange={(v) => set(key, v)}
                />
              )}

              {spec.kind === "boolean" && (
                <OptionButtons options={YES_NO} value={draft[key]} onChange={(v) => set(key, v)} />
              )}

              {spec.kind === "date" && (
                <Input
                  type="date"
                  value={draft[key] ?? ""}
                  onChange={(e) => set(key, e.target.value)}
                />
              )}

              {(spec.kind === "money" || spec.kind === "percent" || spec.kind === "integer") && (
                <div className="flex items-center gap-2">
                  {spec.kind === "money" && (
                    <span className="text-sm text-slate-500 dark:text-slate-400">$</span>
                  )}
                  <Input
                    inputMode="decimal"
                    className="w-40"
                    value={draft[key] ?? ""}
                    onChange={(e) => set(key, e.target.value)}
                  />
                  {spec.kind === "percent" && (
                    <span className="text-sm text-slate-500 dark:text-slate-400">%</span>
                  )}
                </div>
              )}

              {spec.kind === "text" && (
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
          {save.isPending ? "Saving…" : "Save and go back"}
        </Button>
        <Button variant="ghost" onClick={onDone} disabled={save.isPending}>
          Cancel
        </Button>
      </div>
    </div>
  );
}
