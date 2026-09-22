"use client";

import { useMemo, useState } from "react";
import { AlertTriangle, Plus, ShieldCheck, Target, Trash2 } from "lucide-react";
import { cn, formatCurrency, formatDate } from "@/lib/utils";
import { DashboardCard } from "@/components/dashboard-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  computeGoalPlan,
  computeGoalProgress,
  emergencyFundGoal,
  formatMonths,
  type Baseline,
  type FinancialPlan,
  type Goal,
  type GoalProgress,
  type Waterfall,
} from "./planning-math";
import type { PlanningAccount } from "@/lib/api";

function GoalRow({
  progress,
  isEmergencyFund,
  onDelete,
}: {
  progress: GoalProgress;
  isEmergencyFund: boolean;
  onDelete?: () => void;
}) {
  const pct = progress.fundedShare;
  const done = pct >= 1;

  return (
    <div className="rounded-xl border border-slate-200 p-4 dark:border-slate-800">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            {isEmergencyFund ? (
              <ShieldCheck className="w-4 h-4 shrink-0 text-indigo-600 dark:text-indigo-400" />
            ) : (
              <Target className="w-4 h-4 shrink-0 text-slate-400" />
            )}
            <span className="font-semibold text-sm text-slate-900 dark:text-white truncate">
              {progress.name}
            </span>
            {done && (
              <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400">
                Funded
              </span>
            )}
          </div>
          <div className="text-xs font-medium text-slate-500 dark:text-slate-400 mt-1">
            {formatCurrency(progress.currentCents)} of {formatCurrency(progress.targetCents)}
            {progress.targetDate && <> · by {formatDate(progress.targetDate)}</>}
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <span
            className={cn(
              "text-sm font-bold font-heading tabular-nums",
              done ? "text-emerald-600 dark:text-emerald-400" : "text-slate-900 dark:text-white",
            )}
          >
            {/* Rounding up to "100%" on a goal that is not actually funded
                reads as done. Floor it so only completion shows 100%. */}
            {done ? "100%" : `${Math.floor(pct * 100)}%`}
          </span>
          {onDelete && (
            <Button
              variant="ghost"
              size="icon"
              onClick={onDelete}
              aria-label={`Delete goal ${progress.name}`}
            >
              <Trash2 className="w-4 h-4 text-slate-400" />
            </Button>
          )}
        </div>
      </div>

      <div className="mt-3 h-2 w-full rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
        <div
          className={cn("h-full rounded-full transition-all", done ? "bg-emerald-500" : "bg-indigo-500")}
          style={{ width: `${Math.min(pct * 100, 100)}%` }}
        />
      </div>

      {!done && (
        <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
          <span className="text-slate-500 dark:text-slate-400">
            At plan:{" "}
            <span className="font-semibold text-slate-700 dark:text-slate-200">
              {progress.monthsAtCurrent === null
                ? "never funded"
                : formatMonths(progress.monthsAtCurrent)}
            </span>
          </span>
          {progress.requiredMonthly !== null && (
            <span className="text-slate-500 dark:text-slate-400">
              Needs:{" "}
              <span className="font-semibold text-slate-700 dark:text-slate-200">
                {formatCurrency(progress.requiredMonthly)}/mo
              </span>{" "}
              to hit the date
            </span>
          )}
          {/* Sequencing is the whole point, so say when this one's turn comes. */}
          {progress.startsInMonths != null && progress.startsInMonths > 0 && (
            <span className="text-slate-500 dark:text-slate-400">
              Starts:{" "}
              <span className="font-semibold text-slate-700 dark:text-slate-200">
                in {formatMonths(progress.startsInMonths)}
              </span>
            </span>
          )}
        </div>
      )}

      {progress.offTrack && (
        <div className="mt-3 flex items-start gap-2 rounded-lg bg-amber-50 p-2.5 dark:bg-amber-500/10">
          <AlertTriangle className="w-4 h-4 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" />
          <p className="text-xs text-amber-900 dark:text-amber-200">
            Off track for {progress.targetDate ? formatDate(progress.targetDate) : "the target date"}
            . You would need {formatCurrency(progress.requiredMonthly ?? 0)} a month, more than the
            plan currently routes here.
          </p>
        </div>
      )}
    </div>
  );
}

