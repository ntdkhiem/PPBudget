"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, Account, NetWorthDataPoint, Transaction, BudgetSummary, Category, Subscription, DashboardSummary } from "@/lib/api";
import { useDateRange } from "@/app/contexts/DateRangeContext";
import { formatCurrency, formatDate } from "@/lib/utils";
import Link from "next/link";
import { AreaChart, Area, ResponsiveContainer, YAxis, PieChart as RechartsPieChart, Pie, Cell, Tooltip as RechartsTooltip } from "recharts";
import { motion } from "framer-motion";
import { useState, useMemo } from "react";
import { format, subMonths } from "date-fns";
import { Skeleton } from "@/components/ui/skeleton";
import { Progress } from "@/components/ui/progress";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ArrowRight, ReceiptText, PieChart, Repeat, CreditCard, Plus, ArrowUpRight, Wallet, ShieldCheck, Inbox, CalendarClock, AlertCircle } from "lucide-react";

export default function DashboardPage() {
  const queryClient = useQueryClient();
  const { date } = useDateRange();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const queryParams = useMemo(() => {
    const params = new URLSearchParams();
    if (date?.from) params.append("start_date", format(date.from, "yyyy-MM-dd"));
    if (date?.to) params.append("end_date", format(date.to, "yyyy-MM-dd"));
    return params.toString();
  }, [date]);

  const currentMonthStr = format(new Date(), "yyyy-MM");

  const { data: accounts, isLoading: loadingAccounts } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, token),
  });

  const { data: summary, isLoading: loadingSummary } = useQuery<DashboardSummary>({
    queryKey: ["reports", "summary", queryParams],
    queryFn: () => apiFetch<DashboardSummary>(`/reports/summary?${queryParams}`, {}, token),
  });

  const { data: netWorthData } = useQuery<NetWorthDataPoint[]>({
    queryKey: ["reports", "net-worth", queryParams],
    queryFn: () => apiFetch<NetWorthDataPoint[]>(`/reports/net-worth?${queryParams}`, {}, token),
    select: (data) => data.map(d => ({ ...d, value: d.value / 100 })),
  });

  const { data: transactions, isLoading: loadingTxns } = useQuery<Transaction[]>({
    queryKey: ["transactions", queryParams],
    queryFn: () => apiFetch<Transaction[]>(`/transactions?${queryParams}`, {}, token),
  });

  const { data: budgets, isLoading: loadingBudgets } = useQuery<BudgetSummary[]>({
    queryKey: ["budgets", currentMonthStr],
    queryFn: () => apiFetch<BudgetSummary[]>(`/budgets/summary?month=${currentMonthStr}`, {}, token),
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const { data: subscriptions, isLoading: loadingSubs } = useQuery<Subscription[]>({
    queryKey: ["subscriptions"],
    queryFn: () => apiFetch<Subscription[]>("/subscriptions", {}, token),
  });



  // Top Spending Categories calculation
  const expensesByCategory = useMemo(() => {
    if (!transactions || !categories) return [];
    const expenses = transactions.filter(t => {
      if (t.amount >= 0) return false;
      if (t.category_id) {
        const cat = categories.find(c => c.id === t.category_id);
        if (cat?.type === 'transfer') return false;
      }
      return true;
    });
    const grouped = expenses.reduce((acc, t) => {
      const cat = t.category_id ? categories.find(c => c.id === t.category_id) : null;
      const catName = cat?.name || "Uncategorized";
      // Use effective_amount if present, otherwise fallback to amount.
      const amount = t.effective_amount !== undefined ? t.effective_amount : t.amount;
      acc[catName] = (acc[catName] || 0) + Math.abs(amount);
      return acc;
    }, {} as Record<string, number>);

    return Object.entries(grouped)
      .map(([name, value]) => ({ name, value }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 5); // Take top 5
  }, [transactions, categories]);

  const needsReviewTransactions = useMemo(() => {
    return (transactions || []).filter(t => !t.category_id && t.amount < 0).sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime());
  }, [transactions]);
  const needsReviewTop = needsReviewTransactions.slice(0, 3);
  const needsReviewCount = needsReviewTransactions.length;

  const upcomingSubs = useMemo(() => {
    if (!subscriptions) return [];
    const now = new Date();
    const nextWeek = new Date();
    nextWeek.setDate(now.getDate() + 7);
    return subscriptions.filter(s => {
      const d = new Date(s.next_billing_date);
      return d >= now && d <= nextWeek;
    }).sort((a, b) => new Date(a.next_billing_date).getTime() - new Date(b.next_billing_date).getTime());
  }, [subscriptions]);

  if (loadingAccounts) {
    return (
      <div className="space-y-10">
        <div className="flex justify-between items-center"><Skeleton className="h-12 w-48" /><Skeleton className="h-20 w-64 rounded-2xl" /></div>
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <Skeleton className="h-96 lg:col-span-2 rounded-3xl" />
          <Skeleton className="h-96 rounded-3xl" />
        </div>
      </div>
    );
  }

  const netWorth = accounts?.reduce((acc, a) => {
    return a.type === "asset" ? acc + a.initial_balance : acc - a.initial_balance;
  }, 0) || 0;

  const recentTransactions = transactions?.slice(0, 5) || [];
  const activeBudgets = budgets?.slice(0, 4) || [];

  const COLORS = ['#6366f1', '#8b5cf6', '#ec4899', '#f43f5e', '#f97316', '#eab308'];

  // Budget vs Actual summary
  const totalBudget = budgets?.reduce((acc, b) => acc + b.amount_cents, 0) || 0;
  const totalSpent = budgets?.reduce((acc, b) => acc + b.spent_total, 0) || 0;
  const budgetPercent = totalBudget > 0 ? Math.min((totalSpent / totalBudget) * 100, 100) : 0;
  const isBudgetOver = totalSpent > totalBudget;

  // Savings Rate
  const inPeriod = summary?.in_period || 0;
  const outPeriod = summary?.out_period || 0;
  const savingsRate = inPeriod > 0 ? ((inPeriod - outPeriod) / inPeriod) * 100 : 0;

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.5 }} className="space-y-8">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold font-heading text-slate-900 dark:text-white">Dashboard Overview</h1>
          <p className="text-slate-500 dark:text-slate-400">Quick access to your finances.</p>
        </div>
      </div>

      {/* Summary Boxes */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="bg-white dark:bg-slate-900/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 relative overflow-hidden group">
          <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity"><ArrowUpRight className="w-16 h-16 text-indigo-600" /></div>
          <div className="flex items-center gap-3 mb-4">
            <div className="bg-indigo-50 dark:bg-indigo-500/10 p-2 rounded-xl text-indigo-600 dark:text-indigo-400"><ArrowUpRight className="w-5 h-5" /></div>
            <h3 className="text-sm font-medium text-slate-500 dark:text-slate-400 font-heading">IN + OUT THIS PERIOD</h3>
          </div>
          <div className="flex flex-col gap-1">
            <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
              {loadingSummary ? <Skeleton className="h-8 w-24" /> : formatCurrency((summary?.in_period || 0) - (summary?.out_period || 0))}
            </div>
            <div className="text-sm font-medium flex items-center gap-2">
              {loadingSummary ? <Skeleton className="h-4 w-32" /> : (
                <>
                  <span className="text-emerald-500">+{formatCurrency(summary?.in_period || 0)}</span>
                  <span className="text-slate-300 dark:text-slate-700 dark:text-slate-300">/</span>
                  <span className="text-rose-500">-{formatCurrency(summary?.out_period || 0)}</span>
                </>
              )}
            </div>
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.2 }} className="bg-white dark:bg-slate-900/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 relative overflow-hidden group">
          <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity"><Repeat className="w-16 h-16 text-rose-600" /></div>
          <div className="flex items-center gap-3 mb-4">
            <div className="bg-rose-50 dark:bg-rose-500/10 p-2 rounded-xl text-rose-600 dark:text-rose-400"><Repeat className="w-5 h-5" /></div>
            <h3 className="text-sm font-medium text-slate-500 dark:text-slate-400 font-heading">SUBSCRIPTIONS TO PAY</h3>
          </div>
          <div className="flex flex-col gap-1">
            <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
              {loadingSummary ? <Skeleton className="h-8 w-24" /> : formatCurrency(Math.max(0, (summary?.subscriptions_to_pay || 0) - (summary?.subscriptions_paid || 0)))}
            </div>
            <div className="text-sm font-medium flex items-center gap-2 text-slate-500 dark:text-slate-400">
              {loadingSummary ? <Skeleton className="h-4 w-32" /> : (
                <>
                  Paid: <span className="text-emerald-600 dark:text-emerald-500 font-semibold">{formatCurrency(summary?.subscriptions_paid || 0)}</span>
                  <span className="text-slate-300 dark:text-slate-700 dark:text-slate-300">/</span>
                  Total: <span className="text-slate-600 dark:text-slate-300 font-semibold">{formatCurrency(summary?.subscriptions_to_pay || 0)}</span>
                </>
              )}
            </div>
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.3 }} className="bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 relative overflow-hidden group">
          <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity"><ShieldCheck className="w-16 h-16 text-emerald-600" /></div>
          <div className="flex items-center gap-3 mb-4">
            <div className="bg-emerald-50 dark:bg-emerald-500/10 p-2 rounded-xl text-emerald-600 dark:text-emerald-400"><ShieldCheck className="w-5 h-5" /></div>
            <h3 className="text-sm font-medium text-slate-500 dark:text-slate-400 font-heading">SAFE TO SPEND</h3>
          </div>
          <div className="flex flex-col gap-1">
            <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
              {loadingSummary || loadingBudgets ? <Skeleton className="h-8 w-24" /> : formatCurrency(Math.max(0, totalBudget - totalSpent))}
            </div>
            <div className="text-xs font-medium text-slate-500 dark:text-slate-400">
              Remaining from your allocated budget
            </div>
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.4 }} className="bg-gradient-to-br from-indigo-600 to-violet-600 p-6 rounded-3xl shadow-xl shadow-indigo-500/20 border border-indigo-400/30 relative overflow-hidden group text-white">
          <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity"><CreditCard className="w-16 h-16" /></div>
          <div className="flex items-center gap-3 mb-4">
            <div className="bg-white dark:bg-slate-900/20 backdrop-blur-sm p-2 rounded-xl text-indigo-100"><CreditCard className="w-5 h-5" /></div>
            <h3 className="text-sm font-medium text-indigo-100 font-heading">NET WORTH</h3>
          </div>
          <div className="text-2xl font-bold font-heading">
            {loadingSummary ? <Skeleton className="h-8 w-24 bg-white dark:bg-slate-900/20" /> : formatCurrency(summary?.net_worth || 0)}
          </div>
        </motion.div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        {/* 1. Needs Review Inbox Widget */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.5 }} className="relative overflow-hidden border border-slate-200 dark:border-slate-800/60 bg-white/60 dark:bg-slate-900/60 backdrop-blur-xl rounded-3xl shadow-sm hover:shadow-xl hover:shadow-blue-500/5 hover:-translate-y-1 transition-all duration-500 ease-out flex flex-col p-6">
          <div className="absolute inset-0 bg-gradient-to-br from-blue-50/50 to-indigo-50/50 dark:from-blue-950/20 dark:to-indigo-950/20 pointer-events-none -z-10" />
          
          <div className="flex flex-row items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-xl bg-blue-500/10 dark:bg-blue-500/20 text-blue-600 dark:text-blue-400">
                <Inbox className="w-5 h-5" />
              </div>
              <h2 className="text-lg font-bold font-heading text-slate-900 dark:text-white">
                Needs Review
              </h2>
            </div>
            {needsReviewCount > 0 && (
              <div className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400">
                {needsReviewCount} pending
              </div>
            )}
          </div>

          <div className="flex-1 space-y-2 mb-4">
            {needsReviewTop.map((tx) => (
              <div 
                key={tx.id}
                className="group flex items-center justify-between p-3 -mx-3 rounded-xl hover:bg-white dark:hover:bg-slate-800/50 transition-colors cursor-pointer"
              >
                <div className="flex items-center gap-4">
                  <div className="w-10 h-10 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400 group-hover:scale-105 group-hover:text-blue-500 transition-all">
                    <AlertCircle className="w-4 h-4" />
                  </div>
                  <div>
                    <p className="text-sm font-medium text-slate-900 dark:text-white">
                      {tx.description}
                    </p>
                    <p className="text-xs text-slate-500 dark:text-slate-400">
                      {formatDate(tx.date)}
                    </p>
                  </div>
                </div>
                <span className="text-sm font-semibold text-slate-900 dark:text-white">
                  {formatCurrency(tx.amount)}
                </span>
              </div>
            ))}
            {needsReviewCount === 0 && (
              <p className="text-sm text-slate-500 text-center py-4">All caught up!</p>
            )}
          </div>

          <div className="mt-auto pt-2 border-t border-slate-100 dark:border-slate-800/60">
            <Link href="/transactions" className="w-full flex items-center justify-center py-2 text-sm font-medium group text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300">
              Review all transactions
              <ArrowRight className="w-4 h-4 ml-2 group-hover:translate-x-1 transition-transform" />
            </Link>
          </div>
        </motion.div>

        {/* 2. Upcoming Subscriptions Widget */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.6 }} className="relative overflow-hidden border border-slate-200 dark:border-slate-800/60 bg-white/60 dark:bg-slate-900/60 backdrop-blur-xl rounded-3xl shadow-sm hover:shadow-xl hover:shadow-purple-500/5 hover:-translate-y-1 transition-all duration-500 ease-out flex flex-col p-6">
          <div className="absolute inset-0 bg-gradient-to-br from-purple-50/50 to-pink-50/50 dark:from-purple-950/20 dark:to-pink-950/20 pointer-events-none -z-10" />
          
          <div className="flex flex-row items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-xl bg-purple-500/10 dark:bg-purple-500/20 text-purple-600 dark:text-purple-400">
                <CalendarClock className="w-5 h-5" />
              </div>
              <h2 className="text-lg font-bold font-heading text-slate-900 dark:text-white">
                Upcoming Bills
              </h2>
            </div>
            <p className="text-xs font-medium text-slate-500 dark:text-slate-400">
              Next 7 days
            </p>
          </div>

          <div className="flex-1 space-y-2 mb-4">
            {upcomingSubs.map((sub) => (
              <div 
                key={sub.id}
                className="group flex items-center justify-between p-3 -mx-3 rounded-xl hover:bg-white dark:hover:bg-slate-800/50 transition-colors cursor-pointer"
              >
                <div className="flex items-center gap-4">
                  <div className={`w-10 h-10 rounded-full flex items-center justify-center bg-purple-500/10 text-purple-600 border border-transparent group-hover:border-purple-200 dark:group-hover:border-purple-700/50 transition-colors`}>
                    <Repeat className="w-4 h-4" />
                  </div>
                  <div>
                    <p className="text-sm font-medium text-slate-900 dark:text-white">
                      {sub.name}
                    </p>
                    <div className="flex items-center text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      <CalendarClock className="w-3 h-3 mr-1" />
                      {formatDate(sub.next_billing_date)}
                    </div>
                  </div>
                </div>
                <span className="text-sm font-semibold text-slate-900 dark:text-white">
                  {formatCurrency(sub.amount)}
                </span>
              </div>
            ))}
            {upcomingSubs.length === 0 && (
              <p className="text-sm text-slate-500 text-center py-4">No upcoming bills this week.</p>
            )}
          </div>

          <div className="mt-auto pt-2 border-t border-slate-100 dark:border-slate-800/60">
            <Link href="/subscriptions" className="w-full flex items-center justify-center py-2 text-sm font-medium group text-purple-600 hover:text-purple-700 dark:text-purple-400 dark:hover:text-purple-300">
              Manage subscriptions
              <ArrowRight className="w-4 h-4 ml-2 group-hover:translate-x-1 transition-transform" />
            </Link>
          </div>
        </motion.div>
      </div>

      {/* Main Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        
        {/* Left Column: Transactions & Budgets */}
        <div className="lg:col-span-2 space-y-6">
          
          {/* Recent Transactions Widget */}
          <div className="bg-white dark:bg-slate-900/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200">
                <ReceiptText className="h-5 w-5 text-indigo-600" />
                <h2 className="text-lg font-bold font-heading">Recent Transactions</h2>
              </div>
              <Link href="/transactions" className="text-sm font-medium text-indigo-600 hover:text-indigo-700 flex items-center gap-1 group">
                View All <ArrowRight className="h-4 w-4 group-hover:translate-x-1 transition-transform" />
              </Link>
            </div>
            
            {loadingTxns ? (
              <div className="space-y-4">{Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-12 w-full" />)}</div>
            ) : recentTransactions.length > 0 ? (
              <div className="space-y-3">
                {recentTransactions.map((txn) => (
                  <div key={txn.id} className="flex items-center justify-between p-3 hover:bg-slate-50 dark:hover:bg-slate-800/50 rounded-xl transition-colors group cursor-pointer gap-4">
                    <div className="flex items-center gap-4 flex-1 min-w-0">
                      <div className="h-10 w-10 shrink-0 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center font-bold text-slate-500">
                        {txn.description.trim().charAt(0).toUpperCase()}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="font-medium text-slate-900 dark:text-white truncate">{txn.description.replace(/\s+/g, ' ').trim()}</p>
                        <p className="text-xs text-slate-500 truncate">{formatDate(txn.date)} &bull; {(txn.category_id ? categories?.find(c => c.id === txn.category_id)?.name : null) || "Uncategorized"}</p>
                      </div>
                    </div>
                    <span className={`font-bold font-heading shrink-0 ${txn.amount < 0 ? 'text-rose-600' : 'text-emerald-600'}`}>
                      {txn.amount > 0 ? "+" : ""}{formatCurrency(txn.amount)}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-slate-500 text-center py-6">No recent transactions found.</p>
            )}
          </div>

          {/* Active Budgets Widget */}
          <div className="bg-white dark:bg-slate-900/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200">
                <PieChart className="h-5 w-5 text-indigo-600" />
                <h2 className="text-lg font-bold font-heading">Active Budgets ({currentMonthStr})</h2>
              </div>
              <Link href="/budgets" className="text-sm font-medium text-indigo-600 hover:text-indigo-700 flex items-center gap-1 group">
                Manage <ArrowRight className="h-4 w-4 group-hover:translate-x-1 transition-transform" />
              </Link>
            </div>
            
            {loadingBudgets ? (
              <div className="space-y-4">{Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-14 w-full" />)}</div>
            ) : activeBudgets.length > 0 ? (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {activeBudgets.map((b) => {
                  const categoryName = categories?.find(c => c.id === b.category_id)?.name || b.name || "Unknown";
                  const percent = b.amount_cents > 0 ? Math.min((b.spent_total / b.amount_cents) * 100, 100) : 0;
                  const isOver = b.spent_total > b.amount_cents;
                  return (
                    <div key={b.id} className="p-4 bg-slate-50 dark:bg-slate-800/50 rounded-2xl border border-slate-100 dark:border-slate-800">
                      <div className="flex justify-between items-center mb-2">
                        <span className="font-semibold text-slate-700 dark:text-slate-300">{categoryName}</span>
                        <span className="text-sm text-slate-500 font-medium">{formatCurrency(b.spent_total)} / {formatCurrency(b.amount_cents)}</span>
                      </div>
                      <Progress value={percent} className={`h-2 ${isOver ? 'bg-rose-100 [&>div]:bg-rose-600' : ''}`} />
                    </div>
                  );
                })}
              </div>
            ) : (
              <p className="text-sm text-slate-500 text-center py-6">No active budgets for this month.</p>
            )}
          </div>

        </div>

        {/* Right Column: Mini Charts & Subscriptions */}
        <div className="space-y-6">
          
          {/* Top Spending Categories Widget */}
          <div className="bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-800">
            <h2 className="text-lg font-bold font-heading text-slate-800 dark:text-slate-200 mb-4">Top Spending</h2>
            {expensesByCategory.length > 0 ? (
              <div className="flex flex-col items-center">
                <div className="w-full h-48">
                  <ResponsiveContainer width="100%" height="100%">
                    <RechartsPieChart>
                      <Pie
                        data={expensesByCategory}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={80}
                        paddingAngle={2}
                        dataKey="value"
                        stroke="none"
                      >
                        {expensesByCategory.map((entry, index) => (
                          <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                        ))}
                      </Pie>
                      <RechartsTooltip 
                        formatter={(value: any) => formatCurrency(Number(value) || 0)}
                        contentStyle={{ borderRadius: '12px', border: 'none', boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1)', backgroundColor: 'var(--tw-prose-body, white)' }}
                        itemStyle={{ color: 'inherit' }}
                      />
                    </RechartsPieChart>
                  </ResponsiveContainer>
                </div>
                <div className="flex flex-wrap gap-3 justify-center mt-2">
                  {expensesByCategory.map((entry, idx) => (
                    <div key={entry.name} className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-400 font-medium">
                      <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: COLORS[idx % COLORS.length] }}></div>
                      <span>{entry.name}</span>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <p className="text-sm text-slate-500 text-center py-6">No expenses found.</p>
            )}
          </div>


          {/* Savings Rate Trend */}
          <div className="bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-800">
            <h2 className="text-lg font-bold font-heading text-slate-800 dark:text-slate-200 mb-2">Savings Rate</h2>
            <div className="flex items-end gap-3">
              <span className={`text-4xl font-extrabold tracking-tight ${savingsRate >= 20 ? 'text-emerald-600 dark:text-emerald-400' : savingsRate > 0 ? 'text-indigo-600 dark:text-indigo-400' : 'text-rose-600 dark:text-rose-400'}`}>
                {savingsRate.toFixed(1)}%
              </span>
            </div>
            <p className="text-sm text-slate-500 dark:text-slate-400 mt-2">
              You saved {formatCurrency(Math.max(0, inPeriod - outPeriod))} out of {formatCurrency(inPeriod)} income this period.
            </p>
          </div>

          {/* Subscriptions Stub */}
          <div className="bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-800">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200">
                <Repeat className="h-5 w-5 text-indigo-600" />
                <h2 className="text-lg font-bold font-heading">Subscriptions</h2>
              </div>
            </div>
            
            <p className="text-sm text-slate-500 dark:text-slate-400 mb-6">Manage all your recurring payments, track expected costs, and view payment history.</p>
            <Link href="/subscriptions">
              <Button className="w-full rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white font-medium">
                Manage Subscriptions
              </Button>
            </Link>
          </div>

        </div>
      </div>


    </motion.div>
  );
}
