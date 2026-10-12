import type { ReactNode } from "react";
import { formatCurrency } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { BudgetCardSkeleton } from "./budget-card";
import type { Bucket } from "./budget-data";

interface BucketColumnProps {
  bucket: Bucket;
  label: string;
  goalPercent: number;
  /** Sum of the column's limits, in cents. */
  totalLimit: number;
  /** 0 when no paycheck is set, which leaves the share blank. */
  paycheckCents: number;
  isLoading: boolean;
  children: ReactNode;
  isEmpty: boolean;
}

/**
 * One 50/30/20 column: its share of the paycheck against the goal, then its
 * budgets. Needs and Wants are off target above their goal, Savings below it.
 */
export function BucketColumn({
  bucket, label, goalPercent, totalLimit, paycheckCents, isLoading, children, isEmpty,
}: BucketColumnProps) {
  const percent = paycheckCents > 0 ? Math.round((totalLimit / paycheckCents) * 100) : null;
  const offTarget = percent !== null && (bucket === "savings" ? percent < goalPercent : percent > goalPercent);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-between items-center bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800/60">
        <div>
          <h2 className="text-lg font-bold text-slate-800 dark:text-slate-100">{label}</h2>
          <p className="text-xs text-slate-500">Goal: {goalPercent}%</p>
        </div>
        <div className="text-right">
          {isLoading ? (
            <>
              <Skeleton className="h-6 w-24 mb-1" />
              <Skeleton className="h-3 w-8 ml-auto" />
            </>
          ) : (
            <>
              <p className="text-lg font-bold text-slate-700 dark:text-slate-300">{formatCurrency(totalLimit)}</p>
              <p className={`text-xs font-bold ${percent === null ? 'text-slate-400' : offTarget ? 'text-amber-500' : 'text-emerald-500'}`}>
                {percent === null ? "—" : `${percent}%`}
              </p>
            </>
          )}
        </div>
      </div>
      {isLoading ? (
        <>
          <BudgetCardSkeleton />
          <BudgetCardSkeleton />
        </>
      ) : isEmpty ? (
        <p className="text-center text-sm text-slate-400 py-4">No {label} budgeted.</p>
      ) : (
        children
      )}
    </div>
  );
}
