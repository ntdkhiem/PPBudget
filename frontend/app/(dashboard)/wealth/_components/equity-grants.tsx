"use client";

import { useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Choice, Field, toFraction } from "./form-bits";
import {
  formatDate,
  pct,
  useDeleteEquityGrant,
  useEquityGrants,
  useSaveEquityGrant,
  type EquityGrant,
} from "./wealth-data";

/**
 * RSU and ESPP grants.
 *
 * The engine had every equity rule and a table to read them from, but no way
 * to put a grant in, so the equity phase could never say anything. The two
 * dates asked for here -- the next vest and the next ESPP purchase -- are what
 * it schedules, ahead of the phase order.
 */

type Kind = "rsu" | "espp";
type Draft = Record<string, string>;

const FREQUENCIES = [
  { value: "monthly", label: "Monthly" },
  { value: "quarterly", label: "Quarterly" },
  { value: "semiannual", label: "Twice a year" },
  { value: "annual", label: "Yearly" },
  { value: "cliff", label: "All at once" },
];

const WINDOWS = [
  { value: "none", label: "No blackout" },
  { value: "quarterly", label: "Quarterly windows" },
  { value: "event_based", label: "Event-based" },
  { value: "unknown", label: "Not sure" },
];

const YES_NO = [
  { value: "yes", label: "Yes" },
  { value: "no", label: "No" },
];

const FREQUENCY_LABEL: Record<string, string> = Object.fromEntries(
  FREQUENCIES.map((f) => [f.value, f.label.toLowerCase()]),
);

// The API speaks timestamps; date inputs speak calendar days.
const dateIn = (iso?: string) => (iso ? iso.slice(0, 10) : "");
const dateOut = (v: string) => (v ? `${v}T00:00:00Z` : undefined);
const pctIn = (f?: number) => (f == null ? "" : String(Number((f * 100).toFixed(4))));
const pctOut = (v: string) => (v ? (toFraction(v) ?? undefined) : undefined);
const numOut = (v: string) => {
  const n = Number.parseFloat(v.replace(/[^0-9.]/g, ""));
  return Number.isFinite(n) ? n : undefined;
};
const boolIn = (b?: boolean) => (b == null ? "" : b ? "yes" : "no");
const boolOut = (v: string) => (v === "yes" ? true : v === "no" ? false : undefined);

function draftFrom(g: EquityGrant): Draft {
  return {
    label: g.label ?? "",
    grant_date: dateIn(g.grant_date),
    total_shares: g.total_shares == null ? "" : String(g.total_shares),
    shares_vested: g.shares_vested == null ? "" : String(g.shares_vested),
    next_vest_date: dateIn(g.next_vest_date),
    vest_frequency: g.vest_frequency ?? "",
    vest_share: pctIn(g.vest_share),
    has_10b5_1: boolIn(g.has_10b5_1),
    blackout_policy: g.blackout_policy ?? "",
    supplemental_withholding_pct: pctIn(g.supplemental_withholding_pct),
    espp_purchase_date: dateIn(g.espp_purchase_date),
    espp_offering_start: dateIn(g.espp_offering_start),
    espp_discount_pct: pctIn(g.espp_discount_pct),
    espp_has_lookback: boolIn(g.espp_has_lookback),
    espp_contribution_pct: pctIn(g.espp_contribution_pct),
    espp_plan_max_pct: pctIn(g.espp_plan_max_pct),
  };
}

/**
 * The payload for one kind. A field left empty is sent as absent, which the
 * server stores as unknown rather than zero -- the same rule as the profile.
 */
function grantFrom(kind: Kind, d: Draft, id?: string): Partial<EquityGrant> & { kind: Kind } {
  const shared = { id, kind, label: d.label?.trim() || undefined };
  if (kind === "rsu") {
    return {
      ...shared,
      grant_date: dateOut(d.grant_date),
      total_shares: numOut(d.total_shares ?? ""),
      shares_vested: numOut(d.shares_vested ?? ""),
      next_vest_date: dateOut(d.next_vest_date),
      vest_frequency: (d.vest_frequency || undefined) as EquityGrant["vest_frequency"],
      vest_share: pctOut(d.vest_share ?? ""),
      has_10b5_1: boolOut(d.has_10b5_1),
      blackout_policy: (d.blackout_policy || undefined) as EquityGrant["blackout_policy"],
      supplemental_withholding_pct: pctOut(d.supplemental_withholding_pct ?? ""),
    };
  }
  return {
    ...shared,
    espp_purchase_date: dateOut(d.espp_purchase_date),
    espp_offering_start: dateOut(d.espp_offering_start),
    espp_discount_pct: pctOut(d.espp_discount_pct ?? ""),
    espp_has_lookback: boolOut(d.espp_has_lookback),
    espp_contribution_pct: pctOut(d.espp_contribution_pct ?? ""),
    espp_plan_max_pct: pctOut(d.espp_plan_max_pct ?? ""),
  };
}

