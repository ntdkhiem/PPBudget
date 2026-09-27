"use client";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { pctIn, rateChange, type AccountTerms, type AccountTermsPatch } from "./inputs-data";

/**
 * An account's rates, on the Accounts page next to what the account is.
 *
 * No bank feed reports a rate. Without a card's APR its payoff cannot be
 * ordered against investing; without a savings account's APY idle cash cannot
 * be spotted; without a promotion's end date the balance that must be cleared
 * before it cannot be dated. Setup asked for card APRs only, once.
 */

/** Which rate an account carries, from what it is: debts an APR, cash an APY. */
export type RateKind = "apr" | "apy" | null;

export function rateKindFor(type: string, role: string | null | undefined): RateKind {
  if (type === "liability") return "apr";
  if (role === "checking" || role === "savings") return "apy";
  return null;
}

const dayIn = (iso?: string | null) => (iso ? iso.slice(0, 10) : "");

/** A stored date read as the calendar day it is, not shifted by the local zone. */
const monthOf = (iso: string) =>
  new Date(iso).toLocaleDateString(undefined, { month: "short", year: "numeric", timeZone: "UTC" });

/** "22.49% APR", "0% until Mar 2027, then 22.49%", "4.20% APY", or null when none is recorded. */
export function rateLabel(kind: RateKind, t: AccountTerms | undefined): string | null {
  if (!kind || !t) return null;
  const rate = (f: number) => `${Number((f * 100).toFixed(2))}%`;
  if (kind === "apy") return t.apy != null ? `${rate(t.apy)} APY` : null;
  if (t.promo_apr != null && t.promo_expires_on && new Date(t.promo_expires_on) > new Date()) {
    const until = `${rate(t.promo_apr)} until ${monthOf(t.promo_expires_on)}`;
    return t.apr != null ? `${until}, then ${rate(t.apr)}` : until;
  }
  return t.apr != null ? `${rate(t.apr)} APR` : null;
}

/**
 * The rate inputs for an account being edited. Uncontrolled and named, so
 * they submit with the rest of the edit form; ratesFromForm reads them back.
 */
export function RateFields({ kind, terms }: { kind: RateKind; terms: AccountTerms | undefined }) {
  const [promo, setPromo] = useState(terms?.promo_apr != null);
  if (!kind) return null;

  const box = "rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500";
  return (
    <div className="space-y-4 rounded-xl border border-slate-200 dark:border-slate-700 p-3">
      <div className="space-y-2">
        <Label htmlFor={`edit-${kind}`} className="text-slate-700 dark:text-slate-300">
          {kind === "apr" ? "Interest rate (APR)" : "Interest it earns (APY)"}
        </Label>
        <div className="flex items-center gap-2">
          <Input
            id={`edit-${kind}`}
            name={kind}
            inputMode="decimal"
            placeholder={kind === "apr" ? "22.49" : "4.20"}
            defaultValue={pctIn(kind === "apr" ? terms?.apr : terms?.apy)}
            className={`w-32 ${box}`}
          />
          <span className="text-sm text-slate-500 dark:text-slate-400">%</span>
        </div>
        <p className="text-xs text-slate-500 dark:text-slate-400">
          {kind === "apr"
            ? "On your statement. It decides whether paying this down beats investing."
            : "On your bank's site. The plan compares it with your other cash accounts."}
        </p>
      </div>

      {kind === "apr" && (
        <>
          <label className="flex items-start gap-3 cursor-pointer">
            <input
              type="checkbox"
              name="promo_on"
              checked={promo}
              onChange={(e) => setPromo(e.target.checked)}
              className="mt-0.5 h-4 w-4 rounded border-slate-300 dark:border-slate-600 text-indigo-600 focus:ring-indigo-500 bg-white dark:bg-slate-900 cursor-pointer dark:checked:bg-indigo-500"
            />
            <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
              A promotional rate applies
              <span className="block text-xs font-normal text-slate-500 dark:text-slate-400">
                A 0% balance transfer or purchase offer. The plan dates clearing it before it ends.
              </span>
            </span>
          </label>
          {promo && (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="edit-promo-apr" className="text-slate-700 dark:text-slate-300">Promo rate</Label>
                <div className="flex items-center gap-2">
                  <Input
                    id="edit-promo-apr"
                    name="promo_apr"
                    inputMode="decimal"
                    defaultValue={terms?.promo_apr != null ? pctIn(terms.promo_apr) : "0"}
                    className={`w-24 ${box}`}
                  />
                  <span className="text-sm text-slate-500 dark:text-slate-400">%</span>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-promo-ends" className="text-slate-700 dark:text-slate-300">Ends on</Label>
                <Input
                  id="edit-promo-ends"
                  name="promo_expires_on"
                  type="date"
                  defaultValue={dayIn(terms?.promo_expires_on)}
                  className={box}
                />
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

/**
 * The rate changes in a submitted edit form: only what differs from what is
 * stored, so saving a renamed account does not rewrite its rates. An error
 * names the one thing that cannot be saved as entered.
 */
export function ratesFromForm(
  form: FormData,
  stored: AccountTerms | undefined,
): { patch: AccountTermsPatch; error?: string } {
  const patch: AccountTermsPatch = {};
  const text = (k: string) => (form.get(k) as string | null) ?? "";

  for (const k of ["apr", "apy"] as const) {
    if (!form.has(k)) continue;
    const change = rateChange(text(k), stored?.[k]);
    if (change !== undefined) patch[k] = change;
  }

  // Only a debt's form has the promotion. Its rate and end date are saved,
  // and cleared, together.
  if (form.has("apr")) {
    const had = stored?.promo_apr != null;
    if (form.has("promo_on")) {
      const rate = rateChange(text("promo_apr"), had ? stored?.promo_apr : undefined);
      const ends = text("promo_expires_on");
      if (!ends) return { patch, error: "Add the date the promotional rate ends." };
      const rateChanged = rate !== undefined || !had;
      if (rateChanged || ends !== dayIn(stored?.promo_expires_on)) {
        const value = rate === undefined ? (had ? stored!.promo_apr! : 0) : rate;
        if (value === null) return { patch, error: "Add the promotional rate, or untick it." };
        patch.promo_apr = value;
        patch.promo_expires_on = `${ends}T00:00:00Z`;
      }
    } else if (had) {
      patch.promo_apr = null;
      patch.promo_expires_on = null;
    }
  }
  return { patch };
}
