"use client";

import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, ArrowRight, CalendarClock, CheckCircle2, CreditCard, Inbox, ReceiptText, Repeat } from "lucide-react";
import { apiFetch, getStoredToken, CategorySpend, Subscription, Transaction } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/utils";
import { StatCard } from "@/components/stat-card";
import { useCategoryNames, useRangeQueryParams } from "./dashboard-data";
import { ListSkeleton, TransactionListItem } from "./transaction-list-item";
import { billsDue } from "./upcoming-bills";

const TOP_SPENDING_COUNT = 5;
const RECENT_COUNT = 5;
const NEEDS_REVIEW_PREVIEW = 3;
const UPCOMING_DAYS = 7;
// GET /transactions returns at most this many rows.
const TRANSACTIONS_PAGE_SIZE = 100;

function CardLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link href={href} className="text-sm font-medium text-indigo-600 hover:text-indigo-700 dark:text-indigo-400 dark:hover:text-indigo-300 flex items-center gap-1 group">
      {children} <ArrowRight className="h-4 w-4 group-hover:translate-x-0.5 transition-transform" />
    </Link>
  );
}

function EmptyMessage({ children }: { children: ReactNode }) {
  return <p className="text-sm text-slate-500 text-center py-6">{children}</p>;
}

export function TopSpendingCard() {
  const params = useRangeQueryParams();
  const { data, isLoading } = useQuery<CategorySpend[]>({
    queryKey: ["reports", "spending", params],
    queryFn: () => apiFetch<CategorySpend[]>(`/reports/spending?${params}`, {}, getStoredToken()),
  });

  const top = (data ?? []).slice(0, TOP_SPENDING_COUNT);
  const max = top[0]?.total_spent ?? 0;

  return (
    <StatCard title="Top Spending" icon={CreditCard} tone="rose" action="This period">
      {isLoading ? (
        <ListSkeleton rows={TOP_SPENDING_COUNT} />
      ) : top.length > 0 ? (
        <div className="flex flex-col gap-2">
          {top.map((entry) => (
            <div key={entry.category_id ?? "uncategorized"} className="relative h-10 rounded-lg overflow-hidden flex items-center bg-slate-50 dark:bg-slate-800/40">
              <div
                className="absolute inset-y-0 left-0 rounded-r-lg bg-rose-500/15 dark:bg-rose-400/20"
                style={{ width: `${max > 0 ? (entry.total_spent / max) * 100 : 0}%` }}
              />
              <div className="relative flex justify-between w-full px-3 text-sm gap-3">
                <span className="font-medium text-slate-700 dark:text-slate-200 truncate">{entry.name}</span>
                <span className="font-semibold text-slate-900 dark:text-white shrink-0">{formatCurrency(entry.total_spent)}</span>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <EmptyMessage>No spending this period.</EmptyMessage>
      )}
    </StatCard>
  );
}

export function UpcomingBillsCard() {
  const { data: subscriptions, isLoading } = useQuery<Subscription[]>({
    queryKey: ["subscriptions"],
    queryFn: () => apiFetch<Subscription[]>("/subscriptions", {}, getStoredToken()),
  });

  const bills = useMemo(() => billsDue(subscriptions ?? [], new Date(), UPCOMING_DAYS), [subscriptions]);

  return (
    <StatCard title="Upcoming Bills" icon={CalendarClock} tone="purple" action={<CardLink href="/subscriptions">Manage</CardLink>}>
      <p className="text-xs text-slate-500 dark:text-slate-400 -mt-2 mb-3">Overdue and next {UPCOMING_DAYS} days</p>
      {isLoading ? (
        <ListSkeleton rows={3} />
      ) : bills.length > 0 ? (
        <div className="divide-y divide-slate-100 dark:divide-slate-800">
          {bills.map(({ sub, due, status }) => (
            <div key={`${sub.id}-${due}`} className="flex items-center justify-between py-2.5 gap-4">
              <div className="flex items-center gap-3 min-w-0">
                <div
                  className={`w-9 h-9 shrink-0 rounded-full flex items-center justify-center ${
                    status === "overdue"
                      ? "bg-rose-500/10 text-rose-600 dark:text-rose-400"
                      : "bg-purple-500/10 text-purple-600 dark:text-purple-400"
                  }`}
                >
                  {status === "overdue" ? <AlertCircle className="w-4 h-4" /> : <Repeat className="w-4 h-4" />}
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-slate-900 dark:text-white truncate">{sub.name}</p>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    {formatDate(due)}
                    {status === "overdue" && <span className="font-medium text-rose-600 dark:text-rose-400"> · Overdue</span>}
                    {status === "paid" && <span className="font-medium text-emerald-600 dark:text-emerald-400"> · Paid</span>}
                  </p>
                </div>
              </div>
              <span className="text-sm font-semibold text-slate-900 dark:text-white shrink-0">{formatCurrency(sub.amount)}</span>
            </div>
          ))}
        </div>
      ) : (
        <EmptyMessage>No bills overdue or due in the next {UPCOMING_DAYS} days.</EmptyMessage>
      )}
    </StatCard>
  );
}

export function RecentTransactionsCard({ className }: { className?: string }) {
  const params = useRangeQueryParams();
  const categoryNames = useCategoryNames();
  const { data: transactions, isLoading } = useQuery<Transaction[]>({
    queryKey: ["transactions", "recent", params],
    queryFn: () => apiFetch<Transaction[]>(`/transactions?${params}&limit=${RECENT_COUNT}`, {}, getStoredToken()),
  });

  return (
    <StatCard title="Recent Transactions" icon={ReceiptText} tone="indigo" action={<CardLink href="/transactions">View all</CardLink>} className={className}>
      {isLoading ? (
        <ListSkeleton rows={RECENT_COUNT} />
      ) : transactions?.length ? (
        <div className="divide-y divide-slate-100 dark:divide-slate-800">
          {transactions.map((txn) => (
            <TransactionListItem key={txn.id} txn={txn} categoryName={txn.category_id ? categoryNames.get(txn.category_id) : undefined} />
          ))}
        </div>
      ) : (
        <EmptyMessage>No transactions this period.</EmptyMessage>
      )}
    </StatCard>
  );
}

export function NeedsReviewCard() {
  const categoryNames = useCategoryNames();
  // Same key and URL as the /review page, so reviewing there refreshes this card.
  const { data: unreviewed, isLoading } = useQuery<Transaction[]>({
    queryKey: ["unreviewed"],
    queryFn: () => apiFetch<Transaction[]>("/transactions?unreviewed=true", {}, getStoredToken()),
  });

  const count = unreviewed?.length ?? 0;
  const countLabel = count >= TRANSACTIONS_PAGE_SIZE ? `${TRANSACTIONS_PAGE_SIZE}+` : String(count);

  return (
    <StatCard
      title="Needs Review"
      icon={Inbox}
      tone="amber"
      action={count > 0 ? <CardLink href="/review">Review {countLabel}</CardLink> : undefined}
    >
      {isLoading ? (
        <ListSkeleton rows={NEEDS_REVIEW_PREVIEW} />
      ) : count > 0 ? (
        <div className="divide-y divide-slate-100 dark:divide-slate-800">
          {(unreviewed ?? []).slice(0, NEEDS_REVIEW_PREVIEW).map((txn) => (
            <TransactionListItem key={txn.id} txn={txn} categoryName={txn.category_id ? categoryNames.get(txn.category_id) : undefined} />
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center gap-2 py-6 text-slate-500">
          <CheckCircle2 className="w-8 h-8 text-emerald-500" />
          <p className="text-sm">All caught up.</p>
        </div>
      )}
    </StatCard>
  );
}
