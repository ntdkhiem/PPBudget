"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { format } from "date-fns";
import { ArrowDownUp, PiggyBank, ShieldCheck } from "lucide-react";
import { apiFetch, getStoredToken, BudgetSummary } from "@/lib/api";
import { cn, formatCurrency } from "@/lib/utils";
import { StatCard } from "@/components/stat-card";
import { Skeleton } from "@/components/ui/skeleton";
import { useSummary } from "./dashboard-data";

function KpiValue({ loading, children, className }: { loading: boolean; children: ReactNode; className?: string }) {
  if (loading) return <Skeleton className="h-8 w-32" />;
  return <div className={cn("text-2xl font-bold font-heading text-slate-900 dark:text-white", className)}>{children}</div>;
}

function KpiDetail({ loading, children }: { loading: boolean; children: ReactNode }) {
  if (loading) return <Skeleton className="h-4 w-40 mt-2" />;
  return <div className="text-xs font-medium text-slate-500 dark:text-slate-400 mt-1">{children}</div>;
}

function Bar({ percent, className }: { percent: number; className: string }) {
  return (
    <div className="mt-auto pt-4">
      <div className="h-1.5 w-full rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
        <div className={cn("h-full rounded-full", className)} style={{ width: `${Math.min(Math.max(percent, 0), 100)}%` }} />
      </div>
    </div>
  );
}

export function CashFlowCard() {
  const { data: summary, isLoading } = useSummary();
  const inflow = summary?.in_period ?? 0;
  const outflow = summary?.out_period ?? 0;
  const net = inflow - outflow;

  return (
    <StatCard title="Net Cash Flow" icon={ArrowDownUp} tone="indigo" action="This period">
      <KpiValue loading={isLoading} className={net < 0 ? "text-rose-600 dark:text-rose-400" : undefined}>
        {net > 0 ? "+" : ""}
        {formatCurrency(net)}
      </KpiValue>
      <KpiDetail loading={isLoading}>
        <span className="text-emerald-600 dark:text-emerald-400">+{formatCurrency(inflow)} in</span>
        <span className="text-slate-300 dark:text-slate-600"> / </span>
        <span className="text-rose-600 dark:text-rose-400">-{formatCurrency(outflow)} out</span>
      </KpiDetail>
    </StatCard>
  );
}

export function SavingsRateCard() {
  const { data: summary, isLoading } = useSummary();
  const inflow = summary?.in_period ?? 0;
  const saved = inflow - (summary?.out_period ?? 0);
  const rate = inflow > 0 ? (saved / inflow) * 100 : null;
  const tone =
    rate === null ? undefined : rate >= 20 ? "text-emerald-600 dark:text-emerald-400" : rate > 0 ? undefined : "text-rose-600 dark:text-rose-400";

  return (
    <StatCard title="Savings Rate" icon={PiggyBank} tone="amber" action="This period">
      <KpiValue loading={isLoading} className={tone}>
        {rate === null ? "—" : `${rate.toFixed(1)}%`}
      </KpiValue>
      <KpiDetail loading={isLoading}>
        {rate === null
          ? "No income this period"
          : `Saved ${formatCurrency(Math.max(0, saved))} of ${formatCurrency(inflow)} income`}
      </KpiDetail>
      {!isLoading && rate !== null && <Bar percent={rate} className={rate >= 20 ? "bg-emerald-500" : "bg-amber-500"} />}
    </StatCard>
  );
}

export function SafeToSpendCard() {
  // Budgets are monthly, so this card always shows the current calendar month,
  // independent of the sidebar's date range.
  const now = new Date();
  const month = format(now, "yyyy-MM");
  const monthName = format(now, "MMMM");

  const { data: budgets, isLoading } = useQuery<BudgetSummary[]>({
    queryKey: ["budgets", month],
    queryFn: () => apiFetch<BudgetSummary[]>(`/budgets/summary?month=${month}`, {}, getStoredToken()),
  });

  const allocated = budgets?.reduce((acc, b) => acc + b.amount_cents, 0) ?? 0;
  const spent = budgets?.reduce((acc, b) => acc + b.spent_total, 0) ?? 0;
  const hasBudgets = allocated > 0;
  const spentPercent = hasBudgets ? (spent / allocated) * 100 : 0;

  return (
    <StatCard title="Safe to Spend" icon={ShieldCheck} tone="emerald" action={`${monthName} budget`}>
      <KpiValue loading={isLoading} className={hasBudgets ? undefined : "text-slate-500 dark:text-slate-400"}>
        {hasBudgets ? formatCurrency(Math.max(0, allocated - spent)) : "—"}
      </KpiValue>
      <KpiDetail loading={isLoading}>
        {hasBudgets ? (
          <>
            {formatCurrency(spent)} of {formatCurrency(allocated)} spent
            {spent > allocated && <span className="text-rose-600 dark:text-rose-400"> · over by {formatCurrency(spent - allocated)}</span>}
          </>
        ) : (
          <Link href="/budgets" className="text-indigo-600 hover:text-indigo-700 dark:text-indigo-400">
            Set up a budget for {monthName}
          </Link>
        )}
      </KpiDetail>
      {!isLoading && hasBudgets && <Bar percent={spentPercent} className={spent > allocated ? "bg-rose-500" : "bg-emerald-500"} />}
    </StatCard>
  );
}
