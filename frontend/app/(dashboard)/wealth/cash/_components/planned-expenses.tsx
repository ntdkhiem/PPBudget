"use client";

import { useState } from "react";
import { CalendarClock, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { cn, formatCurrency, formatDate } from "@/lib/utils";
import { DashboardCard } from "@/components/dashboard-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { toCents, toInputValue } from "../../_components/form-bits";
import {
  useDeletePlannedExpense,
  usePlannedExpenses,
  useSavePlannedExpense,
  type PlannedExpense,
} from "../../_components/wealth-data";

/**
 * One-off costs the user can see coming: a car registration, a wedding, a
 * deposit.
 *
 * For the next twelve months the plan holds each one back -- the emergency
 * fund does not count it and no sweep moves it -- so a down payment due next
 * spring is not also the cushion, or advice to invest it. The engine has done
 * this all along; until there was somewhere to enter them the figure was
 * always zero.
 */

interface Draft {
  id?: string;
  label: string;
  amount: string;
  date: string;
}

const EMPTY: Draft = { label: "", amount: "", date: "" };

/** Today as yyyy-mm-dd in UTC, the calendar the server stores days in. */
const todayUTC = () => new Date().toISOString().slice(0, 10);

function ExpenseForm({
  initial,
  onDone,
}: {
  initial: Draft;
  onDone: () => void;
}) {
  const save = useSavePlannedExpense();
  const [draft, setDraft] = useState(initial);
  const set = (k: keyof Draft, v: string) => setDraft((d) => ({ ...d, [k]: v }));

  const submit = () => {
    const amount = toCents(draft.amount);
    if (!draft.label.trim() || amount === null || amount <= 0 || !draft.date) {
      toast.error("A planned expense needs a label, an amount and a date.");
      return;
    }
    save.mutate(
      {
        id: draft.id,
        label: draft.label.trim(),
        amount,
        target_date: `${draft.date}T00:00:00Z`,
      },
      {
        onSuccess: onDone,
        onError: (error: Error) => toast.error(error.message || "Failed to save the expense"),
      },
    );
  };

  return (
    <div className="space-y-3 rounded-xl border border-dashed border-slate-300 p-4 dark:border-slate-700">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div className="space-y-1.5">
          <Label htmlFor="planned-label">What</Label>
          <Input
            id="planned-label"
            value={draft.label}
            onChange={(e) => set("label", e.target.value)}
            placeholder="Car registration"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="planned-amount">Amount</Label>
          <Input
            id="planned-amount"
            inputMode="decimal"
            value={draft.amount}
            onChange={(e) => set("amount", e.target.value)}
            placeholder="850"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="planned-date">When</Label>
          <Input
            id="planned-date"
            type="date"
            value={draft.date}
            onChange={(e) => set("date", e.target.value)}
          />
        </div>
      </div>
      <div className="flex gap-2">
        <Button onClick={submit} disabled={save.isPending}>
          {save.isPending ? "Saving…" : draft.id ? "Save" : "Add"}
        </Button>
        <Button
          variant="outline"
          onClick={onDone}
          disabled={save.isPending}
          className="text-slate-700 dark:text-slate-200"
        >
          Cancel
        </Button>
      </div>
    </div>
  );
}

function ExpenseRow({ expense, onEdit }: { expense: PlannedExpense; onEdit: () => void }) {
  const remove = useDeletePlannedExpense();
  const day = expense.target_date.slice(0, 10);
  // A day that has passed is no longer held back -- the money has most likely
  // gone already -- so say so rather than let the row look as if it counts.
  const passed = day < todayUTC();

  return (
    <div className="flex items-center gap-3 rounded-xl border border-slate-200 p-3 dark:border-slate-800">
      <CalendarClock
        className={cn("h-4 w-4 shrink-0", passed ? "text-slate-300 dark:text-slate-600" : "text-indigo-500")}
      />
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "truncate text-sm font-semibold",
            passed ? "text-slate-400 dark:text-slate-500" : "text-slate-900 dark:text-white",
          )}
        >
          {expense.label}
        </div>
        <div className="text-xs text-slate-500 dark:text-slate-400">
          {formatDate(expense.target_date)}
          {passed && " · passed, no longer held back"}
        </div>
      </div>
      <span className="text-sm font-bold tabular-nums text-slate-900 dark:text-white">
        {formatCurrency(expense.amount)}
      </span>
      <Button variant="ghost" size="icon" onClick={onEdit} aria-label={`Edit ${expense.label}`}>
        <Pencil className="h-4 w-4 text-slate-400" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        onClick={() => remove.mutate(expense.id)}
        disabled={remove.isPending}
        aria-label={`Delete ${expense.label}`}
      >
        <Trash2 className="h-4 w-4 text-slate-400" />
      </Button>
    </div>
  );
}

export function PlannedExpenses({ earmarkedCents }: { earmarkedCents: number }) {
  const { data: expenses, isLoading } = usePlannedExpenses();
  // null: no form open; a draft: adding (no id) or editing (with one).
  const [editing, setEditing] = useState<Draft | null>(null);

  if (isLoading) {
    return (
      <section className="space-y-4">
        <Skeleton className="h-7 w-40" />
        <Skeleton className="h-32 w-full rounded-3xl" />
      </section>
    );
  }

  const sorted = [...(expenses ?? [])].sort((a, b) => a.target_date.localeCompare(b.target_date));

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
          Coming up
        </h2>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          One-off costs you can see coming. Anything due in the next year is held back: the
          emergency fund does not count it, and no advice moves it.
          {earmarkedCents > 0 && <> {formatCurrency(earmarkedCents)} is held back now.</>}
        </p>
      </div>

      <DashboardCard className="space-y-3">
        {sorted.length === 0 && !editing && (
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Nothing planned. Add a cost you know is coming so the plan stops counting that money
            as spare.
          </p>
        )}

        {sorted.map((e) =>
          editing?.id === e.id ? (
            <ExpenseForm key={e.id} initial={editing} onDone={() => setEditing(null)} />
          ) : (
            <ExpenseRow
              key={e.id}
              expense={e}
              onEdit={() =>
                setEditing({
                  id: e.id,
                  label: e.label,
                  amount: toInputValue("money", e.amount),
                  date: e.target_date.slice(0, 10),
                })
              }
            />
          ),
        )}

        {editing && !editing.id ? (
          <ExpenseForm initial={editing} onDone={() => setEditing(null)} />
        ) : (
          <Button
            variant="outline"
            onClick={() => setEditing(EMPTY)}
            className="w-full text-slate-700 dark:text-slate-200"
          >
            <Plus className="mr-2 h-4 w-4" />
            Add a planned expense
          </Button>
        )}
      </DashboardCard>
    </section>
  );
}