/** One line saying what the grant puts on the plan. */
function summary(g: EquityGrant): string {
  if (g.kind === "espp") {
    const parts = [
      g.espp_purchase_date ? `Next purchase ${formatDate(g.espp_purchase_date)}` : "No purchase date yet",
    ];
    if (g.espp_discount_pct != null) {
      parts.push(`${pct(g.espp_discount_pct)} discount${g.espp_has_lookback ? " with lookback" : ""}`);
    }
    if (g.espp_contribution_pct != null) {
      parts.push(
        `${pct(g.espp_contribution_pct)} of pay` +
          (g.espp_plan_max_pct != null ? ` (plan max ${pct(g.espp_plan_max_pct)})` : ""),
      );
    }
    return parts.join(" · ");
  }
  const parts = [g.next_vest_date ? `Next vest ${formatDate(g.next_vest_date)}` : "No vest date yet"];
  if (g.vest_frequency) parts.push(`vests ${FREQUENCY_LABEL[g.vest_frequency] ?? g.vest_frequency}`);
  if (g.has_10b5_1) parts.push("sold through a 10b5-1 plan");
  return parts.join(" · ");
}

function GrantForm({
  kind,
  initial,
  saving,
  error,
  onSave,
  onCancel,
}: {
  kind: Kind;
  initial: Draft;
  saving: boolean;
  error?: string;
  onSave: (d: Draft) => void;
  onCancel: () => void;
}) {
  const [d, setD] = useState<Draft>(initial);
  const set = (k: string) => (v: string) => setD((prev) => ({ ...prev, [k]: v }));
  const input = (k: string, props: React.ComponentProps<typeof Input> = {}) => (
    <Input value={d[k] ?? ""} onChange={(e) => set(k)(e.target.value)} {...props} />
  );

  return (
    <div className="space-y-5 rounded-2xl border border-slate-200 p-5 dark:border-slate-800">
      <Field label="What to call it" hint={kind === "rsu" ? "e.g. New hire grant" : "e.g. ESPP"}>
        {input("label", { className: "max-w-xs" })}
      </Field>

      {kind === "rsu" ? (
        <>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field
              label="Next vest date"
              hint="From your equity portal's schedule. This is the date the plan counts down to."
            >
              {input("next_vest_date", { type: "date" })}
            </Field>
            <Field label="Share of the grant per vest" hint="In percent; 6.25 for quarterly over four years.">
              {input("vest_share", { inputMode: "decimal", className: "w-32" })}
            </Field>
          </div>
          <Field label="How often it vests">
            <Choice value={d.vest_frequency} options={FREQUENCIES} onChange={set("vest_frequency")} />
          </Field>
          <Field
            label="Do sales run through a 10b5-1 plan?"
            hint="A standing plan sells on schedule even inside a closed trading window."
          >
            <Choice value={d.has_10b5_1} options={YES_NO} onChange={set("has_10b5_1")} />
          </Field>
          <Field label="Trading windows">
            <Choice value={d.blackout_policy} options={WINDOWS} onChange={set("blackout_policy")} />
          </Field>
          <div className="grid gap-5 sm:grid-cols-3">
            <Field label="Grant date">{input("grant_date", { type: "date" })}</Field>
            <Field label="Total shares">{input("total_shares", { inputMode: "decimal" })}</Field>
            <Field label="Vested so far">{input("shares_vested", { inputMode: "decimal" })}</Field>
          </div>
          <Field
            label="Federal withholding on vests"
            hint="In percent. Leave empty if your employer withholds the standard 22%."
          >
            {input("supplemental_withholding_pct", { inputMode: "decimal", className: "w-32" })}
          </Field>
        </>
      ) : (
        <>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label="Next purchase date" hint="The end of the current offering period.">
              {input("espp_purchase_date", { type: "date" })}
            </Field>
            <Field label="Offering start">{input("espp_offering_start", { type: "date" })}</Field>
          </div>
          <div className="grid gap-5 sm:grid-cols-3">
            <Field label="Discount" hint="In percent; usually 15.">
              {input("espp_discount_pct", { inputMode: "decimal" })}
            </Field>
            <Field label="You contribute" hint="Percent of pay.">
              {input("espp_contribution_pct", { inputMode: "decimal" })}
            </Field>
            <Field label="Plan maximum" hint="Percent of pay.">
              {input("espp_plan_max_pct", { inputMode: "decimal" })}
            </Field>
          </div>
          <Field
            label="Does the price look back to the offering start?"
            hint="A lookback prices shares at the lower of the start and purchase dates."
          >
            <Choice value={d.espp_has_lookback} options={YES_NO} onChange={set("espp_has_lookback")} />
          </Field>
        </>
      )}

      {error && <p className="text-sm text-rose-600 dark:text-rose-400">{error}</p>}

      <div className="flex gap-2">
        <Button onClick={() => onSave(d)} disabled={saving}>
          {saving ? "Saving…" : "Save"}
        </Button>
        <Button variant="ghost" onClick={onCancel} disabled={saving}>
          Cancel
        </Button>
      </div>
    </div>
  );
}

