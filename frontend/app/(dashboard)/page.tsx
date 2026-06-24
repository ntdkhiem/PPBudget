"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, Account, NetWorthDataPoint, Transaction, BudgetSummary, Category, Subscription, DashboardSummary } from "@/lib/api";
import { useDateRange } from "@/app/contexts/DateRangeContext";
import { formatCurrency, formatDate } from "@/lib/utils";
import Link from "next/link";
import { AreaChart, Area, ResponsiveContainer, YAxis } from "recharts";
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
import { ArrowRight, ReceiptText, PieChart, Repeat, CreditCard, Plus, ArrowUpRight, Wallet } from "lucide-react";

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

  const [isSubOpen, setIsSubOpen] = useState(false);
  const [selectedSub, setSelectedSub] = useState<Subscription | null>(null);
  const [subName, setSubName] = useState("");
  const [subAmount, setSubAmount] = useState("");
  const [subCycle, setSubCycle] = useState<"monthly" | "yearly">("monthly");
  const [subDate, setSubDate] = useState("");

  const handleOpenSub = (sub?: Subscription) => {
    if (sub) {
      setSelectedSub(sub);
      setSubName(sub.name);
      setSubAmount((sub.amount / 100).toString());
      setSubCycle(sub.billing_cycle as "monthly" | "yearly");
      setSubDate(sub.next_billing_date.split("T")[0]);
    } else {
      setSelectedSub(null);
      setSubName("");
      setSubAmount("");
      setSubCycle("monthly");
      setSubDate("");
    }
    setIsSubOpen(true);
  };

  const addSubMutation = useMutation({
    mutationFn: () => {
      return apiFetch("/subscriptions", {
        method: "POST",
        body: JSON.stringify({
          name: subName,
          amount: Math.round(parseFloat(subAmount) * 100),
          billing_cycle: subCycle,
          next_billing_date: subDate,
        }),
      }, token);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscriptions"] });
      setIsSubOpen(false);
    },
  });

  const updateSubMutation = useMutation({
    mutationFn: () => {
      return apiFetch(`/subscriptions/${selectedSub?.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: subName,
          amount: Math.round(parseFloat(subAmount) * 100),
          billing_cycle: subCycle,
          next_billing_date: subDate,
        }),
      }, token);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscriptions"] });
      setIsSubOpen(false);
    },
  });

  const deleteSubMutation = useMutation({
    mutationFn: (id: string) => apiFetch(`/subscriptions/${id}`, { method: "DELETE" }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscriptions"] });
      setIsSubOpen(false);
    },
  });

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
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200/60 dark:border-slate-800/60 relative overflow-hidden group">
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
                  <span className="text-slate-300 dark:text-slate-700">/</span>
                  <span className="text-rose-500">-{formatCurrency(summary?.out_period || 0)}</span>
                </>
              )}
            </div>
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.2 }} className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200/60 dark:border-slate-800/60 relative overflow-hidden group">
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
                  <span className="text-slate-300 dark:text-slate-700">/</span>
                  Total: <span className="text-slate-600 dark:text-slate-300 font-semibold">{formatCurrency(summary?.subscriptions_to_pay || 0)}</span>
                </>
              )}
            </div>
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.3 }} className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200/60 dark:border-slate-800/60 relative overflow-hidden group">
          <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity"><Wallet className="w-16 h-16 text-emerald-600" /></div>
          <div className="flex items-center gap-3 mb-4">
            <div className="bg-emerald-50 dark:bg-emerald-500/10 p-2 rounded-xl text-emerald-600 dark:text-emerald-400"><Wallet className="w-5 h-5" /></div>
            <h3 className="text-sm font-medium text-slate-500 dark:text-slate-400 font-heading">LEFT TO SPEND</h3>
          </div>
          <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
            {loadingSummary ? <Skeleton className="h-8 w-24" /> : formatCurrency(summary?.left_to_spend || 0)}
          </div>
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.4 }} className="bg-gradient-to-br from-indigo-600 to-violet-600 p-6 rounded-3xl shadow-xl shadow-indigo-500/20 border border-indigo-400/30 relative overflow-hidden group text-white">
          <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity"><CreditCard className="w-16 h-16" /></div>
          <div className="flex items-center gap-3 mb-4">
            <div className="bg-white/20 backdrop-blur-sm p-2 rounded-xl text-indigo-100"><CreditCard className="w-5 h-5" /></div>
            <h3 className="text-sm font-medium text-indigo-100 font-heading">NET WORTH</h3>
          </div>
          <div className="text-2xl font-bold font-heading">
            {loadingSummary ? <Skeleton className="h-8 w-24 bg-white/20" /> : formatCurrency(summary?.net_worth || 0)}
          </div>
        </motion.div>
      </div>

      {/* Main Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        
        {/* Left Column: Transactions & Budgets */}
        <div className="lg:col-span-2 space-y-6">
          
          {/* Recent Transactions Widget */}
          <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200/60 dark:border-slate-800/60">
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
                  <div key={txn.id} className="flex items-center justify-between p-3 hover:bg-slate-50 dark:hover:bg-slate-800/50 rounded-xl transition-colors group cursor-pointer">
                    <div className="flex items-center gap-4">
                      <div className="h-10 w-10 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center font-bold text-slate-500">
                        {txn.description.charAt(0).toUpperCase()}
                      </div>
                      <div>
                        <p className="font-medium text-slate-900 dark:text-white">{txn.description}</p>
                        <p className="text-xs text-slate-500">{formatDate(txn.date)} &bull; {txn.category?.name || "Uncategorized"}</p>
                      </div>
                    </div>
                    <span className={`font-bold font-heading ${txn.amount < 0 ? 'text-rose-600' : 'text-emerald-600'}`}>
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
          <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200/60 dark:border-slate-800/60">
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
          
          {/* Net Worth Mini Chart */}
          <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200/60 dark:border-slate-800/60">
            <h3 className="text-md font-semibold mb-4 text-slate-800 dark:text-slate-200 font-heading">6-Month Trend</h3>
            <div className="h-32 -mx-2">
              {netWorthData ? (
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={netWorthData}>
                    <defs>
                      <linearGradient id="colorMini" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#4f46e5" stopOpacity={0.3}/>
                        <stop offset="95%" stopColor="#4f46e5" stopOpacity={0}/>
                      </linearGradient>
                    </defs>
                    <YAxis domain={['auto', 'auto']} hide />
                    <Area type="monotone" dataKey="value" stroke="#4f46e5" strokeWidth={2} fillOpacity={1} fill="url(#colorMini)" />
                  </AreaChart>
                </ResponsiveContainer>
              ) : (
                <Skeleton className="h-full w-full" />
              )}
            </div>
          </div>

          {/* Subscriptions Stub */}
          <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200/60 dark:border-slate-800/60">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200">
                <Repeat className="h-5 w-5 text-indigo-600" />
                <h2 className="text-lg font-bold font-heading">Subscriptions</h2>
              </div>
              <Button size="icon-sm" variant="ghost" onClick={() => handleOpenSub()} className="text-indigo-600 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 h-8 w-8">
                <Plus size={16} />
              </Button>
            </div>
            
            
            {loadingSubs ? (
              <div className="space-y-3">{Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-14 w-full rounded-xl" />)}</div>
            ) : subscriptions && subscriptions.length > 0 ? (
              <div className="space-y-3">
                {subscriptions.map((sub) => (
                  <div key={sub.id} onClick={() => handleOpenSub(sub)} className="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-800/50 rounded-xl hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer group">
                    <div className="flex items-center gap-3">
                      <div className="h-8 w-8 rounded-lg bg-indigo-100 dark:bg-indigo-900/50 text-indigo-600 dark:text-indigo-400 flex items-center justify-center font-bold text-xs group-hover:scale-110 transition-transform">
                        {sub.name.charAt(0).toUpperCase()}
                      </div>
                      <div>
                        <p className="text-sm font-medium text-slate-900 dark:text-white">{sub.name}</p>
                        <p className="text-xs text-slate-500">Renews on {formatDate(sub.next_billing_date)}</p>
                      </div>
                    </div>
                    <span className="text-sm font-semibold text-slate-700 dark:text-slate-300">{formatCurrency(sub.amount)}</span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-6">
                <p className="text-sm text-slate-500 mb-2">No subscriptions found.</p>
                <Button onClick={() => handleOpenSub()} variant="outline" className="w-full rounded-xl text-indigo-600 border-indigo-200 hover:bg-indigo-50 dark:border-indigo-900 dark:hover:bg-indigo-900/30">
                  Add Subscription
                </Button>
              </div>
            )}
            {subscriptions && subscriptions.length > 0 && (
              <Button onClick={() => handleOpenSub()} variant="outline" className="w-full mt-4 rounded-xl text-indigo-600 border-indigo-200 hover:bg-indigo-50 dark:border-indigo-900 dark:hover:bg-indigo-900/30">
                Manage Subscriptions
              </Button>
            )}
          </div>

        </div>
      </div>

      <Dialog open={isSubOpen} onOpenChange={setIsSubOpen}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>{selectedSub ? "Edit Subscription" : "Add Subscription"}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label>Name</Label>
              <Input placeholder="e.g. Netflix" value={subName} onChange={(e) => setSubName(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label>Amount ($)</Label>
              <Input type="number" step="0.01" placeholder="15.99" value={subAmount} onChange={(e) => setSubAmount(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label>Billing Cycle</Label>
              <Select value={subCycle} onValueChange={(v: "monthly" | "yearly") => setSubCycle(v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="monthly">Monthly</SelectItem>
                  <SelectItem value="yearly">Yearly</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Next Billing Date</Label>
              <Input type="date" value={subDate} onChange={(e) => setSubDate(e.target.value)} />
            </div>
          </div>
          <DialogFooter className="flex flex-col sm:flex-row gap-2">
            {selectedSub && (
              <Button 
                variant="destructive" 
                onClick={() => deleteSubMutation.mutate(selectedSub.id)}
                disabled={deleteSubMutation.isPending}
                className="w-full sm:w-auto"
              >
                {deleteSubMutation.isPending ? "Deleting..." : "Delete"}
              </Button>
            )}
            <div className="flex-1"></div>
            <Button variant="outline" onClick={() => setIsSubOpen(false)}>Cancel</Button>
            <Button 
              onClick={() => selectedSub ? updateSubMutation.mutate() : addSubMutation.mutate()} 
              disabled={!subName || !subAmount || !subDate || addSubMutation.isPending || updateSubMutation.isPending}
              className="bg-indigo-600 hover:bg-indigo-700 text-white"
            >
              {addSubMutation.isPending || updateSubMutation.isPending ? "Saving..." : (selectedSub ? "Save Changes" : "Save Subscription")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </motion.div>
  );
}
