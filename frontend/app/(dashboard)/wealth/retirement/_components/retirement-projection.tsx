"use client";

import { useMemo, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { LineChart, RotateCcw } from "lucide-react";
import { cn } from "@/lib/utils";
import { DashboardCard } from "@/components/dashboard-card";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { Skeleton } from "@/components/ui/skeleton";
import {
  centsToDollars,
  formatCompactCents,
  formatPct,
  project,
  type RetirementInputs,
  type RetirementProfile,
} from "./retirement-math";

// Same theme tokens as the other charts, so light/dark follow automatically.
const BASE_COLOR = "var(--muted-foreground)";
const SCENARIO_COLOR = "var(--chart-1)";
const TARGET_COLOR = "var(--chart-2)";
const AXIS_COLOR = "var(--muted-foreground)";
const GRID_COLOR = "var(--border)";

interface Overrides {
  extraMonthlyCents: number;
  returnDelta: number;
  ageDelta: number;
}

const NO_OVERRIDES: Overrides = { extraMonthlyCents: 0, returnDelta: 0, ageDelta: 0 };

interface TooltipProps {
  active?: boolean;
  payload?: { value?: number | string; dataKey?: string | number; name?: string }[];
  label?: string | number;
}

function ProjectionTooltip({ active, payload, label }: TooltipProps) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-3 shadow-lg dark:border-slate-700/60 dark:bg-slate-800">
      <p className="mb-1.5 text-xs font-medium text-slate-600 dark:text-slate-400">Age {label}</p>
      {payload.map((entry) => (
        <p
          key={String(entry.dataKey)}
          className="text-sm font-semibold text-slate-900 dark:text-white"
        >
          <span className="font-normal text-slate-500 dark:text-slate-400">{entry.name}: </span>
          {centsToDollars(Number(entry.value))}
        </p>
      ))}
    </div>
  );
}