export function EquityGrants() {
  const { data: grants } = useEquityGrants();
  const save = useSaveEquityGrant();
  const remove = useDeleteEquityGrant();
  // The grant being edited, or a new one of a kind; null when only listing.
  const [editing, setEditing] = useState<{ kind: Kind; id?: string; initial: Draft } | null>(null);

  const submit = (d: Draft) => {
    if (!editing) return;
    save.mutate(grantFrom(editing.kind, d, editing.id), { onSuccess: () => setEditing(null) });
  };

  const list = (grants ?? []).filter((g) => g.kind === "rsu" || g.kind === "espp");

  return (
    <section id="equity">
      <h2 className="mb-1 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        Company stock
      </h2>
      <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
        RSU vests and ESPP purchases go on your plan as dated actions, ahead of the phase order.
      </p>

      <div className="space-y-3">
        {list.map((g) =>
          editing?.id === g.id ? (
            <GrantForm
              key={g.id}
              kind={g.kind as Kind}
              initial={editing.initial}
              saving={save.isPending}
              error={save.error?.message}
              onSave={submit}
              onCancel={() => setEditing(null)}
            />
          ) : (
            <div
              key={g.id}
              className="flex items-start gap-3 rounded-2xl border border-slate-200 p-4 dark:border-slate-800"
            >
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-slate-900 dark:text-white">
                  {g.label || (g.kind === "rsu" ? "RSU grant" : "ESPP")}
                  <span className="ml-2 rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium uppercase text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                    {g.kind}
                  </span>
                </div>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{summary(g)}</p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Edit ${g.label || g.kind}`}
                onClick={() => {
                  save.reset();
                  setEditing({ kind: g.kind as Kind, id: g.id, initial: draftFrom(g) });
                }}
              >
                <Pencil className="h-4 w-4 text-slate-400" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Delete ${g.label || g.kind}`}
                disabled={remove.isPending}
                onClick={() => remove.mutate(g.id)}
              >
                <Trash2 className="h-4 w-4 text-slate-400" />
              </Button>
            </div>
          ),
        )}

        {editing && !editing.id ? (
          <GrantForm
            kind={editing.kind}
            initial={editing.initial}
            saving={save.isPending}
            error={save.error?.message}
            onSave={submit}
            onCancel={() => setEditing(null)}
          />
        ) : (
          <div className="flex flex-wrap gap-2">
            {(["rsu", "espp"] as Kind[]).map((kind) => (
              <Button
                key={kind}
                variant="outline"
                onClick={() => {
                  save.reset();
                  setEditing({ kind, initial: {} });
                }}
              >
                <Plus className="h-4 w-4" />
                {kind === "rsu" ? "Add an RSU grant" : "Add an ESPP"}
              </Button>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
