import Link from "next/link";
import { CircleDashed, Plus } from "lucide-react";
import type { CategorySpend } from "@/lib/api";
import { formatCurrency } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

interface UnbudgetedCardProps {
  spending: CategorySpend[] | undefined;
  isLoading: boolean;
  /** Absent for future months, which can't be edited. */
  onBudget?: (categoryId: string, spentCents: number) => void;
}

/** Spending this month that no budget covers, by category, largest first. */
export function UnbudgetedCard({ spending, isLoading, onBudget }: UnbudgetedCardProps) {
  const total = (spending ?? []).reduce((acc, s) => acc + s.total_spent, 0);

  return (
    <section className="bg-white dark:bg-slate-900 p-6 rounded-2xl shadow-sm border border-slate-200 dark:border-slate-800/60">
      <div className="flex justify-between items-start gap-4 mb-4">
        <div>
          <h2 className="text-lg font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
            <CircleDashed size={18} className="text-amber-500" /> Unbudgeted Spending
          </h2>
          <p className="text-sm text-slate-500">Spending in categories without a budget this month.</p>
        </div>
        <div className="text-right shrink-0">
          {isLoading ? (
            <Skeleton className="h-7 w-24" />
          ) : (
            <p className={`text-2xl font-bold ${total > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-slate-400'}`}>
              {formatCurrency(total)}
            </p>
          )}
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}
        </div>
      ) : !spending?.length ? (
        <p className="text-sm text-slate-500 text-center py-4">Everything spent this month is covered by a budget.</p>
      ) : (
        <ul className="divide-y divide-slate-100 dark:divide-slate-800">
          {spending.map((s) => (
            <li key={s.category_id ?? "uncategorized"} className="flex items-center justify-between gap-4 py-2.5">
              <span className="text-sm font-medium text-slate-700 dark:text-slate-200 truncate">{s.name}</span>
              <div className="flex items-center gap-3 shrink-0">
                <span className="text-sm font-semibold text-slate-900 dark:text-white">{formatCurrency(s.total_spent)}</span>
                {s.category_id === null ? (
                  <Link href="/transactions" className="text-xs font-medium text-indigo-600 hover:text-indigo-700 dark:text-indigo-400 w-24 text-right">
                    Categorize
                  </Link>
                ) : onBudget ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onBudget(s.category_id!, s.total_spent)}
                    className="h-7 w-24 text-xs text-indigo-600 hover:text-indigo-700 dark:text-indigo-400"
                  >
                    <Plus size={14} /> Budget it
                  </Button>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
