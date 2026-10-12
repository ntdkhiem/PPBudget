"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useMemo } from "react";
import { apiFetch, budgetMonthParams, BudgetSummary, Category, CategorySpend } from "@/lib/api";
import { formatCurrency, parseDollarsToCents } from "@/lib/utils";
import { ChevronLeft, ChevronRight, Edit2, Plus, Target, TrendingUp, Calendar, AlertCircle } from "lucide-react";
import { format, addMonths, subMonths, startOfMonth, getDaysInMonth, getDate, isSameMonth } from "date-fns";
import { motion } from "framer-motion";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogDescription } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { BUCKETS, sortByUsage, toBudgetCards, type Bucket } from "./_components/budget-data";
import { BudgetCard } from "./_components/budget-card";
import { BucketColumn } from "./_components/bucket-column";
import { UnbudgetedCard } from "./_components/unbudgeted-card";

export default function BudgetsPage() {
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const [currentMonth, setCurrentMonth] = useState(startOfMonth(new Date()));
  const monthStr = format(currentMonth, "yyyy-MM");

  const [editingId, setEditingId] = useState<string | null>(null);

  // Paycheck: the saved value, or the draft while it is being edited.
  const [paycheckDraft, setPaycheckDraft] = useState<string | null>(null);
  const isEditingPaycheck = paycheckDraft !== null;

  const { data: paycheckData, isLoading: loadingPaycheck } = useQuery<{value: string}>({
    queryKey: ["setting", "expected_paycheck"],
    queryFn: () => apiFetch<{value: string}>("/settings/values/expected_paycheck", {}, token),
  });

  const savedPaycheckCents = parseDollarsToCents(paycheckData?.value ?? "") ?? 0;
  const draftPaycheckCents = paycheckDraft === null ? null : parseDollarsToCents(paycheckDraft);
  // While editing, the totals follow what is typed so its effect shows before saving.
  const expectedPaycheckCents = draftPaycheckCents ?? savedPaycheckCents;

  const savePaycheckMutation = useMutation({
    mutationFn: (value: string) => apiFetch("/settings/values/expected_paycheck", {
      method: "PUT",
      body: JSON.stringify({ value })
    }, token),
    onSuccess: (_data, value) => {
      queryClient.setQueryData(["setting", "expected_paycheck"], { value });
      setPaycheckDraft(null);
      toast.success("Expected paycheck saved");
    },
    onError: (err: Error) => {
      toast.error(err.message || "Failed to save expected paycheck");
    }
  });

  const handleSavePaycheck = () => {
    if (draftPaycheckCents === null) return;
    savePaycheckMutation.mutate((draftPaycheckCents / 100).toFixed(2));
  };

  // New Budget Modal State
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [newCategoryId, setNewCategoryId] = useState("");
  const [newLimit, setNewLimit] = useState("");
  const [newBucket, setNewBucket] = useState<Bucket>("needs");
  const [discardConfirmOpen, setDiscardConfirmOpen] = useState(false);

  // Delete Budget Modal State
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [budgetToDelete, setBudgetToDelete] = useState<string | null>(null);

  const { data: budgets, isLoading: loadingBudgets } = useQuery<BudgetSummary[]>({
    queryKey: ["budgets", monthStr],
    queryFn: () => apiFetch<BudgetSummary[]>(`/budgets/summary?${budgetMonthParams(monthStr)}`, {}, token),
  });

  const { data: unbudgeted, isLoading: loadingUnbudgeted } = useQuery<CategorySpend[]>({
    queryKey: ["budgets", "unbudgeted", monthStr],
    queryFn: () => apiFetch<CategorySpend[]>(`/budgets/unbudgeted?${budgetMonthParams(monthStr)}`, {}, token),
  });

  const { data: categories, isLoading: loadingCategories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const resetNewBudget = () => {
    setIsAddModalOpen(false);
    setNewCategoryId("");
    setNewLimit("");
    setNewBucket("needs");
  };

  const updateMutation = useMutation({
    mutationFn: (data: { id?: string, category_id: string, limit_amount: number, period_type: string, bucket: Bucket }) => {
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
          // Always the budget's own period type: defaulting it turned a
          // one-time budget into a monthly one that rolls forward.
          period_type: data.period_type,
          start_date: startDate,
          end_date: endDate,
          bucket: data.bucket,
        }),
      }, token);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["budgets"] });
      setEditingId(null);
      resetNewBudget();
    },
    onError: (err: Error) => {
      toast.error(err.message || "Failed to save budget");
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
    onError: (err: Error) => {
      toast.error(err.message || "Failed to delete budget");
    }
  });

  const changeMonth = (month: Date) => {
    setEditingId(null);
    setCurrentMonth(month);
  };
  const nextMonth = () => changeMonth(addMonths(currentMonth, 1));
  const prevMonth = () => changeMonth(subMonths(currentMonth, 1));

  const daysInMonth = getDaysInMonth(currentMonth);
  const today = new Date();
  const currentMonthStart = startOfMonth(today);
  const isFutureMonth = currentMonth > currentMonthStart;
  const isPastMonth = currentMonth < currentMonthStart;
  const isCurrentMonth = !isFutureMonth && !isPastMonth;
  const currentDay = isSameMonth(today, currentMonth) ? getDate(today) : (today > currentMonth ? daysInMonth : 1);

  const isLoading = loadingBudgets || loadingCategories;
  const budgetCards = useMemo(() => toBudgetCards(budgets ?? [], categories ?? []), [budgets, categories]);

  const totalLimit = budgetCards.reduce((acc, curr) => acc + curr.limit, 0);

  // Pacing is about spending, so it leaves Savings out: putting money away
  // early is not overspending.
  const spendingCards = budgetCards.filter(c => c.bucket !== "savings");
  const spendingLimit = spendingCards.reduce((acc, curr) => acc + curr.limit, 0);
  const spendingSpent = spendingCards.reduce((acc, curr) => acc + curr.spent, 0);
  const unbudgetedTotal = (unbudgeted ?? []).reduce((acc, s) => acc + s.total_spent, 0);

  const monthProgress = (currentDay / daysInMonth) * 100;
  const globalProgress = spendingLimit > 0 ? Math.min((spendingSpent / spendingLimit) * 100, 100) : 0;
  const globalRemaining = Math.max(0, spendingLimit - spendingSpent);
  const isOverPacing = globalProgress > monthProgress;
  const isOverSpent = spendingSpent > spendingLimit;

  const unallocatedCents = expectedPaycheckCents - totalLimit;
  const newLimitCents = parseDollarsToCents(newLimit);
  const isNewBudgetDirty = newCategoryId !== "" || newLimit !== "";

  // Closing a half-filled form asks first, so a stray click outside or Esc
  // doesn't throw the entry away.
  const handleAddBudgetOpenChange = (open: boolean) => {
    if (open) {
      setIsAddModalOpen(true);
    } else if (isNewBudgetDirty) {
      setDiscardConfirmOpen(true);
    } else {
      setIsAddModalOpen(false);
    }
  };

  // From the unbudgeted list: start a budget for that category at what was
  // spent, rounded up to whole dollars.
  const handleBudgetUnbudgeted = (categoryId: string, spentCents: number) => {
    setNewCategoryId(categoryId);
    setNewLimit(String(Math.ceil(spentCents / 100)));
    setNewBucket("needs");
    setIsAddModalOpen(true);
  };

  return (
    <PageContainer>
      <PageHeader
        title="Budgets"
        description={isFutureMonth ? "Viewing future projections. Return to the current month to make structural edits." : "Calibrate your budget to your paycheck with a 50/30/20 strategy."}
      >
        <div className="flex items-center gap-2 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-1 shadow-sm mr-2">
          <Button variant="ghost" size="icon" aria-label="Previous month" onClick={prevMonth} className="h-8 w-8 text-slate-500 hover:text-slate-900 dark:text-slate-100"><ChevronLeft size={18}/></Button>
          <span className="font-semibold w-24 text-center text-slate-700 dark:text-slate-300">{format(currentMonth, 'MMM yyyy')}</span>
          <Button variant="ghost" size="icon" aria-label="Next month" onClick={nextMonth} className="h-8 w-8 text-slate-500 hover:text-slate-900 dark:text-slate-100"><ChevronRight size={18}/></Button>
        </div>
        {!isFutureMonth && (
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
                  <Select value={newBucket} onValueChange={(val) => setNewBucket(val as Bucket)}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select bucket..." />
                    </SelectTrigger>
                    <SelectContent>
                      {BUCKETS.map(b => (
                        <SelectItem key={b.key} value={b.key}>{b.label} ({b.goalPercent}%)</SelectItem>
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
              <div className="flex flex-col gap-3 mt-6">
                <Button
                  onClick={() => newLimitCents !== null && updateMutation.mutate({ category_id: newCategoryId, limit_amount: newLimitCents, period_type: "monthly", bucket: newBucket })}
                  disabled={!newCategoryId || newLimitCents === null || updateMutation.isPending}
                  className="bg-indigo-600 hover:bg-indigo-700 text-white w-full"
                >
                  {updateMutation.isPending ? "Saving..." : "Add for This & Future Months"}
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => newLimitCents !== null && updateMutation.mutate({ category_id: newCategoryId, limit_amount: newLimitCents, period_type: "one_time", bucket: newBucket })}
                  disabled={!newCategoryId || newLimitCents === null || updateMutation.isPending}
                  className="w-full bg-indigo-50 text-indigo-700 hover:bg-indigo-100 dark:bg-indigo-900/30 dark:text-indigo-300 dark:hover:bg-indigo-900/50"
                >
                  {updateMutation.isPending ? "Saving..." : "Add Only for This Month"}
                </Button>
                <Button variant="outline" onClick={() => handleAddBudgetOpenChange(false)} className="w-full">
                  Cancel
                </Button>
              </div>
              {/* Rendered inside the dialog so Radix treats it as a child
                  layer: as a sibling, focusing it read as leaving the form,
                  which asked to close again and reopened this prompt. */}
              <ConfirmDialog
                open={discardConfirmOpen}
                onOpenChange={setDiscardConfirmOpen}
                title="Discard this budget?"
                description="The category and limit you entered will be lost."
                confirmText="Discard"
                cancelText="Keep editing"
                isDestructive
                onConfirm={() => {
                  setDiscardConfirmOpen(false);
                  resetNewBudget();
                }}
              />
            </DialogContent>
          </Dialog>
        )}

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
              {isLoading ? (
                <Skeleton className="h-8 w-32" />
              ) : (
                <p className="text-2xl font-bold text-slate-900 dark:text-slate-100">{formatCurrency(totalLimit)}</p>
              )}
            </div>

            <div className="bg-slate-50 dark:bg-slate-800/50 p-3 rounded-xl border border-slate-100 dark:border-slate-800">
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1">Expected Paycheck</p>
              {loadingPaycheck ? (
                <Skeleton className="h-8 w-32" />
              ) : isEditingPaycheck ? (
                <div className="flex items-center gap-2">
                  <span className="text-slate-500 font-bold">$</span>
                  <Input
                    type="number"
                    aria-label="Expected paycheck"
                    value={paycheckDraft}
                    onChange={e => setPaycheckDraft(e.target.value)}
                    className="w-24 h-8 px-2 font-bold text-slate-900 dark:text-slate-100"
                    autoFocus
                  />
                  <Button size="sm" onClick={handleSavePaycheck} disabled={draftPaycheckCents === null || savePaycheckMutation.isPending} className="h-8">Save</Button>
                </div>
              ) : (
                <button
                  type="button"
                  className="flex items-center gap-2 group"
                  onClick={() => setPaycheckDraft(savedPaycheckCents > 0 ? (savedPaycheckCents / 100).toString() : "")}
                >
                  <span className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
                    {formatCurrency(savedPaycheckCents)}
                  </span>
                  <Edit2 size={14} className="text-slate-400 group-hover:text-slate-600 opacity-0 group-hover:opacity-100 transition-opacity" />
                </button>
              )}
            </div>

            <div className="text-right">
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1">
                {unallocatedCents < 0 && expectedPaycheckCents > 0 ? "Over-Budgeted By" : "Left to Budget"}
              </p>
              {isLoading ? (
                <Skeleton className="h-8 w-28 ml-auto" />
              ) : (
                <p className={`text-2xl font-bold ${
                  expectedPaycheckCents === 0 ? 'text-slate-400'
                    : unallocatedCents < 0 ? 'text-red-500'
                    : 'text-slate-900 dark:text-slate-100'
                }`}>
                  {expectedPaycheckCents === 0 ? "—" : formatCurrency(Math.abs(unallocatedCents))}
                </p>
              )}
            </div>
          </div>
        </div>

        <div className="relative pt-6 pb-2 mt-8 border-t border-transparent">
          {/* Pacing Marker: only the current month has a "today" */}
          {isCurrentMonth && (
            <div
              className="absolute top-0 w-px h-full bg-slate-300 z-10 hidden md:block"
              style={{ left: `${monthProgress}%` }}
            >
              <div className="absolute -top-6 -translate-x-1/2 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-sm text-xs font-semibold text-slate-600 dark:text-slate-400 px-2 py-1 rounded-md flex items-center gap-1 whitespace-nowrap">
                <Calendar size={12}/> Today (Day {currentDay})
              </div>
            </div>
          )}

          <div className="h-4 bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden relative">
            {!isLoading && (
              <motion.div
                className={`h-full rounded-full relative z-20 ${
                  isPastMonth
                    ? (isOverSpent ? 'bg-red-500' : 'bg-emerald-500')
                    : isCurrentMonth && isOverPacing ? 'bg-amber-500' : 'bg-indigo-500'
                }`}
                initial={{ width: 0 }}
                animate={{ width: `${globalProgress}%` }}
                transition={{ duration: 1, ease: "easeOut" }}
              />
            )}
          </div>
        </div>

        <div className="flex flex-wrap justify-between items-center gap-2 mt-4 pt-4 border-t border-slate-100 dark:border-slate-800/50">
          {isLoading ? (
            <>
              <Skeleton className="h-5 w-48" />
              <Skeleton className="h-5 w-32" />
            </>
          ) : (
            <>
              <div className="flex items-center gap-2">
                {isPastMonth ? (
                  isOverSpent ? (
                    <>
                      <AlertCircle size={16} className="text-red-500" />
                      <span className="text-sm font-medium text-red-600">Finished over budget</span>
                    </>
                  ) : (
                    <>
                      <TrendingUp size={16} className="text-emerald-500" />
                      <span className="text-sm font-medium text-emerald-600">Finished within budget</span>
                    </>
                  )
                ) : isFutureMonth ? (
                  <>
                    <Calendar size={16} className="text-slate-400" />
                    <span className="text-sm font-medium text-slate-500">Starts {format(currentMonth, "MMMM d")}</span>
                  </>
                ) : isOverPacing ? (
                  <>
                    <AlertCircle size={16} className="text-amber-500" />
                    <span className="text-sm font-medium text-amber-600">Pacing ahead of schedule</span>
                  </>
                ) : (
                  <>
                    <TrendingUp size={16} className="text-emerald-500" />
                    <span className="text-sm font-medium text-emerald-600">On track for the month</span>
                  </>
                )}
              </div>
              <div className="text-sm font-medium text-slate-600 dark:text-slate-400 text-right">
                {isOverSpent
                  ? `${formatCurrency(spendingSpent - spendingLimit)} over`
                  : `${formatCurrency(globalRemaining)} ${isPastMonth ? "unspent" : "left to spend"}`}
                {unbudgetedTotal > 0 && (
                  <span className="text-amber-600 dark:text-amber-400"> · {formatCurrency(unbudgetedTotal)} unbudgeted</span>
                )}
              </div>
            </>
          )}
        </div>
      </motion.div>

      {/* 3-Column Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mt-8">
        {BUCKETS.map(({ key, label, goalPercent }) => {
          const cards = sortByUsage(budgetCards.filter(c => c.bucket === key));
          return (
            <BucketColumn
              key={key}
              bucket={key}
              label={label}
              goalPercent={goalPercent}
              totalLimit={cards.reduce((acc, c) => acc + c.limit, 0)}
              paycheckCents={expectedPaycheckCents}
              isLoading={isLoading}
              isEmpty={cards.length === 0}
            >
              {cards.map((card, idx) => (
                <BudgetCard
                  key={card.catId}
                  card={card}
                  index={idx}
                  isEditing={editingId === card.catId}
                  canEdit={!isFutureMonth}
                  isPastMonth={isPastMonth}
                  isSaving={updateMutation.isPending}
                  onStartEdit={() => setEditingId(card.catId)}
                  onCancelEdit={() => setEditingId(null)}
                  onSave={({ limitCents, bucket }) => updateMutation.mutate({
                    id: card.budgetId, category_id: card.catId, limit_amount: limitCents, period_type: card.periodType, bucket,
                  })}
                  onDelete={() => {
                    setBudgetToDelete(card.budgetId);
                    setDeleteModalOpen(true);
                  }}
                />
              ))}
            </BucketColumn>
          );
        })}
      </div>

      <div className="mt-8">
        <UnbudgetedCard
          spending={unbudgeted}
          isLoading={loadingUnbudgeted}
          onBudget={isFutureMonth ? undefined : handleBudgetUnbudgeted}
        />
      </div>
    </PageContainer>
  );
}
