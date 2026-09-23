"use client";

import { ArrowRight, Check, Info, TrendingUp } from "lucide-react";
import { cn, formatCurrency } from "@/lib/utils";
import { DashboardCard } from "@/components/dashboard-card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatMonths,
  formatPct,
  STRATEGY_PROFILES,
  type FinancialPlan,
  type StageKind,
  type Strategy,
  type Waterfall,
} from "./planning-math";

const STAGE_BAR: Record<StageKind, string> = {
  starter_ef: "bg-indigo-500",
  high_apr_debt: "bg-rose-500",
  full_ef: "bg-amber-500",
  invest: "bg-emerald-500",
};

const STAGE_DOT: Record<StageKind, string> = {
  starter_ef: "bg-indigo-500",
  high_apr_debt: "bg-rose-500",
  full_ef: "bg-amber-500",
  invest: "bg-emerald-500",
};

function StrategyPicker({
  value,
  onChange,
  disabled,
}: {
  value: Strategy;
  onChange: (s: Strategy) => void;
  disabled: boolean;
}) {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
      {(Object.keys(STRATEGY_PROFILES) as Strategy[]).map((key) => {
        const profile = STRATEGY_PROFILES[key];
        const selected = key === value;
        return (
          <button
            key={key}
            type="button"
            disabled={disabled}
            onClick={() => onChange(key)}
            aria-pressed={selected}
            className={cn(
              "text-left rounded-xl border p-4 transition-colors disabled:opacity-60",
              selected
                ? "border-indigo-500 bg-indigo-50 dark:bg-indigo-500/10 dark:border-indigo-400"
                : "border-slate-200 hover:border-slate-300 dark:border-slate-800 dark:hover:border-slate-700",
            )}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="font-semibold text-sm text-slate-900 dark:text-white">
                {profile.label}
              </span>
              {selected && <Check className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />}
            </div>
            <div className="text-xs font-medium text-slate-600 dark:text-slate-400 mt-1">
              {formatPct(profile.targetSavingsRate)} of income saved
            </div>
            <p className="text-xs text-slate-600 dark:text-slate-400 mt-2">{profile.blurb}</p>
          </button>
        );
      })}
    </div>
  );
}

