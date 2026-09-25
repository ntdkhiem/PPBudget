"use client";

import Link from "next/link";
import { AlertTriangle, CircleAlert, Info } from "lucide-react";
import { cn, formatCurrency } from "@/lib/utils";
import { DashboardCard } from "@/components/dashboard-card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useSaveRetirementAccount, type PlanSummary } from "../../_components/wealth-data";
import {
  ACCOUNT_KIND_LABELS,
  centsToDollars,
  formatPct,
  isMatchable,
  type AccountKind,
  type Flag,
  type RetirementAccount,
  type RetirementProfile,
} from "./retirement-math";
import type { PlanningAccount } from "@/lib/api";

/**
 * Every investment account, with its tax treatment and what goes into it.
 *
 * Which accounts appear is decided by their role on the Accounts page; this
 * table only says how each is taxed and funded. Picking a treatment is what
 * used to be called linking, which the page never actually offered a way to do.
 *
 * Workplace plans are funded by the payroll rate on the Profile tab, so they
 * take no monthly figure here: one contribution rate, one match, both from the
 * server, rather than a second figure per account that could disagree with it.
 */
export function AccountsTable({
  profile,
  accounts,
  summary,
  flags,
  loading,
}: {
  profile: RetirementProfile;
  accounts: PlanningAccount[];
  summary: PlanSummary | undefined;
  flags: Flag[];
  loading: boolean;
}) {
  const save = useSaveRetirementAccount();

  if (loading) return <Skeleton className="h-56 w-full rounded-3xl" />;

  const nonMatchFlags = flags.filter((f) => f.kind !== "unclaimed_match");

  const update = (ra: RetirementAccount, patch: { kind?: AccountKind; monthly?: number }) => {
    const kind = patch.kind ?? ra.kind;
    // Terms need a treatment; until one is picked there is nothing to save.
    if (!kind) return;
    save.mutate({
      account_id: ra.account_id,
      kind,
      monthly_contribution: patch.monthly ?? ra.monthly_contribution_cents,
    });
  };

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
          Accounts and contributions
        </h2>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Every account marked as an investment on the{" "}
          <Link href="/accounts" className="font-medium underline underline-offset-2">
            Accounts page
          </Link>
          . Say how each one is taxed and what you put in.
        </p>
      </div>

      {nonMatchFlags.length > 0 && (
        <div className="space-y-2">
          {nonMatchFlags.map((flag, i) => (
            <div
              key={`${flag.kind}-${i}`}
              className={cn(
                "flex items-start gap-3 rounded-xl border p-4",
                flag.severity === "bad"
                  ? "border-rose-200 bg-rose-50 dark:border-rose-500/30 dark:bg-rose-500/10"
                  : "border-amber-200 bg-amber-50 dark:border-amber-500/30 dark:bg-amber-500/10",
              )}
            >
              {flag.severity === "bad" ? (
                <CircleAlert className="mt-0.5 h-5 w-5 shrink-0 text-rose-600 dark:text-rose-400" />
              ) : (
                <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" />
              )}
              <div
                className={cn(
                  "text-sm",
                  flag.severity === "bad"
                    ? "text-rose-900 dark:text-rose-200"
                    : "text-amber-900 dark:text-amber-200",
                )}
              >
                <span className="font-semibold">{flag.title}.</span> {flag.detail}
              </div>
            </div>
          ))}
        </div>
      )}

      <PayrollAndLimits summary={summary} />

      <DashboardCard className="p-0 overflow-hidden">
        {profile.accounts.length === 0 ? (
          <div className="flex items-start gap-3 p-6">
            <Info className="mt-0.5 h-5 w-5 shrink-0 text-slate-400" />
            <p className="text-sm text-slate-500 dark:text-slate-400">
              No investment accounts yet. Mark your 401(k), IRA, HSA and brokerage accounts as
              “Investment or retirement” on the Accounts page and they will appear here.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left dark:border-slate-800">
                  {["Account", "Tax treatment", "Balance", "You put in"].map((h) => (
                    <th
                      key={h}
                      className="px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400 whitespace-nowrap"
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {profile.accounts.map((ra) => {
                  const account = accounts.find((a) => a.id === ra.account_id);

                  return (
                    <tr
                      key={ra.account_id}
                      className="border-b border-slate-100 last:border-0 dark:border-slate-800/60"
                    >
                      <td className="px-4 py-3 font-medium text-slate-900 dark:text-white whitespace-nowrap">
                        {account?.name ?? "Unknown account"}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        <select
                          aria-label={`Tax treatment for ${account?.name ?? "this account"}`}
                          value={ra.kind ?? ""}
                          disabled={save.isPending}
                          onChange={(e) => update(ra, { kind: e.target.value as AccountKind })}
                          className={cn(
                            "rounded-lg border bg-white px-2 py-1.5 text-sm dark:bg-slate-900",
                            ra.kind
                              ? "border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-200"
                              : "border-amber-300 text-amber-700 dark:border-amber-500/40 dark:text-amber-300",
                          )}
                        >
                          <option value="" disabled>
                            Choose…
                          </option>
                          {(Object.keys(ACCOUNT_KIND_LABELS) as AccountKind[]).map((k) => (
                            <option key={k} value={k}>
                              {ACCOUNT_KIND_LABELS[k]}
                            </option>
                          ))}
                        </select>
                      </td>
                      <td className="px-4 py-3 tabular-nums text-slate-900 dark:text-white whitespace-nowrap">
                        {account ? formatCurrency(account.balance) : "—"}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        {isMatchable(ra.kind) ? (
                          <span className="text-slate-500 dark:text-slate-400">Through payroll</span>
                        ) : (
                        <div className="flex items-center gap-1.5">
                          <span className="text-slate-500 dark:text-slate-400">$</span>
                          <Input
                            // Remount when the saved figure changes, so the box
                            // shows what the server now holds.
                            key={`${ra.account_id}-${ra.monthly_contribution_cents}`}
                            aria-label={`Monthly contribution to ${account?.name ?? "this account"}`}
                            inputMode="decimal"
                            className="h-8 w-24"
                            disabled={!ra.kind || save.isPending}
                            defaultValue={String(ra.monthly_contribution_cents / 100)}
                            onBlur={(e) => {
                              const n = Number.parseFloat(e.target.value.replace(/[^0-9.]/g, ""));
                              const cents = Number.isFinite(n) ? Math.round(n * 100) : 0;
                              if (cents !== ra.monthly_contribution_cents) update(ra, { monthly: cents });
                            }}
                          />
                          <span className="text-xs text-slate-500 dark:text-slate-400">/mo</span>
                        </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </DashboardCard>

      {save.isError && (
        <p className="text-sm text-rose-600 dark:text-rose-400">
          Could not save that change: {save.error.message}
        </p>
      )}
    </section>
  );
}

/**
 * What goes in through payroll and the ceilings it goes in against, both as
 * the server worked them out -- the same figures the match action uses.
 */
function PayrollAndLimits({ summary }: { summary: PlanSummary | undefined }) {
  if (!summary) return null;
  const w = summary.workplace;
  const l = summary.limits;
  const missingMatch = w != null && w.match_available > w.match_earned;

  return (
    <div className="space-y-1.5 text-sm text-slate-600 dark:text-slate-300">
      {w ? (
        <p>
          <span className="font-medium text-slate-900 dark:text-white">Through payroll:</span>{" "}
          {formatPct(w.rate, 1)} of pay, {centsToDollars(w.annual_deferral)} a year.
          {w.match_available > 0 && (
            <>
              {" "}
              Employer match{" "}
              <span
                className={cn(
                  "font-medium tabular-nums",
                  missingMatch
                    ? "text-rose-600 dark:text-rose-400"
                    : "text-emerald-600 dark:text-emerald-400",
                )}
              >
                {centsToDollars(w.match_earned)}
              </span>{" "}
              of {centsToDollars(w.match_available)} available.
            </>
          )}
        </p>
      ) : (
        <p>
          Add your pay and contribution rate on the{" "}
          <Link href="/wealth/profile" className="font-medium underline underline-offset-2">
            Profile tab
          </Link>{" "}
          to see what goes in through payroll.
        </p>
      )}
      <p className="text-xs text-slate-500 dark:text-slate-400">
        {l.tax_year} limits: workplace plan {centsToDollars(l.workplace)} · IRA{" "}
        {centsToDollars(l.ira)} · HSA{" "}
        {l.hsa != null ? centsToDollars(l.hsa) : "depends on your coverage tier"}.
      </p>
    </div>
  );
}
