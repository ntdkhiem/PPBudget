"use client";

import { AlertTriangle, CircleAlert, Info } from "lucide-react";
import { cn, formatCurrency } from "@/lib/utils";
import { DashboardCard } from "@/components/dashboard-card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ACCOUNT_KIND_LABELS,
  centsToDollars,
  employerMatchFor,
  formatPct,
  maxEmployerMatchFor,
  MATCHABLE_KINDS,
  type Flag,
  type RetirementProfile,
} from "./retirement-math";
import type { PlanningAccount } from "@/lib/api";

export function AccountsTable({
  profile,
  accounts,
  flags,
  loading,
}: {
  profile: RetirementProfile;
  accounts: PlanningAccount[];
  flags: Flag[];
  loading: boolean;
}) {
  if (loading) return <Skeleton className="h-56 w-full rounded-3xl" />;

  const nonMatchFlags = flags.filter((f) => f.kind !== "unclaimed_match");

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
          Accounts and contributions
        </h2>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Balances come from your linked accounts. Contributions and employer terms are yours to
          maintain.
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

      <DashboardCard className="p-0 overflow-hidden">
        {profile.accounts.length === 0 ? (
          <div className="flex items-start gap-3 p-6">
            <Info className="mt-0.5 h-5 w-5 shrink-0 text-slate-400" />
            <p className="text-sm text-slate-500 dark:text-slate-400">
              No accounts linked yet. Open setup to choose which of your accounts are retirement
              savings and what you contribute to each.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left dark:border-slate-800">
                  {["Account", "Type", "Balance", "Your contribution", "Employer match", "Limit used"].map(
                    (h) => (
                      <th
                        key={h}
                        className="px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400 whitespace-nowrap"
                      >
                        {h}
                      </th>
                    ),
                  )}
                </tr>
              </thead>
              <tbody>
                {profile.accounts.map((ra) => {
                  const account = accounts.find((a) => a.id === ra.account_id);
                  const annual = ra.monthly_contribution_cents * 12;
                  const earned = employerMatchFor(ra, profile.gross_annual_income_cents);
                  const possible = maxEmployerMatchFor(ra, profile.gross_annual_income_cents);
                  const missingMatch = possible > earned;
                  const limit = ra.annual_limit_cents;
                  const overLimit = limit != null && annual > limit;

                  return (
                    <tr
                      key={ra.account_id}
                      className="border-b border-slate-100 last:border-0 dark:border-slate-800/60"
                    >
                      <td className="px-4 py-3 font-medium text-slate-900 dark:text-white whitespace-nowrap">
                        {account?.name ?? "Unlinked account"}
                      </td>
                      <td className="px-4 py-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                        {ACCOUNT_KIND_LABELS[ra.kind]}
                      </td>
                      <td className="px-4 py-3 tabular-nums text-slate-900 dark:text-white whitespace-nowrap">
                        {account ? formatCurrency(account.balance) : "—"}
                      </td>
                      <td className="px-4 py-3 tabular-nums text-slate-900 dark:text-white whitespace-nowrap">
                        {formatCurrency(ra.monthly_contribution_cents)}/mo
                        <span className="block text-xs text-slate-600 dark:text-slate-400">
                          {centsToDollars(annual)}/yr
                        </span>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        {!MATCHABLE_KINDS.includes(ra.kind) ? (
                          <span className="text-slate-500 dark:text-slate-400">n/a</span>
                        ) : possible === 0 ? (
                          <span className="text-slate-500 dark:text-slate-400">none set</span>
                        ) : (
                          <span
                            className={cn(
                              "tabular-nums font-medium",
                              missingMatch
                                ? "text-rose-600 dark:text-rose-400"
                                : "text-emerald-600 dark:text-emerald-400",
                            )}
                          >
                            {centsToDollars(earned)}
                            <span className="block text-xs font-normal text-slate-600 dark:text-slate-400">
                              of {centsToDollars(possible)} available
                            </span>
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        {limit == null ? (
                          <span className="text-slate-500 dark:text-slate-400">not set</span>
                        ) : (
                          <span
                            className={cn(
                              "tabular-nums font-medium",
                              overLimit
                                ? "text-amber-600 dark:text-amber-400"
                                : "text-slate-900 dark:text-white",
                            )}
                          >
                            {formatPct(annual / limit)}
                            <span className="block text-xs font-normal text-slate-600 dark:text-slate-400">
                              of {centsToDollars(limit)}
                            </span>
                          </span>
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

      <p className="text-xs text-slate-600 dark:text-slate-400">
        Contribution limits are the figures you entered, not looked up. They change annually —
        check them against the current IRS limits.
      </p>
    </section>
  );
}
