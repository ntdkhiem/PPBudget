"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useMemo, useEffect } from "react";
import { apiFetch, BudgetSummary, Category } from "@/lib/api";
import { formatCurrency } from "@/lib/utils";
import { ChevronLeft, ChevronRight, Edit2, Check, X, Plus, Target, TrendingUp, Calendar, AlertCircle, Trash2 } from "lucide-react";
import { format, addMonths, subMonths, startOfMonth, getDaysInMonth, getDate, isSameMonth } from "date-fns";
import { motion, AnimatePresence } from "framer-motion";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogTrigger, DialogDescription } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Progress } from "@/components/ui/progress";
import { EmptyState } from "@/components/ui/empty-state";
import { PageContainer } from "@/components/page-container";
import { DashboardCard } from "@/components/dashboard-card";
import { PageHeader } from "@/components/page-header";

export default function BudgetsPage() {
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
  
  const [currentMonth, setCurrentMonth] = useState(startOfMonth(new Date()));
  const monthStr = format(currentMonth, "yyyy-MM");

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editLimit, setEditLimit] = useState("");
  const [editBucket, setEditBucket] = useState<"needs" | "wants" | "savings">("needs");
  
  // Paycheck state
  const [expectedPaycheckStr, setExpectedPaycheckStr] = useState("");
  const [isEditingPaycheck, setIsEditingPaycheck] = useState(false);

  const { data: paycheckData, isLoading: loadingPaycheck } = useQuery<{value: string}>({
    queryKey: ["setting", "expected_paycheck"],
    queryFn: () => apiFetch<{value: string}>("/settings/values/expected_paycheck", {}, token),
  });

  useEffect(() => {
    if (paycheckData?.value) {
      setExpectedPaycheckStr(paycheckData.value);
    }
  }, [paycheckData]);

  const expectedPaycheck = parseFloat(expectedPaycheckStr) || 0;
  const expectedPaycheckCents = expectedPaycheck * 100;

  const savePaycheckMutation = useMutation({
    mutationFn: (value: string) => apiFetch("/settings/values/expected_paycheck", {
      method: "PUT",
      body: JSON.stringify({ value })
    }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["setting", "expected_paycheck"] });
      setIsEditingPaycheck(false);
      toast.success("Expected paycheck saved");
    }
  });

  const handleSavePaycheck = () => {
    savePaycheckMutation.mutate(expectedPaycheckStr);
  };

  // New Budget Modal State
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [newCategoryId, setNewCategoryId] = useState("");
  const [newLimit, setNewLimit] = useState("");
  const [newBucket, setNewBucket] = useState<"needs" | "wants" | "savings">("needs");

  // Delete Budget Modal State
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [budgetToDelete, setBudgetToDelete] = useState<string | null>(null);

  const { data: budgets, isLoading: loadingBudgets } = useQuery<BudgetSummary[]>({
    queryKey: ["budgets", monthStr],
    queryFn: () => apiFetch<BudgetSummary[]>(`/budgets/summary?month=${monthStr}`, {}, token),
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const updateMutation = useMutation({
    mutationFn: (data: { id?: string, category_id: string, limit_amount: number, period_type?: string, bucket?: string }) => {
      const year = currentMonth.getFullYear();
      const month = currentMonth.getMonth();
      const startDate = format(new Date(year, month, 1), "yyyy-MM-dd") + "T00:00:00Z";
      const endDate = format(new Date(year, month + 1, 0), "yyyy-MM-dd") + "T00:00:00Z";
      const catName = categories?.find(c => c.id === data.category_id)?.name || "Budget";

      return apiFetch(data.id ? `/budgets/${data.id}` : "/budgets", {
        method: data.id ? "PUT" : "POST",
        body: JSON.stringify({ 
          name: `${catName} Budget`,
          category_id: data.category_id, 
          amount_cents: data.limit_amount, 
          period_type: data.period_type || "monthly",
          start_date: startDate,
          end_date: endDate,
          bucket: data.bucket || "needs"
        }),
      }, token);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["budgets"] });
      setEditingId(null);
      setIsAddModalOpen(false);
      setNewCategoryId("");
      setNewLimit("");
      setNewBucket("needs");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: ({ id, all }: { id: string; all: boolean }) => 
      apiFetch(`/budgets/${id}${all ? '?all=true' : ''}`, { method: "DELETE" }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["budgets"] });
      setDeleteModalOpen(false);
      setBudgetToDelete(null);
      toast.success("Budget deleted successfully");
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to delete budget");
    }
  });

  const nextMonth = () => setCurrentMonth(addMonths(currentMonth, 1));
  const prevMonth = () => setCurrentMonth(subMonths(currentMonth, 1));

  const daysInMonth = getDaysInMonth(currentMonth);
  const today = new Date();
  const currentDay = isSameMonth(today, currentMonth) ? getDate(today) : (today > currentMonth ? daysInMonth : 1);
  const daysRemaining = daysInMonth - currentDay + 1;

  const budgetCards = useMemo(() => {
    return categories?.map(cat => {
      const budget = budgets?.find(b => b.category_id === cat.id);
      return {
        catId: cat.id,
        catName: cat.name,
        budgetId: budget?.id,
        limit: budget?.amount_cents || 0,
        spent: budget?.spent_total || 0,
        bucket: budget?.bucket || "needs",
      };
    }).filter(c => c.limit > 0 || c.spent > 0) || [];
  }, [categories, budgets]);

  const totalLimit = budgetCards.reduce((acc, curr) => acc + curr.limit, 0);
  const totalSpent = budgetCards.reduce((acc, curr) => acc + curr.spent, 0);
  
  const monthProgress = (currentDay / daysInMonth) * 100;
  const globalProgress = totalLimit > 0 ? Math.min((totalSpent / totalLimit) * 100, 100) : 0;
  const globalRemaining = Math.max(0, totalLimit - totalSpent);
  const isOverPacing = globalProgress > monthProgress;
  
  const handleAddBudgetOpenChange = (open: boolean) => {
    if (!open) {
      if (newCategoryId || newLimit) {
        if (window.confirm("Are you sure you want to cancel? Any unsaved changes will be lost.")) {
          setIsAddModalOpen(false);
          setNewCategoryId("");
          setNewLimit("");
          setNewBucket("needs");
        }
      } else {
        setIsAddModalOpen(false);
      }
    } else {
      setIsAddModalOpen(true);
    }
  };

  const renderCard = (card: any, idx: number) => {
    const isEditing = editingId === card.catId;
    const progress = card.limit > 0 ? Math.min((card.spent / card.limit) * 100, 100) : 0;
    const isOverLimit = card.spent > card.limit;
    const remaining = Math.max(0, card.limit - card.spent);
    
    const safeToSpendPerDay = remaining / Math.max(1, daysRemaining);
    const avgSpentPerDay = card.spent / Math.max(1, currentDay);

    return (
      <motion.div 
        key={card.catId} 
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: idx * 0.05 }}
        className={`bg-white dark:bg-slate-900 p-5 rounded-2xl shadow-sm flex flex-col border transition-all ${
          isOverLimit && card.limit > 0 
            ? 'bg-red-50/30 border-red-200 shadow-[0_0_15px_rgba(239,68,68,0.05)]' 
            : 'border-slate-200 dark:border-slate-800/60 hover:shadow-md hover:border-slate-300'
        }`}
      >
        <div className="flex justify-between items-center mb-4">
          <div className="flex items-center gap-2">
            <h3 className="text-md font-bold text-slate-800 dark:text-slate-100 truncate pr-2">{card.catName}</h3>
            {isEditing && (
              <Select value={editBucket} onValueChange={(val: any) => setEditBucket(val)}>
                <SelectTrigger className="h-7 w-24 text-xs px-2">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="needs">Needs</SelectItem>
                  <SelectItem value="wants">Wants</SelectItem>
                  <SelectItem value="savings">Savings</SelectItem>
                </SelectContent>
              </Select>
            )}
          </div>
          <AnimatePresence mode="wait">
            {isEditing ? (
              <motion.div 
                key="edit"
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.9 }}
                className="flex gap-1"
              >
                <Button 
                  size="icon-sm"
                  variant="ghost"
                  onClick={() => updateMutation.mutate({ id: card.budgetId, category_id: card.catId, limit_amount: Math.round(parseFloat(editLimit) * 100), bucket: editBucket })}
                  className="text-emerald-600 hover:bg-emerald-50 h-8 w-8"
                >
                  <Check size={16} />
                </Button>
                <Button 
                  size="icon-sm"
                  variant="ghost"
                  onClick={() => setEditingId(null)} 
                  className="text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 h-8 w-8"
                >
                  <X size={16} />
                </Button>
              </motion.div>
            ) : (
              <motion.div
                key="view"
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.9 }}
                className="flex gap-1"
              >
                <Button
                  size="icon-sm"
                  variant="ghost"
                  onClick={() => { setEditingId(card.catId); setEditLimit((card.limit / 100).toString()); setEditBucket(card.bucket as "needs" | "wants" | "savings"); }}
                  className="text-slate-400 hover:text-indigo-600 h-8 w-8"
                >
                  <Edit2 size={14} />
                </Button>
                {card.budgetId && (
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => {
                      setBudgetToDelete(card.budgetId as string);
                      setDeleteModalOpen(true);
                    }}
                    className="text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 h-8 w-8"
                  >
                    <Trash2 size={14} />
                  </Button>
                )}
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        <div className="flex justify-between items-end mb-3">
          <div>
            <p className="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-0.5">Spent</p>
            <p className={`text-xl font-bold tracking-tight ${isOverLimit && card.limit > 0 ? "text-red-600" : "text-slate-900 dark:text-slate-100"}`}>
              {formatCurrency(card.spent)}
            </p>
          </div>
          <div className="text-right">
            <p className="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-0.5">Limit</p>
            <AnimatePresence mode="wait">
              {isEditing ? (
                <motion.div 
                  key="edit-input"
                  initial={{ opacity: 0, x: 10 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: -10 }}
                  className="flex items-center justify-end h-7"
                >
                  <span className="text-slate-500 mr-1 font-semibold">$</span>
                  <Input 
                    type="number" 
                    value={editLimit} 
                    onChange={e => setEditLimit(e.target.value)}
                    className="w-20 h-7 px-2 text-right font-semibold text-slate-900 dark:text-slate-100"
                    autoFocus
                  />
                </motion.div>
              ) : (
                <motion.p 
                  key="view-limit"
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: 10 }}
                  className="text-md font-semibold text-slate-500 h-7 flex items-center justify-end"
                >
                  {formatCurrency(card.limit)}
                </motion.p>
              )}
            </AnimatePresence>
          </div>
        </div>

        <div className="w-full bg-slate-100 dark:bg-slate-800 rounded-full h-1.5 mb-3 overflow-hidden">
          <motion.div 
            className={`h-full rounded-full ${
              isOverLimit ? 'bg-red-500' : progress > 80 ? 'bg-amber-400' : 'bg-indigo-500'
            }`}
            initial={{ width: 0 }}
            animate={{ width: `${progress}%` }}
            transition={{ duration: 1, ease: "easeOut" }}
          ></motion.div>
        </div>
        
        {card.limit > 0 && (
          <div className="flex justify-between mt-auto pt-3 border-t border-slate-100 dark:border-slate-800/50">
             <div className="text-left">
              <p className="text-[9px] uppercase font-bold text-slate-400 mb-0.5">Left</p>
              <p className={`text-xs font-semibold ${remaining === 0 ? 'text-red-500' : 'text-slate-700 dark:text-slate-300'}`}>
                {formatCurrency(remaining)}
              </p>
            </div>
            <div className="text-right">
              <p className="text-[9px] uppercase font-bold text-slate-400 mb-0.5">Safe / Day</p>
              <p className={`text-xs font-semibold ${safeToSpendPerDay === 0 ? 'text-red-500' : 'text-emerald-600'}`}>
                {formatCurrency(safeToSpendPerDay)}
              </p>
            </div>
          </div>
        )}
      </motion.div>
    );
  };

  const needsBudgets = budgetCards.filter(c => c.bucket === "needs");
  const wantsBudgets = budgetCards.filter(c => c.bucket === "wants");
  const savingsBudgets = budgetCards.filter(c => c.bucket === "savings");

  const needsTotal = needsBudgets.reduce((acc, curr) => acc + curr.limit, 0);
  const wantsTotal = wantsBudgets.reduce((acc, curr) => acc + curr.limit, 0);
  const savingsTotal = savingsBudgets.reduce((acc, curr) => acc + curr.limit, 0);

  const getPercent = (amount: number) => {
    if (expectedPaycheckCents === 0) return 0;
    return Math.round((amount / expectedPaycheckCents) * 100);
  };

  if (loadingBudgets) return <div className="p-8 text-center text-slate-500">Loading budgets...</div>;

  return (
    <PageContainer>
      <PageHeader title="Budgets" description="Calibrate your budget to your paycheck with a 50/30/20 strategy.">
        <div className="flex items-center gap-2 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-1 shadow-sm mr-2">
          <Button variant="ghost" size="icon" onClick={prevMonth} className="h-8 w-8 text-slate-500 hover:text-slate-900 dark:text-slate-100"><ChevronLeft size={18}/></Button>
          <span className="font-semibold w-24 text-center text-slate-700 dark:text-slate-300">{format(currentMonth, 'MMM yyyy')}</span>
          <Button variant="ghost" size="icon" onClick={nextMonth} className="h-8 w-8 text-slate-500 hover:text-slate-900 dark:text-slate-100"><ChevronRight size={18}/></Button>
        </div>
        <Dialog open={isAddModalOpen} onOpenChange={handleAddBudgetOpenChange}>
          <DialogTrigger asChild>
            <Button className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl px-5 h-11 shadow-md shadow-indigo-500/20 flex items-center gap-2 transition-all active:scale-95">
              <Plus size={18} /> New Budget
            </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-[425px]">
              <DialogHeader>
                <DialogTitle>Create New Budget</DialogTitle>
                <DialogDescription>Set a spending limit for a specific category this month.</DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid gap-2">
                  <Label>Category</Label>
                  <Select value={newCategoryId} onValueChange={setNewCategoryId}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select category..." />
                    </SelectTrigger>
                    <SelectContent>
                      {categories?.filter(c => !budgets?.find(b => b.category_id === c.id))?.map(c => (
                        <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-2">
                  <Label>Bucket</Label>
                  <Select value={newBucket} onValueChange={(val: any) => setNewBucket(val)}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select bucket..." />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="needs">Needs (50%)</SelectItem>
                      <SelectItem value="wants">Wants (30%)</SelectItem>
                      <SelectItem value="savings">Savings (20%)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-2">
                  <Label>Monthly Limit ($)</Label>
                  <Input 
                    type="number" 
                    placeholder="e.g. 500" 
                    value={newLimit}
                    onChange={e => setNewLimit(e.target.value)}
                  />
                </div>
              </div>
              <div className="flex flex-col gap-3 mt-6">
                <Button 
                  onClick={() => updateMutation.mutate({ category_id: newCategoryId, limit_amount: Math.round(parseFloat(newLimit) * 100), period_type: "monthly", bucket: newBucket })}
                  disabled={!newCategoryId || !newLimit || updateMutation.isPending}
                  className="bg-indigo-600 hover:bg-indigo-700 text-white w-full"
                >
                  {updateMutation.isPending ? "Saving..." : "Add for This & Future Months"}
                </Button>
                <Button 
                  variant="secondary"
                  onClick={() => updateMutation.mutate({ category_id: newCategoryId, limit_amount: Math.round(parseFloat(newLimit) * 100), period_type: "one_time", bucket: newBucket })}
                  disabled={!newCategoryId || !newLimit || updateMutation.isPending}
                  className="w-full bg-indigo-50 text-indigo-700 hover:bg-indigo-100 dark:bg-indigo-900/30 dark:text-indigo-300 dark:hover:bg-indigo-900/50"
                >
                  {updateMutation.isPending ? "Saving..." : "Add Only for This Month"}
                </Button>
                <Button variant="outline" onClick={() => setIsAddModalOpen(false)} className="w-full">
                  Cancel
                </Button>
              </div>
            </DialogContent>
          </Dialog>

          <Dialog open={deleteModalOpen} onOpenChange={setDeleteModalOpen}>
            <DialogContent className="sm:max-w-[425px]">
              <DialogHeader>
                <DialogTitle>Delete Budget</DialogTitle>
                <DialogDescription>
                  Are you sure you want to delete this budget?
                </DialogDescription>
              </DialogHeader>
              <div className="flex flex-col gap-3 mt-6">
                <Button 
                  variant="destructive"
                  className="w-full bg-red-700 hover:bg-red-800 text-white"
                  disabled={deleteMutation.isPending}
                  onClick={() => {
                    if (budgetToDelete) deleteMutation.mutate({ id: budgetToDelete, all: true });
                  }}
                >
                  Delete for This & Future Months
                </Button>
                <Button 
                  variant="destructive"
                  className="w-full"
                  disabled={deleteMutation.isPending}
                  onClick={() => {
                    if (budgetToDelete) deleteMutation.mutate({ id: budgetToDelete, all: false });
                  }}
                >
                  Delete for This Month Only
                </Button>
                <Button variant="outline" onClick={() => setDeleteModalOpen(false)} className="w-full">
                  Cancel
                </Button>
              </div>
            </DialogContent>
          </Dialog>
      </PageHeader>

      {/* Global Month Overview & Paycheck Calibration */}
      <motion.div 
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="bg-white dark:bg-slate-900 p-6 md:p-8 rounded-2xl shadow-sm border border-slate-200 dark:border-slate-800/60 relative overflow-hidden"
      >
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-2">
          <div>
            <h2 className="text-lg font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2 mb-1">
              <Target size={20} className="text-indigo-500"/> Paycheck Calibration
            </h2>
            <p className="text-sm text-slate-500">Ensure your total budget matches your monthly income.</p>
          </div>
          
          <div className="flex flex-wrap gap-8 items-end">
            <div>
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1">Total Budgeted</p>
              <p className="text-2xl font-bold text-slate-900 dark:text-slate-100">{formatCurrency(totalLimit)}</p>
            </div>
            
            <div className="bg-slate-50 dark:bg-slate-800/50 p-3 rounded-xl border border-slate-100 dark:border-slate-800">
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1">Expected Paycheck</p>
              {isEditingPaycheck ? (
                <div className="flex items-center gap-2">
                  <span className="text-slate-500 font-bold">$</span>
                  <Input 
                    type="number" 
                    value={expectedPaycheckStr} 
                    onChange={e => setExpectedPaycheckStr(e.target.value)}
                    className="w-24 h-8 px-2 font-bold text-slate-900 dark:text-slate-100"
                    autoFocus
                  />
                  <Button size="sm" onClick={handleSavePaycheck} className="h-8">Save</Button>
                </div>
              ) : (
                <div className="flex items-center gap-2 group cursor-pointer" onClick={() => setIsEditingPaycheck(true)}>
                  <p className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
                    {expectedPaycheckCents > 0 ? formatCurrency(expectedPaycheckCents) : "$0.00"}
                  </p>
                  <Edit2 size={14} className="text-slate-400 group-hover:text-slate-600 opacity-0 group-hover:opacity-100 transition-opacity" />
                </div>
              )}
            </div>

            <div className="text-right">
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1">Left to Budget</p>
              <p className={`text-2xl font-bold ${expectedPaycheckCents - totalLimit < 0 ? 'text-red-500' : 'text-slate-400'}`}>
                {formatCurrency(Math.max(0, expectedPaycheckCents - totalLimit))}
              </p>
            </div>
          </div>
        </div>

        <div className="relative pt-6 pb-2 mt-8 border-t border-transparent">
          {/* Pacing Marker */}
          <div 
            className="absolute top-0 w-px h-full bg-slate-300 z-10 hidden md:block" 
            style={{ left: `${monthProgress}%` }}
          >
            <div className="absolute -top-6 -translate-x-1/2 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-sm text-xs font-semibold text-slate-600 dark:text-slate-400 px-2 py-1 rounded-md flex items-center gap-1 whitespace-nowrap">
              <Calendar size={12}/> Today (Day {currentDay})
            </div>
          </div>

          <div className="h-4 bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden relative">
            <motion.div 
              className={`h-full rounded-full relative z-20 ${
                isOverPacing ? 'bg-amber-500' : 'bg-indigo-500'
              }`}
              initial={{ width: 0 }}
              animate={{ width: `${globalProgress}%` }}
              transition={{ duration: 1, ease: "easeOut" }}
            />
          </div>
        </div>

        <div className="flex justify-between items-center mt-4 pt-4 border-t border-slate-100 dark:border-slate-800/50">
          <div className="flex items-center gap-2">
            {isOverPacing ? (
              <AlertCircle size={16} className="text-amber-500" />
            ) : (
              <TrendingUp size={16} className="text-emerald-500" />
            )}
            <span className={`text-sm font-medium ${isOverPacing ? 'text-amber-600' : 'text-emerald-600'}`}>
              {isOverPacing ? "Pacing ahead of schedule" : "On track for the month"}
            </span>
          </div>
          <div className="text-sm font-medium text-slate-600 dark:text-slate-400">
            {formatCurrency(globalRemaining)} remaining
          </div>
        </div>
      </motion.div>

      {/* 3-Column Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mt-8">
        
        {/* Needs */}
        <div className="flex flex-col gap-4">
          <div className="flex justify-between items-center bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800/60">
            <div>
              <h2 className="text-lg font-bold text-slate-800 dark:text-slate-100">Needs</h2>
              <p className="text-xs text-slate-500">Goal: 50%</p>
            </div>
            <div className="text-right">
              <p className="text-lg font-bold text-slate-700 dark:text-slate-300">{formatCurrency(needsTotal)}</p>
              <p className={`text-xs font-bold ${getPercent(needsTotal) > 50 ? 'text-amber-500' : 'text-emerald-500'}`}>
                {getPercent(needsTotal)}%
              </p>
            </div>
          </div>
          {needsBudgets.map((card, idx) => renderCard(card, idx))}
          {needsBudgets.length === 0 && <p className="text-center text-sm text-slate-400 py-4">No Needs budgeted.</p>}
        </div>

        {/* Wants */}
        <div className="flex flex-col gap-4">
          <div className="flex justify-between items-center bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800/60">
            <div>
              <h2 className="text-lg font-bold text-slate-800 dark:text-slate-100">Wants</h2>
              <p className="text-xs text-slate-500">Goal: 30%</p>
            </div>
            <div className="text-right">
              <p className="text-lg font-bold text-slate-700 dark:text-slate-300">{formatCurrency(wantsTotal)}</p>
              <p className={`text-xs font-bold ${getPercent(wantsTotal) > 30 ? 'text-amber-500' : 'text-emerald-500'}`}>
                {getPercent(wantsTotal)}%
              </p>
            </div>
          </div>
          {wantsBudgets.map((card, idx) => renderCard(card, idx))}
          {wantsBudgets.length === 0 && <p className="text-center text-sm text-slate-400 py-4">No Wants budgeted.</p>}
        </div>

        {/* Savings */}
        <div className="flex flex-col gap-4">
          <div className="flex justify-between items-center bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800/60">
            <div>
              <h2 className="text-lg font-bold text-slate-800 dark:text-slate-100">Savings</h2>
              <p className="text-xs text-slate-500">Goal: 20%</p>
            </div>
            <div className="text-right">
              <p className="text-lg font-bold text-slate-700 dark:text-slate-300">{formatCurrency(savingsTotal)}</p>
              <p className={`text-xs font-bold ${getPercent(savingsTotal) < 20 ? 'text-amber-500' : 'text-emerald-500'}`}>
                {getPercent(savingsTotal)}%
              </p>
            </div>
          </div>
          {savingsBudgets.map((card, idx) => renderCard(card, idx))}
          {savingsBudgets.length === 0 && <p className="text-center text-sm text-slate-400 py-4">No Savings budgeted.</p>}
        </div>

      </div>
    </PageContainer>
  );
}