export function AllocationPlan({
  plan,
  waterfall,
  loading,
  onStrategyChange,
  saving,
}: {
  plan: FinancialPlan;
  /** Computed once on the page so every section describes the same plan. */
  waterfall: Waterfall;
  loading: boolean;
  onStrategyChange: (s: Strategy) => void;
  saving: boolean;
}) {
  const activeStage = waterfall.stages.find((s) => s.active);
  const investStage = waterfall.stages.find((s) => s.kind === "invest");

  if (loading) {
    return (
      <section className="space-y-4">
        <Skeleton className="h-7 w-52" />
        <Skeleton className="h-64 w-full rounded-3xl" />
      </section>
    );
  }

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
          Where the next dollar goes
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
          One pool, competing claims. Each priority takes the whole surplus until it is
          satisfied, so every stage below gets a real start date.
        </p>
      </div>

      <DashboardCard className="space-y-6">
        <StrategyPicker value={plan.strategy} onChange={onStrategyChange} disabled={saving} />

        <div className="space-y-4">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">
              Priority waterfall
            </h3>
            <span className="text-xs font-medium text-slate-600 dark:text-slate-400">
              {formatCurrency(waterfall.poolCents)}/mo surplus
              {waterfall.targetPoolCents > 0 && (
                <> · strategy targets {formatCurrency(waterfall.targetPoolCents)}</>
              )}
            </span>
          </div>

          {waterfall.poolCents <= 0 ? (
            <div className="flex items-start gap-3 rounded-xl border border-rose-200 bg-rose-50 p-4 dark:border-rose-500/30 dark:bg-rose-500/10">
              <Info className="w-5 h-5 shrink-0 text-rose-600 dark:text-rose-400 mt-0.5" />
              <p className="text-sm text-rose-900 dark:text-rose-200">
                You have no monthly surplus to allocate. Until income exceeds spending, no funding
                stage can make progress — the fix is in your spending, not in the ordering below.
              </p>
            </div>
          ) : (
            <>
              {/* Stacked bar: one segment per stage, sized by its share of the timeline. */}
              <div className="flex h-3 w-full rounded-full overflow-hidden bg-slate-100 dark:bg-slate-800">
                {waterfall.stages.map((stage) => {
                  const total = waterfall.crossoverMonths ?? 0;
                  const share =
                    stage.kind === "invest"
                      ? total === 0
                        ? 1
                        : 0.25
                      : total > 0
                        ? (stage.monthsToComplete ?? 0) / total
                        : 0;
                  if (share <= 0) return null;
                  return (
                    <div
                      key={stage.kind}
                      className={STAGE_BAR[stage.kind]}
                      style={{ flexGrow: share }}
                      title={`${stage.label}: ${formatMonths(stage.monthsToComplete)}`}
                    />
                  );
                })}
              </div>

              <ol className="space-y-3">
                {waterfall.stages.map((stage) => (
                  <li
                    key={stage.kind}
                    className={cn(
                      "flex items-start gap-3 rounded-xl border p-3",
                      stage.active
                        ? "border-indigo-300 bg-indigo-50/60 dark:border-indigo-500/40 dark:bg-indigo-500/10"
                        : "border-transparent",
                    )}
                  >
                    <span
                      className={cn(
                        "mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full",
                        stage.complete ? "bg-slate-300 dark:bg-slate-700" : STAGE_DOT[stage.kind],
                      )}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                        <span
                          className={cn(
                            "text-sm font-semibold",
                            stage.complete && !stage.blockedReason
                              ? "text-slate-400 line-through dark:text-slate-600"
                              : stage.complete
                                ? "text-slate-500 dark:text-slate-400"
                                : "text-slate-900 dark:text-white",
                          )}
                        >
                          {stage.label}
                        </span>
                        {stage.active && (
                          <span className="rounded-full bg-indigo-600 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-white">
                            Funding now
                          </span>
                        )}
                        {/* A stage held back for missing data is not "Done" —
                            calling it that implies the debt is handled. */}
                        {stage.complete &&
                          (stage.blockedReason ? (
                            <span className="text-xs font-medium text-amber-600 dark:text-amber-400">
                              Skipped
                            </span>
                          ) : (
                            <span className="text-xs font-medium text-emerald-600 dark:text-emerald-400">
                              Done
                            </span>
                          ))}
                      </div>

                      <p className="text-xs text-slate-600 dark:text-slate-400 mt-0.5">
                        {stage.blockedReason ?? stage.description}
                      </p>

                      {!stage.complete && (
                        <div className="text-xs font-medium text-slate-600 dark:text-slate-300 mt-1.5">
                          {stage.kind === "invest" ? (
                            <>
                              {formatCurrency(stage.monthlyCents)}/mo
                              {stage.startsInMonths != null && stage.startsInMonths > 0 && (
                                <> · starts in {formatMonths(stage.startsInMonths)}</>
                              )}
                            </>
                          ) : (
                            <>
                              {formatCurrency(stage.remainingCents ?? 0)} to go ·{" "}
                              {formatCurrency(stage.monthlyCents)}/mo ·{" "}
                              {stage.monthsToComplete === null
                                ? "never at this rate"
                                : `${formatMonths(stage.monthsToComplete)} of funding`}
                              {stage.startsInMonths != null && stage.startsInMonths > 0 && (
                                <> · begins in {formatMonths(stage.startsInMonths)}</>
                              )}
                            </>
                          )}
                        </div>
                      )}
                    </div>
                  </li>
                ))}
              </ol>

              {/* The headline: when funding ends and investing begins. */}
              <div className="flex items-start gap-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-500/30 dark:bg-emerald-500/10">
                <TrendingUp className="w-5 h-5 shrink-0 text-emerald-600 dark:text-emerald-400 mt-0.5" />
                <div className="text-sm text-emerald-900 dark:text-emerald-200">
                  {waterfall.crossoverMonths === null ? (
                    <>Your cushion never completes at the current surplus.</>
                  ) : waterfall.crossoverMonths === 0 ? (
                    <>
                      <strong>Your cushion is already complete.</strong> All{" "}
                      {formatCurrency(investStage?.monthlyCents ?? 0)} a month can go straight to
                      investing.
                    </>
                  ) : (
                    <>
                      <strong>Crossover in {formatMonths(waterfall.crossoverMonths)}.</strong> Until
                      then {formatCurrency(waterfall.poolCents)} a month goes to{" "}
                      {activeStage?.label.toLowerCase()}
                      <ArrowRight className="inline w-3.5 h-3.5 mx-1 align-[-2px]" />
                      after that, the same amount starts being invested.
                    </>
                  )}
                </div>
              </div>
            </>
          )}
        </div>
      </DashboardCard>
    </section>
  );
}
