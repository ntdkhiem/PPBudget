"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useMemo } from "react";
import { apiFetch, BudgetSummary, Category } from "@/lib/api";
import { formatCurrency } from "@/lib/utils";
import { ChevronLeft, ChevronRight, Edit2, Check, X, Plus, Target, TrendingUp, Calendar, AlertCircle } from "lucide-react";
import { format, addMonths, subMonths, startOfMonth, getDaysInMonth, getDate, isSameMonth } from "date-fns";
import { motion, AnimatePresence } from "framer-motion";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogTrigger, DialogDescription } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Progress } from "@/components/ui/progress";

export default function BudgetsPage() {
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
  
  const [currentMonth, setCurrentMonth] = useState(startOfMonth(new Date()));
  const monthStr = format(currentMonth, "yyyy-MM");

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editLimit, setEditLimit] = useState("");
  
  // New Budget Modal State
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [newCategoryId, setNewCategoryId] = useState("");
  const [newLimit, setNewLimit] = useState("");

  const { data: budgets, isLoading: loadingBudgets } = useQuery<BudgetSummary[]>({
    queryKey: ["budgets", monthStr],
    queryFn: () => apiFetch<BudgetSummary[]>(`/budgets/summary?month=${monthStr}`, {}, token),
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const updateMutation = useMutation({
    mutationFn: (data: { id?: string, category_id: string, limit_amount: number }) => {
      const year = currentMonth.getFullYear();
      const month = currentMonth.getMonth();
      // Ensure strict local-to-UTC date strings to avoid timezone drift
      const startDate = format(new Date(year, month, 1), "yyyy-MM-dd") + "T00:00:00Z";
      const endDate = format(new Date(year, month + 1, 0), "yyyy-MM-dd") + "T00:00:00Z";
      const catName = categories?.find(c => c.id === data.category_id)?.name || "Budget";

      return apiFetch(data.id ? `/budgets/${data.id}` : "/budgets", {
        method: data.id ? "PUT" : "POST",
        body: JSON.stringify({ 
          name: `${catName} Budget`,
          category_id: data.category_id, 
          amount_cents: data.limit_amount, 
          period_type: "monthly",
          start_date: startDate,
          end_date: endDate
        }),
      }, token);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["budgets", monthStr] });
      setEditingId(null);
      setIsAddModalOpen(false);
      setNewCategoryId("");
      setNewLimit("");
    },
  });

  const nextMonth = () => setCurrentMonth(addMonths(currentMonth, 1));
  const prevMonth = () => setCurrentMonth(subMonths(currentMonth, 1));

  // Global metrics
  const daysInMonth = getDaysInMonth(currentMonth);
  const today = new Date();
  const currentDay = isSameMonth(today, currentMonth) ? getDate(today) : (today > currentMonth ? daysInMonth : 1);
  const monthProgress = (currentDay / daysInMonth) * 100;
  const daysRemaining = daysInMonth - currentDay + 1; // including today

  const budgetCards = useMemo(() => {
    return categories?.map(cat => {
      const budget = budgets?.find(b => b.category_id === cat.id);
      return {
        catId: cat.id,
        catName: cat.name,
        budgetId: budget?.id,
        limit: budget?.amount_cents || 0,
        spent: budget?.spent_total || 0,
      };
    }).filter(c => c.limit > 0 || c.spent > 0) || [];
  }, [categories, budgets]);

  const totalLimit = budgetCards.reduce((acc, curr) => acc + curr.limit, 0);
  const totalSpent = budgetCards.reduce((acc, curr) => acc + curr.spent, 0);
  const globalProgress = totalLimit > 0 ? Math.min((totalSpent / totalLimit) * 100, 100) : 0;
  const globalRemaining = Math.max(0, totalLimit - totalSpent);
  
  const isOverPacing = globalProgress > monthProgress;

  if (loadingBudgets) return <div className="p-8 text-center text-slate-500">Loading budgets...</div>;

  return (
    <div className="max-w-6xl mx-auto space-y-8 pb-12">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-slate-100 mb-2">Budgets</h1>
          <p className="text-slate-500">Track and manage your spending limits.</p>
        </div>
        
        <div className="flex items-center gap-4">
          <div className="flex items-center bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 px-2 py-1.5">
            <Button variant="ghost" size="icon" onClick={prevMonth} className="h-8 w-8 text-slate-500 hover:text-slate-900 dark:text-slate-100"><ChevronLeft size={18}/></Button>
            <span className="font-semibold text-sm w-32 text-center text-slate-800 dark:text-slate-100">
              {format(currentMonth, "MMMM yyyy")}
            </span>
            <Button variant="ghost" size="icon" onClick={nextMonth} className="h-8 w-8 text-slate-500 hover:text-slate-900 dark:text-slate-100"><ChevronRight size={18}/></Button>
          </div>

          <Dialog open={isAddModalOpen} onOpenChange={setIsAddModalOpen}>
            <DialogTrigger asChild>
              <Button className="bg-indigo-600 hover:bg-indigo-700 text-white shadow-sm rounded-xl px-5 h-11 transition-all active:scale-95 flex items-center gap-2">
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
                  <Label>Monthly Limit ($)</Label>
                  <Input 
                    type="number" 
                    placeholder="e.g. 500" 
                    value={newLimit}
                    onChange={e => setNewLimit(e.target.value)}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsAddModalOpen(false)}>Cancel</Button>
                <Button 
                  onClick={() => updateMutation.mutate({ category_id: newCategoryId, limit_amount: Math.round(parseFloat(newLimit) * 100) })}
                  disabled={!newCategoryId || !newLimit || updateMutation.isPending}
                  className="bg-indigo-600 hover:bg-indigo-700 text-white"
                >
                  {updateMutation.isPending ? "Saving..." : "Save Budget"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      {/* Global Month Overview */}
      <motion.div 
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="bg-white dark:bg-slate-900 p-6 md:p-8 rounded-2xl shadow-sm border border-slate-200 dark:border-slate-800/60 relative overflow-hidden"
      >
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-6">
          <div>
            <h2 className="text-lg font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2 mb-1">
              <Target size={20} className="text-indigo-500"/> Total Month Overview
            </h2>
            <p className="text-sm text-slate-500">All budgeted categories combined</p>
          </div>
          
          <div className="flex gap-8">
            <div>
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1">Total Spent</p>
              <p className="text-2xl font-bold text-slate-900 dark:text-slate-100">{formatCurrency(totalSpent)}</p>
            </div>
            <div className="text-right">
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1">Total Budget</p>
              <p className="text-2xl font-bold text-slate-400">{formatCurrency(totalLimit)}</p>
            </div>
          </div>
        </div>

        <div className="relative pt-6 pb-2">
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

      {/* Detailed Budget Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {budgetCards.length === 0 ? (
          <div className="col-span-full text-center py-12 bg-slate-50 dark:bg-slate-800/50 rounded-2xl border border-slate-200 dark:border-slate-800/60 text-slate-500">
            No budgets found for this month.
          </div>
        ) : (
          budgetCards.map((card, idx) => {
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
                className={`bg-white dark:bg-slate-900 p-6 rounded-2xl shadow-sm flex flex-col border transition-all ${
                  isOverLimit && card.limit > 0 
                    ? 'bg-red-50/30 border-red-200 shadow-[0_0_15px_rgba(239,68,68,0.05)]' 
                    : 'border-slate-200 dark:border-slate-800/60 hover:shadow-md hover:border-slate-300'
                }`}
              >
                <div className="flex justify-between items-center mb-6">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${isOverLimit ? 'bg-red-100 text-red-600' : 'bg-indigo-50 dark:bg-indigo-900/30 text-indigo-600'}`}>
                      <Target size={18} />
                    </div>
                    <h3 className="text-lg font-bold text-slate-800 dark:text-slate-100 truncate pr-2">{card.catName}</h3>
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
                          onClick={() => updateMutation.mutate({ id: card.budgetId, category_id: card.catId, limit_amount: Math.round(parseFloat(editLimit) * 100) })}
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
                      >
                        <Button
                          size="icon-sm"
                          variant="ghost"
                          onClick={() => { setEditingId(card.catId); setEditLimit((card.limit / 100).toString()); }}
                          className="text-slate-400 hover:text-indigo-600 h-8 w-8"
                        >
                          <Edit2 size={14} />
                        </Button>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>

                <div className="flex justify-between items-end mb-4">
                  <div>
                    <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-1">Spent</p>
                    <p className={`text-2xl font-bold tracking-tight ${isOverLimit && card.limit > 0 ? "text-red-600" : "text-slate-900 dark:text-slate-100"}`}>
                      {formatCurrency(card.spent)}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-1">Limit</p>
                    <AnimatePresence mode="wait">
                      {isEditing ? (
                        <motion.div 
                          key="edit-input"
                          initial={{ opacity: 0, x: 10 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: -10 }}
                          className="flex items-center justify-end h-8"
                        >
                          <span className="text-slate-500 mr-1 font-semibold">$</span>
                          <Input 
                            type="number" 
                            value={editLimit} 
                            onChange={e => setEditLimit(e.target.value)}
                            className="w-24 h-8 px-2 text-right font-semibold text-slate-900 dark:text-slate-100"
                            autoFocus
                          />
                        </motion.div>
                      ) : (
                        <motion.p 
                          key="view-limit"
                          initial={{ opacity: 0, x: -10 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: 10 }}
                          className="text-lg font-semibold text-slate-500 h-8 flex items-center justify-end"
                        >
                          {formatCurrency(card.limit)}
                        </motion.p>
                      )}
                    </AnimatePresence>
                  </div>
                </div>

                <div className="w-full bg-slate-100 dark:bg-slate-800 rounded-full h-2 mb-4 overflow-hidden">
                  <motion.div 
                    className={`h-full rounded-full ${
                      isOverLimit ? 'bg-red-500' : progress > 80 ? 'bg-amber-400' : 'bg-emerald-500'
                    }`}
                    initial={{ width: 0 }}
                    animate={{ width: `${progress}%` }}
                    transition={{ duration: 1, ease: "easeOut" }}
                  ></motion.div>
                </div>
                
                {card.limit > 0 && (
                  <div className="grid grid-cols-2 gap-2 mt-auto pt-4 border-t border-slate-100 dark:border-slate-800/50">
                    <div className="bg-slate-50 dark:bg-slate-800/50 rounded-lg p-2 text-center">
                      <p className="text-[10px] uppercase font-bold text-slate-400 mb-0.5">Avg / Day</p>
                      <p className="text-sm font-semibold text-slate-700 dark:text-slate-300">{formatCurrency(avgSpentPerDay)}</p>
                    </div>
                    <div className="bg-slate-50 dark:bg-slate-800/50 rounded-lg p-2 text-center">
                      <p className="text-[10px] uppercase font-bold text-slate-400 mb-0.5">Safe / Day</p>
                      <p className={`text-sm font-semibold ${safeToSpendPerDay === 0 ? 'text-red-500' : 'text-emerald-600'}`}>
                        {formatCurrency(safeToSpendPerDay)}
                      </p>
                    </div>
                  </div>
                )}
              </motion.div>
            );
          })
        )}
      </div>
    </div>
  );
}
