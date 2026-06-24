"use client";

import React, { useRef, useState, useCallback, useMemo, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";
import { motion, AnimatePresence } from "framer-motion";
import { apiFetch, Transaction, Category } from "@/lib/api";
import { formatCurrency } from "@/lib/utils";
import { format, parseISO } from "date-fns";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { toast } from "sonner";
import { Inbox, CheckCircle2, Check, LayoutGrid, Layers, Loader2 } from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";

export default function ReviewPage() {
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
  const queryClient = useQueryClient();

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [activeTxnId, setActiveTxnId] = useState<string | null>(null);

  const { data: transactions, isLoading } = useQuery<Transaction[]>({
    queryKey: ["unreviewed"],
    queryFn: () => apiFetch<Transaction[]>("/transactions?unreviewed=true", {}, token),
    refetchInterval: 30000,
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const reviewMutation = useMutation({
    mutationFn: ({ id, categoryId }: { id: string; categoryId: string | null }) =>
      apiFetch(`/transactions/${id}/review`, {
        method: "PATCH",
        body: JSON.stringify(categoryId ? { category_id: categoryId } : {}),
      }, token),
    onMutate: async ({ id, categoryId }) => {
      await queryClient.cancelQueries({ queryKey: ["unreviewed"] });
      const previous = queryClient.getQueryData<Transaction[]>(["unreviewed"]);
      queryClient.setQueryData<Transaction[]>(["unreviewed"], (old) => 
        old?.filter(t => t.id !== id) || []
      );
      return { previous };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (err, variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(["unreviewed"], context.previous);
      }
      toast.error("Failed to approve transaction");
    },
  });

  const bulkReviewMutation = useMutation({
    mutationFn: async ({ ids, categoryId }: { ids: string[]; categoryId: string | null }) => {
      // For a real app, backend should have a bulk endpoint.
      // Here we just fire promises.
      await Promise.all(
        ids.map(id => apiFetch(`/transactions/${id}/review`, {
          method: "PATCH",
          body: JSON.stringify(categoryId ? { category_id: categoryId } : {}),
        }, token))
      );
    },
    onSuccess: () => {
      toast.success(`Successfully approved ${selectedIds.size} transactions`);
      setSelectedIds(new Set());
      queryClient.invalidateQueries({ queryKey: ["unreviewed"] });
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: () => toast.error("Failed to process bulk review"),
  });

  // Virtualization
  const parentRef = useRef<HTMLDivElement>(null);
  const rowVirtualizer = useVirtualizer({
    count: transactions?.length || 0,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 76,
    overscan: 10,
  });

  // Keyboard Navigation
  useEffect(() => {
    if (!transactions || transactions.length === 0) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement) return;
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const currentIndex = transactions.findIndex(t => t.id === activeTxnId);
        let nextIndex = 0;
        if (e.key === "ArrowDown") {
          nextIndex = currentIndex < transactions.length - 1 ? currentIndex + 1 : 0;
        } else {
          nextIndex = currentIndex > 0 ? currentIndex - 1 : transactions.length - 1;
        }
        setActiveTxnId(transactions[nextIndex].id);
        rowVirtualizer.scrollToIndex(nextIndex, { align: "center" });
      }
      if (e.key === "Enter" && activeTxnId) {
        e.preventDefault();
        reviewMutation.mutate({ id: activeTxnId, categoryId: null });
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [transactions, activeTxnId, rowVirtualizer, reviewMutation]);

  const toggleSelection = (id: string) => {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelectedIds(next);
  };

  const selectAll = () => {
    if (!transactions) return;
    if (selectedIds.size === transactions.length) setSelectedIds(new Set());
    else setSelectedIds(new Set(transactions.map(t => t.id)));
  };

  if (isLoading) {
    return <div className="flex h-[50vh] items-center justify-center"><Loader2 className="h-8 w-8 animate-spin text-indigo-500" /></div>;
  }

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }} className="space-y-6 pb-24 h-[calc(100vh-8rem)] flex flex-col">
      <div className="flex flex-col md:flex-row md:items-end justify-between shrink-0">
        <div>
          <h1 className="text-4xl font-bold font-heading text-slate-900 dark:text-white mb-2 flex items-center gap-3">
            <div className="p-2 bg-amber-100 dark:bg-amber-500/20 text-amber-600 dark:text-amber-400 rounded-xl">
              <Inbox className="h-6 w-6" />
            </div>
            Needs Review
          </h1>
          <p className="text-slate-500 dark:text-slate-400 text-lg">
            Approve or categorize {transactions?.length || 0} newly imported transactions.
          </p>
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl border border-slate-200/60 dark:border-slate-800/60 rounded-3xl shadow-sm flex-1 flex flex-col overflow-hidden relative">
        <div className="px-6 py-4 border-b border-slate-200/60 dark:border-slate-800/60 flex items-center justify-between bg-slate-50/50 dark:bg-slate-800/50">
          <div className="flex items-center gap-4">
            <input 
              type="checkbox" 
              checked={(transactions?.length ?? 0) > 0 && selectedIds.size === transactions?.length}
              onChange={selectAll}
              className="w-5 h-5 rounded-md border-slate-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer"
            />
            <span className="text-sm font-medium text-slate-600 dark:text-slate-400">Select All</span>
          </div>
          <div className="text-sm text-slate-500 flex gap-4">
            <span className="hidden sm:inline">Use <kbd className="bg-slate-200 dark:bg-slate-800 px-1.5 py-0.5 rounded text-xs">↑</kbd> <kbd className="bg-slate-200 dark:bg-slate-800 px-1.5 py-0.5 rounded text-xs">↓</kbd> to navigate</span>
            <span className="hidden sm:inline">Press <kbd className="bg-slate-200 dark:bg-slate-800 px-1.5 py-0.5 rounded text-xs">Enter</kbd> to approve</span>
          </div>
        </div>

        {transactions?.length === 0 ? (
          <div className="flex-1 flex items-center justify-center">
            <EmptyState 
              icon={CheckCircle2} 
              title="All caught up!" 
              description="No transactions need review right now. You're fully categorized." 
            />
          </div>
        ) : (
          <div ref={parentRef} className="flex-1 overflow-auto custom-scrollbar">
            <div style={{ height: `${rowVirtualizer.getTotalSize()}px`, width: '100%', position: 'relative' }}>
              <AnimatePresence>
                {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                  const txn = transactions![virtualRow.index];
                  const isSelected = selectedIds.has(txn.id);
                  const isActive = activeTxnId === txn.id;
                  
                  return (
                    <motion.div
                      key={txn.id}
                      initial={{ opacity: 0, scale: 0.98 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, x: -50, transition: { duration: 0.2 } }}
                      style={{
                        position: 'absolute',
                        top: `${virtualRow.start}px`,
                        left: 0,
                        width: '100%',
                        height: `${virtualRow.size}px`,
                      }}
                      className={`flex items-center px-6 border-b border-slate-100 dark:border-slate-800/60 transition-colors cursor-pointer group ${isActive ? 'bg-indigo-50/50 dark:bg-indigo-900/10' : 'hover:bg-slate-50 dark:hover:bg-slate-800/40'} ${isSelected ? 'bg-indigo-50 dark:bg-indigo-900/20' : ''}`}
                      onClick={() => setActiveTxnId(txn.id)}
                    >
                      <div className="flex items-center gap-4 w-full h-full py-4">
                        <input 
                          type="checkbox" 
                          checked={isSelected}
                          onChange={() => toggleSelection(txn.id)}
                          onClick={(e) => e.stopPropagation()}
                          className="w-5 h-5 rounded-md border-slate-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer"
                        />
                        <div className="flex-1 min-w-0">
                          <p className="font-medium text-slate-900 dark:text-slate-100 truncate text-lg">
                            {txn.description}
                          </p>
                          <p className="text-sm text-slate-500">
                            {format(parseISO(txn.date), "MMM d, yyyy")}
                          </p>
                        </div>
                        
                        <div className="hidden md:block mx-4">
                          <Popover>
                            <PopoverTrigger asChild onClick={(e) => e.stopPropagation()}>
                              <Button variant="ghost" className={`h-10 px-4 rounded-xl text-sm font-medium border ${txn.category_id ? 'border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-300' : 'border-indigo-200 text-indigo-600 bg-indigo-50 hover:bg-indigo-100 dark:text-indigo-400 dark:bg-indigo-500/10 dark:border-indigo-500/30'}`}>
                                {categories?.find(c => c.id === txn.category_id)?.name || "Assign Category"}
                              </Button>
                            </PopoverTrigger>
                            <PopoverContent className="w-64 p-2 rounded-2xl" align="center">
                              <h4 className="font-medium text-sm px-2 py-2 text-slate-500">Select Category</h4>
                              <div className="max-h-60 overflow-y-auto custom-scrollbar pr-1">
                                {categories?.map((cat) => (
                                  <div
                                    key={cat.id}
                                    className={`px-3 py-2 text-sm rounded-xl cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-between mb-1 ${txn.category_id === cat.id ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400' : ''}`}
                                    onClick={(e) => { e.stopPropagation(); reviewMutation.mutate({ id: txn.id, categoryId: cat.id }); }}
                                  >
                                    {cat.name}
                                    {txn.category_id === cat.id && <Check className="h-4 w-4" />}
                                  </div>
                                ))}
                              </div>
                            </PopoverContent>
                          </Popover>
                        </div>

                        <div className="text-right ml-4">
                          <p className={`font-semibold text-lg ${txn.amount < 0 ? "text-rose-600 dark:text-rose-400" : "text-emerald-600 dark:text-emerald-400"}`}>
                            {formatCurrency(txn.amount)}
                          </p>
                        </div>

                        <div className="ml-6 flex items-center">
                          <Button
                            onClick={(e) => { e.stopPropagation(); reviewMutation.mutate({ id: txn.id, categoryId: null }); }}
                            className="h-10 w-10 sm:w-auto px-0 sm:px-6 rounded-xl bg-emerald-100 hover:bg-emerald-200 text-emerald-700 dark:bg-emerald-500/20 dark:hover:bg-emerald-500/30 dark:text-emerald-400 shadow-none border-0"
                          >
                            <Check className="h-5 w-5 sm:mr-2" />
                            <span className="hidden sm:inline font-medium">Approve</span>
                          </Button>
                        </div>
                      </div>
                    </motion.div>
                  );
                })}
              </AnimatePresence>
            </div>
          </div>
        )}
      </div>

      {/* Bulk Action Bar */}
      <AnimatePresence>
        {selectedIds.size > 0 && (
          <motion.div 
            initial={{ opacity: 0, y: 50, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 50, scale: 0.95 }}
            className="fixed bottom-8 left-1/2 -translate-x-1/2 bg-slate-900/90 dark:bg-slate-100/90 backdrop-blur-xl shadow-2xl rounded-2xl px-6 py-4 flex items-center gap-6 z-50 border border-slate-700 dark:border-slate-300"
          >
            <div className="flex items-center gap-3">
              <div className="bg-indigo-600 text-white font-bold h-8 w-8 rounded-full flex items-center justify-center text-sm shadow-md">
                {selectedIds.size}
              </div>
              <span className="text-white dark:text-slate-900 font-medium">Selected</span>
            </div>
            
            <div className="h-8 w-px bg-slate-700 dark:bg-slate-300 mx-2"></div>
            
            <div className="flex gap-3">
              <Button 
                onClick={() => bulkReviewMutation.mutate({ ids: Array.from(selectedIds), categoryId: null })}
                disabled={bulkReviewMutation.isPending}
                className="bg-emerald-500 hover:bg-emerald-600 text-white border-0 shadow-lg shadow-emerald-500/20 rounded-xl px-6"
              >
                {bulkReviewMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : <Check className="w-4 h-4 mr-2" />}
                Approve All
              </Button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