function SliderRow({
  label,
  display,
  value,
  min,
  max,
  step,
  onChange,
}: {
  label: string;
  display: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (v: number) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-sm font-medium text-slate-900 dark:text-white">{label}</span>
        <span className="font-heading text-sm font-semibold tabular-nums text-indigo-600 dark:text-indigo-400">
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

export function RetirementProjection({
  profile,
  inputs,
  loading,
}: {
  profile: RetirementProfile;
  inputs: RetirementInputs;
  loading: boolean;
}) {
  const [overrides, setOverrides] = useState<Overrides>(NO_OVERRIDES);

  const baseline = useMemo(() => project(profile, inputs), [profile, inputs]);

  const scenario = useMemo(
    () =>
      project(profile, inputs, {
        extraMonthlyCents: overrides.extraMonthlyCents,
        realReturn: inputs.realReturn + overrides.returnDelta,
        retirementAge: profile.target_retirement_age + overrides.ageDelta,
      }),
    [profile, inputs, overrides],
  );

  // The two paths run to different ages when the age slider moves, so the
  // chart is keyed on age and each series carries its own value.
  const data = useMemo(() => {
    const byAge = new Map<number, { age: number; base?: number; scen?: number }>();
    for (const p of baseline.years) {
      byAge.set(p.age, { age: p.age, base: p.balanceCents });
    }
    for (const p of scenario.years) {
      const row = byAge.get(p.age) ?? { age: p.age };
      row.scen = p.balanceCents;
      byAge.set(p.age, row);
    }
    return [...byAge.values()].sort((a, b) => a.age - b.age);
  }, [baseline, scenario]);

  if (loading) return <Skeleton className="h-96 w-full rounded-3xl" />;

  const touched =
    overrides.extraMonthlyCents !== 0 || overrides.returnDelta !== 0 || overrides.ageDelta !== 0;

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
          What if?
        </h2>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Three levers move this number: how much you add, what it earns, and how long it has.
        </p>
      </div>

      <DashboardCard className="space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <LineChart className="h-5 w-5 text-indigo-600 dark:text-indigo-400" />
            <span className="text-sm font-semibold text-slate-900 dark:text-white">
              Projected balance vs. your number
            </span>
          </div>
          {touched && (
            <Button
              variant="ghost"
              onClick={() => setOverrides(NO_OVERRIDES)}
              className="text-slate-700 dark:text-slate-200"
            >
              <RotateCcw className="mr-2 h-4 w-4" />
              Reset
            </Button>
          )}
        </div>

        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          <SliderRow
            label="Extra saved per month"
            display={`+${centsToDollars(overrides.extraMonthlyCents)}`}
            value={overrides.extraMonthlyCents}
            min={0}
            max={500000}
            step={5000}
            onChange={(v) => setOverrides((o) => ({ ...o, extraMonthlyCents: v }))}
          />
          <SliderRow
            label="Return vs. your assumption"
            display={`${overrides.returnDelta >= 0 ? "+" : ""}${formatPct(overrides.returnDelta, 1)}`}
            value={overrides.returnDelta}
            min={-0.04}
            max={0.04}
            step={0.005}
            onChange={(v) => setOverrides((o) => ({ ...o, returnDelta: v }))}
          />
          <SliderRow
            label="Retirement age"
            display={`${profile.target_retirement_age + overrides.ageDelta}`}
            value={overrides.ageDelta}
            min={-10}
            max={10}
            step={1}
            onChange={(v) => setOverrides((o) => ({ ...o, ageDelta: v }))}
          />
        </div>

        <div
          className="h-80 w-full"
          role="img"
          aria-label="Projected retirement balance against the target"
        >
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="retirementFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" style={{ stopColor: SCENARIO_COLOR, stopOpacity: 0.3 }} />
                  <stop offset="95%" style={{ stopColor: SCENARIO_COLOR, stopOpacity: 0 }} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" vertical={false} stroke={GRID_COLOR} />
              <XAxis
                dataKey="age"
                stroke={GRID_COLOR}
                tickLine={false}
                dy={10}
                tick={{ fill: AXIS_COLOR, fontSize: 12 }}
                minTickGap={24}
              />
              <YAxis
                tickFormatter={formatCompactCents}
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
                wrapperStyle={{ fontSize: 12 }}
                /* Recharts colours each label with its own series colour,
                   overriding the wrapper. On a dark surface the brand indigo
                   lands at 4.45:1 -- just under the 4.5 small text needs. The
                   swatch beside each label already carries the colour identity,
                   so the text does not have to. */
                formatter={(value: string) => (
                  <span style={{ color: AXIS_COLOR }}>{value}</span>
                )}
              />
              {inputs.targetNestEggCents > 0 && (
                <ReferenceLine
                  y={inputs.targetNestEggCents}
                  stroke={TARGET_COLOR}
                  strokeDasharray="6 4"
                  strokeWidth={2}
                  label={{
                    value: `Target ${formatCompactCents(inputs.targetNestEggCents)}`,
                    position: "insideTopLeft",
                    fill: TARGET_COLOR,
                    fontSize: 12,
                  }}
                />
              )}
              <Area
                type="monotone"
                dataKey="base"
                name="Current plan"
                stroke={BASE_COLOR}
                strokeWidth={2}
                strokeDasharray="5 4"
                fill="none"
                dot={false}
                connectNulls
              />
              <Area
                type="monotone"
                dataKey="scen"
                name="Scenario"
                stroke={SCENARIO_COLOR}
                strokeWidth={3}
                fill="url(#retirementFill)"
                dot={false}
                connectNulls
                activeDot={{ r: 6, fill: SCENARIO_COLOR, stroke: "var(--card)", strokeWidth: 2 }}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <Stat
            label="At retirement"
            base={centsToDollars(baseline.projectedCents)}
            scenario={centsToDollars(scenario.projectedCents)}
            better={
              scenario.projectedCents === baseline.projectedCents
                ? null
                : scenario.projectedCents > baseline.projectedCents
            }
          />
          <Stat
            label="Against your number"
            base={formatPct(baseline.fundedShare)}
            scenario={formatPct(scenario.fundedShare)}
            better={
              scenario.fundedShare === baseline.fundedShare
                ? null
                : scenario.fundedShare > baseline.fundedShare
            }
          />
          <Stat
            label="Shortfall"
            base={baseline.gapCents > 0 ? centsToDollars(baseline.gapCents) : "none"}
            scenario={scenario.gapCents > 0 ? centsToDollars(scenario.gapCents) : "none"}
            better={
              scenario.gapCents === baseline.gapCents ? null : scenario.gapCents < baseline.gapCents
            }
          />
        </div>

        <p className="text-xs text-slate-600 dark:text-slate-400">
          Today&rsquo;s dollars, compounding at a {formatPct(inputs.realReturn, 1)} real return with
          contributions applied at year end. It assumes steady contributions and steady returns —
          real markets do neither, and it models no taxes, fees, or Social Security.
        </p>
      </DashboardCard>
    </section>
  );
}

function Stat({
  label,
  base,
  scenario,
  better,
}: {
  label: string;
  base: string;
  scenario: string;
  better: boolean | null;
}) {
  return (
    <div className="rounded-xl border border-slate-200 p-3 dark:border-slate-800">
      <div className="text-xs font-medium text-slate-600 dark:text-slate-400">{label}</div>
      <div className="mt-1.5 flex items-baseline gap-2">
        {/* Striking through an unchanged value would imply a change that isn't there. */}
        {base !== scenario && (
          <span className="text-sm text-slate-400 line-through dark:text-slate-600">{base}</span>
        )}
        <span
          className={cn(
            "font-heading text-base font-bold",
            better === true
              ? "text-emerald-600 dark:text-emerald-400"
              : better === false
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
