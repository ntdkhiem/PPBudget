"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, Suspense, useMemo } from "react";
import { apiFetch, Subscription, Transaction } from "@/lib/api";
import { Plus, Trash2, Edit2, Repeat, CheckCircle2, CircleDashed } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { formatCurrency, formatDate } from "@/lib/utils";
import Link from "next/link";
import { useDateRange } from "@/app/contexts/DateRangeContext";
import { format, parseISO, formatDistanceToNow, isPast, isToday } from "date-fns";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { PageContainer } from "@/components/page-container";
import { DashboardCard } from "@/components/dashboard-card";
import { PageHeader } from "@/components/page-header";

function SubscriptionsContent() {
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
  const { date } = useDateRange();

  const queryParams = useMemo(() => {
    const params = new URLSearchParams();
    if (date?.from) params.append("start_date", format(date.from, "yyyy-MM-dd"));
    if (date?.to) params.append("end_date", format(date.to, "yyyy-MM-dd"));
    return params.toString();
  }, [date]);

  const { data: subscriptions, isLoading: loadingSubs } = useQuery<Subscription[]>({
    queryKey: ["subscriptions"],
    queryFn: () => apiFetch<Subscription[]>("/subscriptions", {}, token),
  });

  const { data: transactions, isLoading: loadingTxns } = useQuery<Transaction[]>({
    queryKey: ["transactions", "all", queryParams],
    queryFn: async () => {
      let allTxns: Transaction[] = [];
      let currentCursorDate: string | null = null;
      let currentCursorId: string | null = null;
      let hasMore = true;

      while (hasMore) {
        const params = new URLSearchParams(queryParams);
        if (currentCursorDate && currentCursorId) {
          params.append("cursor_date", currentCursorDate);
          params.append("cursor_id", currentCursorId);
        }
        const res = await apiFetch<Transaction[]>(`/transactions?${params.toString()}`, {}, token);
        if (res.length > 0) {
          allTxns = [...allTxns, ...res];
          const last = res[res.length - 1];
          currentCursorDate = last.date;
          currentCursorId = last.id;
        }
        if (res.length < 50) {
          hasMore = false;
        }
      }
      return allTxns;
    },
  });

  const [isSubOpen, setIsSubOpen] = useState(false);
  const [selectedSub, setSelectedSub] = useState<Subscription | null>(null);
  const [subName, setSubName] = useState("");
  const [subAmount, setSubAmount] = useState("");
  const [subCycle, setSubCycle] = useState<"weekly" | "monthly" | "yearly">("monthly");
  const [subDate, setSubDate] = useState("");

  const handleOpenSub = (sub?: Subscription) => {
    if (sub) {
      setSelectedSub(sub);
      setSubName(sub.name);
      setSubAmount((sub.amount / 100).toString());
      setSubCycle(sub.billing_cycle as "weekly" | "monthly" | "yearly");
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

  if (loadingSubs || loadingTxns) return <div className="p-8 text-center text-slate-500">Loading subscriptions...</div>;

  const subsList = subscriptions || [];
  
  // Calculations
  const sumExpected = subsList.reduce((acc, sub) => acc + sub.amount, 0);
  
  let sumNotPaid = 0;
  subsList.forEach(sub => {
    const isPaid = transactions?.some(t => t.subscription_id === sub.id);
    if (!isPaid) {
      sumNotPaid += sub.amount;
    }
  });

  const expectedMonthlyCosts = subsList.reduce((acc, sub) => {
    return acc + (sub.billing_cycle === 'yearly' ? Math.round(sub.amount / 12) : sub.billing_cycle === 'weekly' ? sub.amount * 4 : sub.amount);
  }, 0);

  return (
    <PageContainer>
      <PageHeader title="Recurring Payments" description="Manage recurring payments and track expected costs.">
        <Button 
          onClick={() => handleOpenSub()}
          className="bg-indigo-600 hover:bg-indigo-700 text-white shadow-md shadow-indigo-500/20 rounded-xl px-5 h-11 flex items-center gap-2 transition-all active:scale-95"
        >
          <Plus size={18} /> Add Subscription
        </Button>
      </PageHeader>

      <div className="bg-white dark:bg-slate-900/80 backdrop-blur-xl rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm overflow-hidden">
        <div className="grid grid-cols-1 divide-y divide-slate-100 dark:divide-slate-800">
          <div className="hidden md:grid grid-cols-12 gap-4 p-4 bg-slate-50 dark:bg-slate-800/50 text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
            <div className="col-span-4">Subscription</div>
            <div className="col-span-3">Next Expected Match</div>
            <div className="col-span-3">Paid This Period</div>
            <div className="col-span-2 text-right">Actions</div>
          </div>
          
          <AnimatePresence>
            {subsList.map((sub) => {
              const paidTxns = transactions?.filter(t => t.subscription_id === sub.id) || [];
              const isPaid = paidTxns.length > 0;
              
              return (
                <motion.div
                  key={sub.id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  className="p-4 grid grid-cols-1 md:grid-cols-12 gap-4 items-center hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors group"
                >
                  <div className="col-span-4 flex items-center gap-3">
                    <div className={`h-10 w-10 shrink-0 rounded-xl flex items-center justify-center font-bold text-sm ${isPaid ? "bg-emerald-100 dark:bg-emerald-500/20 text-emerald-600 dark:text-emerald-400" : "bg-indigo-100 dark:bg-indigo-500/20 text-indigo-600 dark:text-indigo-400"}`}>
                      {sub.name.charAt(0).toUpperCase()}
                    </div>
                    <div>
                      <p className="font-semibold text-slate-900 dark:text-slate-100">{sub.name}</p>
                      <p className="text-sm font-medium text-slate-700 dark:text-slate-300">{formatCurrency(sub.amount)} <span className="text-xs text-slate-500 font-normal">/{sub.billing_cycle === 'yearly' ? 'yr' : sub.billing_cycle === 'weekly' ? 'wk' : 'mo'}</span></p>
                    </div>
                  </div>
                  
                  <div className="col-span-3">
                    <div className="md:hidden text-xs text-slate-500 mb-1 font-medium">Next Expected:</div>
                    <span className="text-sm text-slate-700 dark:text-slate-300">{formatDate(sub.next_billing_date)}</span>
                  </div>
                  
                  <div className="col-span-3">
                    <div className="md:hidden text-xs text-slate-500 mb-1 font-medium">Paid This Period:</div>
                    {isPaid ? (
                      <div className="flex flex-wrap gap-2">
                        {paidTxns.map(txn => (
                          <Link key={txn.id} href={`/transactions?edit=${txn.id}`} className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 text-xs font-medium border border-emerald-200 dark:border-emerald-500/20 hover:bg-emerald-100 dark:hover:bg-emerald-500/20 transition-colors">
                            <CheckCircle2 size={14} />
                            {formatDate(txn.date)}
                          </Link>
                        ))}
                      </div>
                    ) : (
                      (() => {
                        const subDateObj = parseISO(sub.next_billing_date);
                        const isPastDue = isPast(subDateObj) && !isToday(subDateObj);
                        let expectedText = "";
                        if (isToday(subDateObj)) {
                          expectedText = "Expected today";
                        } else if (isPastDue) {
                          expectedText = `Overdue by ${formatDistanceToNow(subDateObj)}`;
                        } else {
                          expectedText = `Expected ${formatDistanceToNow(subDateObj, { addSuffix: true })}`;
                        }
                        
                        return (
                          <span className={`inline-flex items-center gap-1.5 text-sm italic ${isPastDue ? 'text-amber-500 dark:text-amber-400 font-medium' : 'text-slate-400 dark:text-slate-500'}`}>
                            <CircleDashed size={14} /> {expectedText}
                          </span>
                        );
                      })()
                    )}
                  </div>
                  
                  <div className="col-span-2 flex justify-end gap-1">
                    <Button 
                      variant="ghost" 
                      size="icon"
                      onClick={() => handleOpenSub(sub)}
                      className="text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 dark:hover:text-indigo-400 dark:hover:bg-indigo-500/10"
                    >
                      <Edit2 size={16} />
                    </Button>
                    <Button 
                      variant="ghost" 
                      size="icon"
                      onClick={() => {
                        if (confirm("Are you sure you want to delete this subscription?")) {
                          deleteSubMutation.mutate(sub.id!);
                        }
                      }}
                      className="text-slate-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:text-rose-400 dark:hover:bg-rose-500/10"
                    >
                      <Trash2 size={16} />
                    </Button>
                  </div>
                </motion.div>
              );
            })}
          </AnimatePresence>
          {subsList.length === 0 && (
            <div className="p-12 text-center text-slate-500">
              No subscriptions found. Create one to start tracking.
            </div>
          )}
        </div>
      </div>

      {/* Summaries */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm flex flex-col justify-center">
          <p className="text-sm font-medium text-slate-500 dark:text-slate-400 mb-1">Active and expected subscriptions</p>
          <p className="text-2xl font-bold text-slate-900 dark:text-white">{formatCurrency(sumExpected)}</p>
        </div>
        <div className="bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm flex flex-col justify-center">
          <p className="text-sm font-medium text-slate-500 dark:text-slate-400 mb-1">Expected and not yet paid</p>
          <p className="text-2xl font-bold text-amber-600 dark:text-amber-500">{formatCurrency(sumNotPaid)}</p>
        </div>
        <div className="bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm flex flex-col justify-center">
          <p className="text-sm font-medium text-slate-500 dark:text-slate-400 mb-1">Expected monthly costs</p>
          <p className="text-2xl font-bold text-indigo-600 dark:text-indigo-400">{formatCurrency(expectedMonthlyCosts)}</p>
        </div>
      </div>

      <Dialog open={isSubOpen} onOpenChange={setIsSubOpen}>
        <DialogContent className="sm:max-w-[425px] bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <DialogHeader>
            <DialogTitle className="text-slate-900 dark:text-slate-100">{selectedSub ? "Edit Subscription" : "Add Subscription"}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label className="text-slate-700 dark:text-slate-300">Name</Label>
              <Input placeholder="e.g. Netflix" value={subName} onChange={(e) => setSubName(e.target.value)} className="bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-900 dark:text-slate-100" />
            </div>
            <div className="grid gap-2">
              <Label className="text-slate-700 dark:text-slate-300">Amount ($)</Label>
              <Input type="number" step="0.01" placeholder="15.99" value={subAmount} onChange={(e) => setSubAmount(e.target.value)} className="bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-900 dark:text-slate-100" />
            </div>
            <div className="grid gap-2">
              <Label className="text-slate-700 dark:text-slate-300">Billing Cycle</Label>
              <Select value={subCycle} onValueChange={(v: "weekly" | "monthly" | "yearly") => setSubCycle(v)}>
                <SelectTrigger className="bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-900 dark:text-slate-100"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="weekly">Weekly</SelectItem>
                  <SelectItem value="monthly">Monthly</SelectItem>
                  <SelectItem value="yearly">Yearly</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label className="text-slate-700 dark:text-slate-300">Next Billing Date</Label>
              <Input type="date" value={subDate} onChange={(e) => setSubDate(e.target.value)} className="bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-900 dark:text-slate-100" />
            </div>
          </div>
          <DialogFooter className="flex flex-col sm:flex-row gap-2">
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
    </PageContainer>
  );
}

export default function SubscriptionsPage() {
  return (
    <Suspense fallback={<div className="p-8 text-center text-slate-500">Loading...</div>}>
      <SubscriptionsContent />
    </Suspense>
  );
}
