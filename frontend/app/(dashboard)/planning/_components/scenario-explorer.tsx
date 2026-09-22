"use client";

import { useMemo, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { FlaskConical, RotateCcw } from "lucide-react";
import { cn, formatCurrency } from "@/lib/utils";
import { DashboardCard } from "@/components/dashboard-card";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatMonths,
  formatPct,
  project,
  type Baseline,
  type FinancialPlan,
  type ScenarioInput,
} from "./planning-math";

// Same theme tokens the net worth chart uses, so both follow light/dark mode.
const BASELINE_COLOR = "var(--muted-foreground)";
const SCENARIO_COLOR = "var(--chart-1)";
const AXIS_COLOR = "var(--muted-foreground)";
const GRID_COLOR = "var(--border)";

const HORIZONS = [12, 24, 36] as const;
type Horizon = (typeof HORIZONS)[number];

const formatCompact = (val: number) =>
  new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(val / 100);

const DEFAULT_SCENARIO: ScenarioInput = {
  monthlyContributionCents: 0,
  incomeChange: 0,
  spendingCut: 0,
};

interface TooltipProps {
  active?: boolean;
  payload?: { value?: number | string; dataKey?: string | number; name?: string }[];
  label?: string | number;
}

function ProjectionTooltip({ active, payload, label }: TooltipProps) {
  if (!active || !payload?.length) return null;
  return (
    <div className="bg-white dark:bg-slate-800 p-3 rounded-xl shadow-lg border border-slate-200 dark:border-slate-700/60">
      <p className="text-slate-500 dark:text-slate-400 text-xs mb-1.5 font-medium">
        {Number(label) === 0 ? "Today" : `In ${formatMonths(Number(label))}`}
      </p>
      {payload.map((entry) => (
        <p key={String(entry.dataKey)} className="text-sm font-semibold text-slate-900 dark:text-white">
          <span className="text-slate-500 dark:text-slate-400 font-normal">{entry.name}: </span>
          {formatCurrency(Number(entry.value))}
        </p>
      ))}
    </div>
  );
}

function SliderRow({
  label,
  value,
  display,
  min,
  max,
  step,
  onChange,
}: {
  label: string;
  value: number;
  display: string;
  min: number;
  max: number;
  step: number;
  onChange: (v: number) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-sm font-medium text-slate-900 dark:text-white">{label}</span>
        <span className="text-sm font-semibold font-heading tabular-nums text-indigo-600 dark:text-indigo-400">
          {display}
        </span>
      </div>
      <Slider
        value={[value]}
        min={min}
        max={max}
        step={step}
        onValueChange={([v]) => onChange(v)}
        aria-label={label}
      />
    </div>
  );
}

function DeltaStat({
  label,
  baseline,
  scenario,
  better,
}: {
  label: string;
  baseline: string;
  scenario: string;
  /** "up" when a larger scenario value is good, "down" when smaller is good. */
  better?: "up" | "down" | null;
}) {
  return (
    <div className="rounded-xl border border-slate-200 p-3 dark:border-slate-800">
      <div className="text-xs font-medium text-slate-500 dark:text-slate-400">{label}</div>
      <div className="mt-1.5 flex items-baseline gap-2">
        {/* Striking through an identical value reads as a change that isn't there. */}
        {baseline !== scenario && (
          <span className="text-sm text-slate-400 line-through dark:text-slate-600">{baseline}</span>
        )}
        <span
          className={cn(
            "text-base font-bold font-heading",
            better === "up"
              ? "text-emerald-600 dark:text-emerald-400"
              : better === "down"
                ? "text-rose-600 dark:text-rose-400"
                : "text-slate-900 dark:text-white",
          )}
        >
          {scenario}
        </span>
      </div>
    </div>
  );
}

