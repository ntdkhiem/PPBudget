"use client";

import { useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Choice, Field } from "./form-bits";
import {
  PAY_FREQUENCIES,
  PAYSTUB_FIELDS,
  paystubDraft,
  paystubFrom,
  todayInput,
  usePaystub,
  useSavePaystub,
} from "./inputs-data";
import { usd } from "./wealth-data";

/** A paystub's calendar day, read in UTC as it is stored; local time shifts it a day west. */
const dayOf = (iso: string) =>
  new Date(iso).toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });

/**
 * The latest paystub, and the place to enter one.
 *
 * Its year-to-date figures turn several actions from estimates into exact
 * amounts -- how much 401(k) room is left, how much vest withholding is
 * already covered. It could only be entered by re-running the whole setup,
 * which is a lot of ceremony for one document.
 */
export function PaystubCard() {
  const { data: stored, isLoading } = usePaystub();
  const save = useSavePaystub();
  const [draft, setDraft] = useState<Record<string, string> | null>(null);

  // Blocked actions link here as /wealth/profile#paystub, but the page renders
  // after its data loads, so the browser's own jump to the anchor has already
  // missed. Scroll once the section exists.
  useEffect(() => {
    if (!isLoading && window.location.hash === "#paystub") {
      document.getElementById("paystub")?.scrollIntoView({ block: "start" });
    }
  }, [isLoading]);

  const set = (k: string) => (v: string) => setDraft((d) => ({ ...d, [k]: v }));
  const open = () => {
    save.reset();
    // The stored figures to correct, dated as they were; changing the date
    // records a newer reading beside it rather than overwriting this one.
    setDraft({ as_of_date: todayInput(), ...paystubDraft(stored) });
  };
  const submit = () => {
    if (!draft) return;
    save.mutate(paystubFrom(draft, stored, draft.as_of_date || todayInput()), {
      onSuccess: () => setDraft(null),
    });
  };

  return (
    <section id="paystub" className="scroll-mt-24">
      <h2 className="mb-1 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        Your latest paystub
      </h2>
      <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
        The year-to-date column turns several actions into exact amounts, like how much 401(k)
        room is left this year.
      </p>

      {draft ? (
        <div className="space-y-5 rounded-2xl border border-slate-200 p-5 dark:border-slate-800">
          <Field
            label="Paystub date"
            hint="Keep the date to correct this reading. A newer date is saved alongside it, so a mid-year change stays visible."
          >
            <Input
              type="date"
              className="max-w-xs"
              value={draft.as_of_date ?? ""}
              onChange={(e) => set("as_of_date")(e.target.value)}
            />
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            {PAYSTUB_FIELDS.map((f) => (
              <Field key={f.key} label={f.label}>
                <div className="flex items-center gap-2">
                  <span className="text-sm text-slate-500 dark:text-slate-400">$</span>
                  <Input
                    inputMode="decimal"
                    value={draft[f.key] ?? ""}
                    onChange={(e) => set(f.key)(e.target.value)}
                  />
                </div>
              </Field>
            ))}
            <Field label="Paychecks so far this year">
              <Input
                inputMode="numeric"
                className="w-32"
                value={draft.paychecks_ytd ?? ""}
                onChange={(e) => set("paychecks_ytd")(e.target.value)}
              />
            </Field>
          </div>
          <Field label="How often are you paid?">
            <Choice
              value={draft.paychecks_per_year}
              options={PAY_FREQUENCIES}
              onChange={set("paychecks_per_year")}
            />
          </Field>

          {save.error && (
            <p className="text-sm text-rose-600 dark:text-rose-400">{save.error.message}</p>
          )}

          <div className="flex gap-2">
            <Button onClick={submit} disabled={save.isPending || !draft.gross}>
              {save.isPending ? "Saving…" : "Save"}
            </Button>
            <Button variant="ghost" onClick={() => setDraft(null)} disabled={save.isPending}>
              Cancel
            </Button>
          </div>
        </div>
      ) : stored ? (
        <div className="flex items-start gap-3 rounded-2xl border border-slate-200 p-4 dark:border-slate-800">
          <div className="min-w-0 flex-1">
            <div className="text-sm font-semibold text-slate-900 dark:text-white">
              As of {dayOf(stored.as_of_date)}
              {stored.paychecks_ytd != null && (
                <span className="ml-2 font-normal text-slate-500 dark:text-slate-400">
                  {stored.paychecks_ytd}
                  {stored.paychecks_per_year != null && ` of ${stored.paychecks_per_year}`} paychecks
                </span>
              )}
            </div>
            <dl className="mt-2 grid grid-cols-2 gap-x-6 gap-y-1 text-sm sm:grid-cols-3">
              {PAYSTUB_FIELDS.map((f) => (
                <div key={f.key} className="flex justify-between gap-2">
                  <dt className="text-slate-500 dark:text-slate-400">{f.label.replace(/, (year to date|YTD)$/, "")}</dt>
                  <dd className="font-medium text-slate-900 dark:text-white">{usd(stored[f.key])}</dd>
                </div>
              ))}
            </dl>
          </div>
          <Button variant="ghost" size="icon" aria-label="Update your paystub" onClick={open}>
            <Pencil className="h-4 w-4 text-slate-400" />
          </Button>
        </div>
      ) : (
        !isLoading && (
          <Button variant="outline" onClick={open}>
            <Plus className="h-4 w-4" />
            Enter a paystub
          </Button>
        )
      )}
    </section>
  );
}