function AddGoalForm({ onAdd }: { onAdd: (goal: Goal) => void }) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [target, setTarget] = useState("");
  const [date, setDate] = useState("");

  const reset = () => {
    setName("");
    setTarget("");
    setDate("");
    setOpen(false);
  };

  const submit = () => {
    const cents = Math.round(Number.parseFloat(target) * 100);
    if (!name.trim() || !Number.isFinite(cents) || cents <= 0) return;
    onAdd({
      id: `goal_${Date.now().toString(36)}`,
      name: name.trim(),
      target_cents: cents,
      target_date: date || undefined,
      current_cents: 0,
      priority: 1,
    });
    reset();
  };

  if (!open) {
    return (
      <Button
        variant="outline"
        onClick={() => setOpen(true)}
        className="w-full text-slate-700 dark:text-slate-200"
      >
        <Plus className="w-4 h-4 mr-2" />
        Add a goal
      </Button>
    );
  }

  return (
    <div className="rounded-xl border border-dashed border-slate-300 p-4 space-y-3 dark:border-slate-700">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="goal-name">Name</Label>
          <Input
            id="goal-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="House down payment"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="goal-target">Target amount</Label>
          <Input
            id="goal-target"
            inputMode="decimal"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            placeholder="20000"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="goal-date">Target date (optional)</Label>
          <Input
            id="goal-date"
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
          />
        </div>
      </div>
      <div className="flex gap-2">
        <Button onClick={submit}>Add goal</Button>
        <Button variant="outline" onClick={reset} className="text-slate-700 dark:text-slate-200">
          Cancel
        </Button>
      </div>
    </div>
  );
}

export function GoalsProgress({
  baseline,
  plan,
  accounts,
  waterfall,
  loading,
  onPlanChange,
}: {
  baseline: Baseline;
  plan: FinancialPlan;
  accounts: PlanningAccount[];
  /** Shared with the allocation section so both timelines agree. */
  waterfall: Waterfall;
  loading: boolean;
  onPlanChange: (plan: FinancialPlan) => void;
}) {
  const now = useMemo(() => new Date(), []);

  const efGoal = useMemo(() => emergencyFundGoal(baseline, plan), [baseline, plan]);
  const efProgress = useMemo(
    () => computeGoalProgress(efGoal, accounts, waterfall.poolCents, now),
    [efGoal, accounts, waterfall.poolCents, now],
  );

  // Custom goals queue behind the cushion, then take the surplus one at a time
  // in priority order -- the same rule the waterfall above uses.
  const investStage = waterfall.stages.find((s) => s.kind === "invest");
  const goalPool = investStage?.monthlyCents ?? 0;

  const goalProgress = useMemo(
    () => computeGoalPlan(plan.goals, accounts, goalPool, waterfall.crossoverMonths, now),
    [plan.goals, accounts, goalPool, waterfall.crossoverMonths, now],
  );

  const addGoal = (goal: Goal) => onPlanChange({ ...plan, goals: [...plan.goals, goal] });
  const deleteGoal = (id: string) =>
    onPlanChange({ ...plan, goals: plan.goals.filter((g) => g.id !== id) });

  if (loading) {
    return (
      <section className="space-y-4">
        <Skeleton className="h-7 w-40" />
        <Skeleton className="h-48 w-full rounded-3xl" />
      </section>
    );
  }

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
          Goals
        </h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
          The emergency fund is derived from your essentials and target, never typed in. Everything
          else is yours to define.
        </p>
      </div>

      <DashboardCard className="space-y-3">
        {efGoal.target_cents <= 0 ? (
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Budget some categories as &ldquo;needs&rdquo; to give the emergency fund a target.
          </p>
        ) : (
          <GoalRow progress={efProgress} isEmergencyFund />
        )}

        {goalProgress.map((progress) => (
          <GoalRow
            key={progress.id}
            progress={progress}
            isEmergencyFund={false}
            onDelete={() => deleteGoal(progress.id)}
          />
        ))}

        {plan.goals.length > 0 && (
          <p className="text-xs text-slate-400 dark:text-slate-500">
            Goals are funded one at a time in priority order, each taking the full{" "}
            {formatCurrency(goalPool)} a month once the cushion above is complete.
          </p>
        )}

        <AddGoalForm onAdd={addGoal} />
      </DashboardCard>
    </section>
  );
}