export function ScenarioExplorer({
  baseline,
  plan,
  loading,
}: {
  baseline: Baseline;
  plan: FinancialPlan;
  loading: boolean;
}) {
  const [horizon, setHorizon] = useState<Horizon>(24);
  const [scenario, setScenario] = useState<ScenarioInput>(DEFAULT_SCENARIO);

  const projection = useMemo(
    () => project(baseline, plan, scenario, horizon),
    [baseline, plan, scenario, horizon],
  );

  const touched =
    scenario.monthlyContributionCents !== 0 ||
    scenario.incomeChange !== 0 ||
    scenario.spendingCut !== 0;

  // Contribution slider spans a sensible range around the current surplus.
  const contributionMax = Math.max(100000, Math.abs(baseline.monthlySurplus) * 2);

  if (loading) {
    return (
      <section className="space-y-4">
        <Skeleton className="h-7 w-44" />
        <Skeleton className="h-80 w-full rounded-3xl" />
      </section>
    );
  }

  const horizonToggle = (
    <div className="flex bg-slate-100 dark:bg-slate-800/80 p-1 rounded-lg">
      {HORIZONS.map((h) => (
        <button
          key={h}
          onClick={() => setHorizon(h)}
          aria-pressed={horizon === h}
          className={cn(
            "px-3 py-1.5 text-xs font-medium rounded-md transition-colors",
            horizon === h
              ? "bg-white dark:bg-slate-700 text-indigo-600 dark:text-indigo-400 shadow-sm"
              : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300",
          )}
        >
          {h}mo
        </button>
      ))}
    </div>
  );

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
          What if?
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
          Move a slider to see the path change against your current trajectory.
        </p>
      </div>

      <DashboardCard className="space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <FlaskConical className="w-5 h-5 text-indigo-600 dark:text-indigo-400" />
            <span className="text-sm font-semibold text-slate-900 dark:text-white">
              Scenario vs. current path
            </span>
          </div>
          <div className="flex items-center gap-2">
            {touched && (
              <Button
                variant="ghost"
                onClick={() => setScenario(DEFAULT_SCENARIO)}
                className="text-slate-700 dark:text-slate-200"
              >
                <RotateCcw className="w-4 h-4 mr-2" />
                Reset
              </Button>
            )}
            {horizonToggle}
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <SliderRow
            label="Extra saved per month"
            value={scenario.monthlyContributionCents}
            display={`${scenario.monthlyContributionCents >= 0 ? "+" : ""}${formatCurrency(scenario.monthlyContributionCents)}`}
            min={-contributionMax}
            max={contributionMax}
            step={5000}
            onChange={(v) => setScenario((s) => ({ ...s, monthlyContributionCents: v }))}
          />
          <SliderRow
            label="Income change"
            value={scenario.incomeChange}
            display={`${scenario.incomeChange >= 0 ? "+" : ""}${formatPct(scenario.incomeChange)}`}
            min={-0.5}
            max={0.5}
            step={0.01}
            onChange={(v) => setScenario((s) => ({ ...s, incomeChange: v }))}
          />
          <SliderRow
            label="Discretionary spending cut"
            value={scenario.spendingCut}
            display={formatPct(scenario.spendingCut)}
            min={0}
            max={1}
            step={0.05}
            onChange={(v) => setScenario((s) => ({ ...s, spendingCut: v }))}
          />
        </div>

        <div className="h-72 w-full" role="img" aria-label="Projected net worth, current path versus scenario">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={projection.points} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="scenarioFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" style={{ stopColor: SCENARIO_COLOR, stopOpacity: 0.3 }} />
                  <stop offset="95%" style={{ stopColor: SCENARIO_COLOR, stopOpacity: 0 }} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" vertical={false} stroke={GRID_COLOR} />
              <XAxis
                dataKey="monthIndex"
                tickFormatter={(v) => (Number(v) === 0 ? "now" : `${v}mo`)}
                stroke={GRID_COLOR}
                tickLine={false}
                dy={10}
                tick={{ fill: AXIS_COLOR, fontSize: 12 }}
                minTickGap={24}
              />
              <YAxis
                tickFormatter={formatCompact}
                stroke={GRID_COLOR}
                tickLine={false}
                width={65}
                tick={{ fill: AXIS_COLOR, fontSize: 12 }}
              />
              <Tooltip
                content={<ProjectionTooltip />}
                cursor={{ stroke: AXIS_COLOR, strokeWidth: 1, strokeDasharray: "4 4" }}
              />
              <Legend
                verticalAlign="top"
                height={32}
                iconType="plainline"
                wrapperStyle={{ fontSize: 12, color: AXIS_COLOR }}
              />
              <Area
                type="monotone"
                dataKey="baselineNetWorth"
                name="Current path"
                stroke={BASELINE_COLOR}
                strokeWidth={2}
                strokeDasharray="5 4"
                fill="none"
                dot={false}
              />
              <Area
                type="monotone"
                dataKey="scenarioNetWorth"
                name="Scenario"
                stroke={SCENARIO_COLOR}
                strokeWidth={3}
                fill="url(#scenarioFill)"
                dot={false}
                activeDot={{ r: 6, fill: SCENARIO_COLOR, stroke: "var(--card)", strokeWidth: 2 }}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <DeltaStat
            label={`Net worth in ${horizon} months`}
            baseline={formatCompact(projection.baselineFinalNetWorth)}
            scenario={formatCompact(projection.scenarioFinalNetWorth)}
            better={
              projection.scenarioFinalNetWorth === projection.baselineFinalNetWorth
                ? null
                : projection.scenarioFinalNetWorth > projection.baselineFinalNetWorth
                  ? "up"
                  : "down"
            }
          />
          <DeltaStat
            label="Emergency fund complete"
            baseline={
              projection.baselineEfFundedMonth === null
                ? `not within ${horizon}mo`
                : formatMonths(projection.baselineEfFundedMonth)
            }
            scenario={
              projection.scenarioEfFundedMonth === null
                ? `not within ${horizon}mo`
                : formatMonths(projection.scenarioEfFundedMonth)
            }
            better={
              projection.scenarioEfFundedMonth === projection.baselineEfFundedMonth
                ? null
                : projection.scenarioEfFundedMonth === null
                  ? "down"
                  : projection.baselineEfFundedMonth === null ||
                      projection.scenarioEfFundedMonth < projection.baselineEfFundedMonth
                    ? "up"
                    : "down"
            }
          />
          <DeltaStat
            label={`Months of cover at ${horizon}mo`}
            baseline={
              projection.points.at(-1)?.baselineEfMonths == null
                ? "—"
                : `${projection.points.at(-1)!.baselineEfMonths!.toFixed(1)} mo`
            }
            scenario={
              projection.points.at(-1)?.scenarioEfMonths == null
                ? "—"
                : `${projection.points.at(-1)!.scenarioEfMonths!.toFixed(1)} mo`
            }
            better={null}
          />
        </div>

        <p className="text-xs text-slate-400 dark:text-slate-500">
          Straight-line projection: your current monthly surplus, compounded at{" "}
          {formatPct(plan.assumptions.invest_return_apr, 1)} a year. It does not model taxes,
          irregular income, or market swings. The discretionary cut applies to wants and unbudgeted
          spending only.
        </p>
      </DashboardCard>
    </section>
  );
}
