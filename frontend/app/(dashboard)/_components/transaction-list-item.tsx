import { Transaction } from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";
import { formatCurrency, formatDate } from "@/lib/utils";

export function TransactionListItem({ txn, categoryName }: { txn: Transaction; categoryName?: string }) {
  const description = txn.description.replace(/\s+/g, " ").trim();
  return (
    <div className="flex items-center justify-between py-2.5 gap-4">
      <div className="flex items-center gap-3 flex-1 min-w-0">
        <div className="h-9 w-9 shrink-0 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-sm font-bold text-slate-500">
          {description.charAt(0).toUpperCase()}
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-slate-900 dark:text-white truncate">{description}</p>
          <p className="text-xs text-slate-500 truncate">
            {formatDate(txn.date)} &bull; {categoryName ?? "Uncategorized"}
          </p>
        </div>
      </div>
      <span className={`text-sm font-semibold font-heading shrink-0 ${txn.amount < 0 ? "text-rose-600 dark:text-rose-400" : "text-emerald-600 dark:text-emerald-400"}`}>
        {txn.amount > 0 ? "+" : ""}
        {formatCurrency(txn.amount)}
      </span>
    </div>
  );
}

export function ListSkeleton({ rows }: { rows: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: rows }).map((_, i) => (
        <Skeleton key={i} className="h-11 w-full" />
      ))}
    </div>
  );
}
