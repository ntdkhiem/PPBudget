"use client";

import Link from "next/link";
import { CloudRain, ShieldCheck, TriangleAlert, Unlock } from "lucide-react";
import { cn, formatCurrency } from "@/lib/utils";
import { StatCard } from "@/components/stat-card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  computeMovable,
  computeVariance,
  coverageWarning,
  emergencyFundMetric,
  formatPct,
  type Baseline,
  type FinancialPlan,
  type Health,
} from "./planning-math";
import type { PlanningMonth } from "@/lib/api";

const VALUE_TEXT: Record<Health, string> = {
  good: "text-emerald-600 dark:text-emerald-400",
  warn: "text-amber-600 dark:text-amber-400",
  bad: "text-rose-600 dark:text-rose-400",
  unknown: "text-slate-400 dark:text-slate-500",
};

const BAR: Record<Health, string> = {
  good: "bg-emerald-500",
  warn: "bg-amber-500",
  bad: "bg-rose-500",
  unknown: "bg-slate-300 dark:bg-slate-700",
};

const TONE: Record<Health, "emerald" | "amber" | "rose" | "indigo"> = {
  good: "emerald",
  warn: "amber",
  bad: "rose",
  unknown: "indigo",
};

function Bar({ fill, status }: { fill: number; status: Health }) {
  return (
    <div className="mt-auto pt-4">
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
        <div
          className={cn("h-full rounded-full transition-all", BAR[status])}
          style={{ width: `${Math.min(Math.max(fill * 100, 0), 100)}%` }}
        />
      </div>
    </div>
  );
}

export function PositionCards({
  baseline,
  plan,
  months,
  loading,
}: {
  baseline: Baseline;
  plan: FinancialPlan;
  months: PlanningMonth[];
  loading: boolean;
}) {
  if (loading) {
    return (
      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-44 w-full rounded-3xl" />
        ))}
      </div>
    );
  }

  const ef = emergencyFundMetric(baseline, plan.emergency_fund.target_months);
  const movable = computeMovable(baseline);
  const variance = computeVariance(baseline, plan, months);
  const warning = coverageWarning(baseline);

  // A wide gap between wants and wants-plus-unclassified means we genuinely
  // don't know how much is movable, so show a range rather than a false point.
  const movableIsRange = movable.unclassifiedCents > 0;

  const stressStatus: Health = variance.unknown
    ? "unknown"
    : variance.shortfallVsStressed <= 0
      ? "good"
      : "warn";

  return (
    <div className="space-y-4">
      {warning && (
        <div className="flex items-start gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-500/30 dark:bg-amber-500/10">
          <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" />
          <div className="text-sm text-amber-900 dark:text-amber-200">
            {warning}{" "}
            <Link href="/budgets" className="font-semibold underline underline-offset-2">
              Assign buckets in Budgets
            </Link>
            .
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        {/* 1. The anchor number. Absorbs what used to be a separate cash tile. */}
        <StatCard
          title="Emergency fund"
          icon={ShieldCheck}
          tone={TONE[ef.status]}
          action={`${plan.emergency_fund.target_months} mo target`}
        >
          <div className={cn("text-2xl font-bold font-heading", VALUE_TEXT[ef.status])}>
            {ef.value === null ? "—" : `${ef.value.toFixed(1)} mo`}
          </div>
          <p className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">{ef.note}</p>
          <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">
            {formatCurrency(baseline.liquidAssets)} liquid
            {baseline.usedOverrides.liquid ? " across the accounts you chose" : " across all assets"}
          </p>
          {ef.value !== null && (
            <Bar fill={ef.value / plan.emergency_fund.target_months} status={ef.status} />
          )}
        </StatCard>

        {/* 2. The number that says how much of the budget you can actually act on. */}
        <StatCard title="Movable money" icon={Unlock} tone="indigo" action="per month">
          <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
            {movableIsRange
              ? `${formatCurrency(movable.discretionaryCents)}–${formatCurrency(
                  movable.discretionaryCents + movable.unclassifiedCents,
                )}`
              : formatCurrency(movable.discretionaryCents)}
          </div>
          <p className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">
            of {formatCurrency(movable.outflowCents)} monthly spending.{" "}
            {formatCurrency(movable.committedCents)} is essentials you cannot easily move
            {movable.alreadySavingCents > 0 && (
              <>
                , and {formatCurrency(movable.alreadySavingCents)} is already going into savings
              </>
            )}
            .
          </p>
          {movableIsRange && (
            <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">
              {formatCurrency(movable.unclassifiedCents)} is unbudgeted, so it could be either.
            </p>
          )}
          <Bar fill={movable.maxMovableShare} status="unknown" />
        </StatCard>

        {/* 3. Averages hide the month the fund actually exists for. */}
        <StatCard
          title="Worst month"
          icon={CloudRain}
          tone={TONE[stressStatus]}
          action="stress test"
        >
          {variance.unknown ? (
            <>
              <div className="text-2xl font-bold font-heading text-slate-400 dark:text-slate-500">
                —
              </div>
              <p className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">
                Needs a couple of complete months before a worst case means anything.
              </p>
            </>
          ) : (
            <>
              <div
                className={cn("text-2xl font-bold font-heading", VALUE_TEXT[stressStatus])}
              >
                {formatCurrency(variance.worstEssentials)}
              </div>
              <p className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">
                Your priciest month of essentials, {formatPct(
                  variance.typicalEssentials > 0
                    ? variance.worstEssentials / variance.typicalEssentials - 1
                    : 0,
                )}{" "}
                above typical.
              </p>
              <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">
                {variance.shortfallVsStressed > 0
                  ? `Sized on that, ${plan.emergency_fund.target_months} months is ${formatCurrency(
                      variance.stressedTargetCents,
                    )} — you are ${formatCurrency(variance.shortfallVsStressed)} short.`
                  : `Covered even at that rate (${variance.stressedMonths?.toFixed(1)} months).`}
              </p>
              {variance.stressedMonths !== null && (
                <Bar
                  fill={variance.stressedMonths / plan.emergency_fund.target_months}
                  status={stressStatus}
                />
              )}
            </>
          )}
        </StatCard>
      </div>
    </div>
  );
}
