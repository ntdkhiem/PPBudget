"use client";

import { Minus, TrendingDown, TrendingUp } from "lucide-react";
import { cn, formatCurrency } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatPct,
  formatSignedPct,
  STRATEGY_PROFILES,
  type FinancialPlan,
  type Trend,
} from "./planning-math";

function TrendCell({ trend, target }: { trend: Trend; target?: string }) {
  const isRate = trend.key === "savingsRate";
  const Icon =
    trend.direction === "flat" ? Minus : trend.direction === "up" ? TrendingUp : TrendingDown;

  return (
    <div className="min-w-0 flex-1 px-4 py-3 sm:py-0">
      <div className="text-xs font-medium text-slate-500 dark:text-slate-400">{trend.label}</div>

      <div className="mt-1 flex items-baseline gap-2">
        <span className="text-lg font-bold font-heading text-slate-900 dark:text-white tabular-nums">
          {isRate ? formatPct(trend.current, 1) : formatCurrency(trend.current)}
        </span>

        {trend.unknown ? (
          <span className="text-xs font-medium text-slate-400 dark:text-slate-500">
            not enough history
          </span>
        ) : (
          <span
            className={cn(
              "inline-flex items-center gap-0.5 text-xs font-semibold",
              trend.direction === "flat"
                ? "text-slate-400 dark:text-slate-500"
                : trend.good
                  ? "text-emerald-600 dark:text-emerald-400"
                  : "text-rose-600 dark:text-rose-400",
            )}
          >
            <Icon className="h-3.5 w-3.5" />
            {trend.direction === "flat"
              ? "steady"
              : isRate
                ? `${trend.change > 0 ? "+" : ""}${(trend.change * 100).toFixed(1)} pts`
                : formatSignedPct(trend.change)}
          </span>
        )}
      </div>

      {target && <div className="mt-0.5 text-xs text-slate-400 dark:text-slate-500">{target}</div>}
    </div>
  );
}

export function TrendStrip({
  trends,
  plan,
  loading,
}: {
  trends: Trend[];
  plan: FinancialPlan;
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-24 w-full rounded-2xl" />;
  }

  const targetRate = STRATEGY_PROFILES[plan.strategy].targetSavingsRate;

  return (
    <div className="rounded-2xl border border-slate-200 bg-white dark:border-slate-800/60 dark:bg-slate-900/80">
      <div className="flex flex-col divide-y divide-slate-100 sm:flex-row sm:divide-y-0 sm:divide-x dark:divide-slate-800">
        {trends.map((trend) => (
          <TrendCell
            key={trend.key}
            trend={trend}
            target={
              trend.key === "savingsRate"
                ? `${formatPct(targetRate)} target on ${STRATEGY_PROFILES[plan.strategy].label.toLowerCase()}`
                : undefined
            }
          />
        ))}
      </div>
      <p className="border-t border-slate-100 px-4 py-2 text-xs text-slate-400 dark:border-slate-800 dark:text-slate-500">
        Latest complete month, with the change measured across the two halves of the window — one
        odd month on its own does not make a trend.
      </p>
    </div>
  );
}
