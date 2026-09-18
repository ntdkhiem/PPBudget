"use client";

import { useState, useMemo, useEffect, useRef, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, Transaction, Category, Account, Subscription } from "@/lib/api";
import { cn } from "@/lib/utils";
import { format, parseISO } from "date-fns";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useDateRange } from "@/app/contexts/DateRangeContext";
import { useDebounce } from "use-debounce";
import dynamic from "next/dynamic";
import TransactionFilters from "./TransactionFilters";

const AddTransactionDialog = dynamic(() => import('./AddTransactionDialog'), { ssr: false });
const EditTransactionDialog = dynamic(() => import('./EditTransactionDialog'), { ssr: false });

import { motion } from "framer-motion";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "sonner";
import { Plus, Trash2, Loader2, Edit2, SearchX, Inbox, ExternalLink, Check, ChevronsUpDown, Search, ListFilter, ArrowRightLeft, XCircle, ChevronDown, Wallet, Unlink, X } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { DashboardCard } from "@/components/dashboard-card";
import { PageHeader } from "@/components/page-header";
import { TransactionTableRow } from "@/components/transaction-table-row";

export default function TransactionsPage() {
  const parentRef = useRef<HTMLDivElement>(null);

  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
  const queryClient = useQueryClient();

  const [isAddOpen, setIsAddOpen] = useState(false);
  const [selectedTxn, setSelectedTxn] = useState<Transaction | null>(null);
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [quickEditTxnId, setQuickEditTxnId] = useState<string | null>(null);
  const [cursorStack, setCursorStack] = useState<{date: string, id: string}[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [isBulkCategoryOpen, setIsBulkCategoryOpen] = useState(false);
  const [isBulkDeleteOpen, setIsBulkDeleteOpen] = useState(false);
  const [bulkCategoryId, setBulkCategoryId] = useState("");
  const searchParams = useSearchParams();
  const router = useRouter();

    const [paidBy, setPaidBy] = useState<{transaction_id: string, amount: number, description?: string, date?: string}[]>([]);
  const [paysFor, setPaysFor] = useState<{transaction_id: string, amount: number, description?: string, date?: string}[]>([]);

  // Smart Filtering States
  const [searchQuery, setSearchQuery] = useState('');
  const [debouncedSearchQuery] = useDebounce(searchQuery, 300);
  const [selectedCategories, setSelectedCategories] = useState<string[]>([]);
  const [selectedAccounts, setSelectedAccounts] = useState<string[]>([]);
  const [selectedType, setSelectedType] = useState('All');

  useEffect(() => {
    if (selectedTxn) {
      setPaidBy(selectedTxn.paid_by || []);
      setPaysFor(selectedTxn.pays_for || []);
    }
  }, [selectedTxn]);

  useEffect(() => {
    const editId = searchParams.get("edit");
    if (editId) {
      // Fetch this specific transaction
      apiFetch<Transaction>(`/transactions/${editId}`, {}, token)
        .then((txn) => {
          setSelectedTxn(txn);
          setIsEditOpen(true);
          // Remove from URL so it doesn't trigger again on refresh
          router.replace("/transactions", { scroll: false });
        })
        .catch((err) => {
          console.error("Failed to fetch transaction for edit:", err);
        });
    }
  }, [searchParams, router, token]);


  const { date } = useDateRange();

  useEffect(() => {
    setCursorStack([]);
  }, [date]);

  const currentCursor = cursorStack[cursorStack.length - 1];

  const queryParams = useMemo(() => {
    const params = new URLSearchParams();
    if (debouncedSearchQuery) {
      params.append("search", debouncedSearchQuery);
    } else {
      if (date?.from) params.append("start_date", format(date.from, "yyyy-MM-dd"));
      if (date?.to) params.append("end_date", format(date.to, "yyyy-MM-dd"));
    }
    if (currentCursor) {
      params.append("cursor_date", currentCursor.date);
      params.append("cursor_id", currentCursor.id);
    }
    if (selectedCategories.length > 0) {
      params.append("categories", selectedCategories.join(","));
    }
    if (selectedAccounts.length > 0) {
      params.append("accounts", selectedAccounts.join(","));
    }
    if (selectedType && selectedType !== "All") {
      params.append("type", selectedType);
    }
    return params.toString();
  }, [date, currentCursor, debouncedSearchQuery, selectedCategories, selectedAccounts, selectedType]);

  const { data: unreviewedTransactions } = useQuery<Transaction[]>({
    queryKey: ["unreviewed"],
    queryFn: () => apiFetch<Transaction[]>("/transactions?unreviewed=true", {}, token),
    refetchInterval: 30000,
  });

  const { data: transactions, isLoading } = useQuery<Transaction[]>({
    queryKey: ["transactions", queryParams],
    queryFn: () => apiFetch<Transaction[]>(`/transactions?${queryParams}`, {}, token),
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const { data: accounts } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch("/accounts", {}, token),
    enabled: !!token,
  });

  const { data: subscriptions } = useQuery<Subscription[]>({
    queryKey: ["subscriptions"],
    queryFn: () => apiFetch("/subscriptions", {}, token),
    enabled: !!token,
  });

  const filteredTransactions = useMemo(() => {
    return transactions || [];
  }, [transactions]);

  const toggleCategory = (categoryId: string) => {
    setSelectedCategories(prev =>
      prev.includes(categoryId) ? prev.filter(c => c !== categoryId) : [...prev, categoryId]
    );
  };

  const toggleAccount = (accountId: string) => {
    setSelectedAccounts(prev =>
      prev.includes(accountId) ? prev.filter(a => a !== accountId) : [...prev, accountId]
    );
  };

  const clearFilters = () => {
    setSearchQuery('');
    setSelectedCategories([]);
    setSelectedAccounts([]);
    setSelectedType('All');
  };
  const hasActiveFilters = searchQuery.length > 0 || selectedCategories.length > 0 || selectedAccounts.length > 0 || selectedType !== 'All';

  const bulkDeleteMutation = useMutation({
    mutationFn: (ids: string[]) => apiFetch("/transactions/bulk/delete", {
      method: "POST",
      body: JSON.stringify({ transaction_ids: ids }),
    }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
      setSelectedIds([]);
      setIsBulkDeleteOpen(false);
      toast.success("Transactions deleted successfully");
    },
    onError: () => toast.error("Failed to delete transactions"),
  });

  const bulkCategoryMutation = useMutation({
    mutationFn: ({ ids, catId }: { ids: string[], catId: string }) => apiFetch("/transactions/bulk/category", {
      method: "POST",
      body: JSON.stringify({ transaction_ids: ids, category_id: catId }),
    }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
      setSelectedIds([]);
      setIsBulkCategoryOpen(false);
      setBulkCategoryId("");
      toast.success("Categories updated successfully");
    },
    onError: () => toast.error("Failed to update categories"),
  });

  const handleSelectAll = (checked: boolean) => {
    if (checked && transactions) {
      setSelectedIds(transactions.map(t => t.id));
    } else {
      setSelectedIds([]);
    }
  };

  const handleSelectRow = useCallback((id: string, checked: boolean) => {
    if (checked) {
      setSelectedIds(prev => [...prev, id]);
    } else {
      setSelectedIds(prev => prev.filter(x => x !== id));
    }
  }, []);

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/transactions/${id}`, { method: "DELETE" }, token),
    onSuccess: () => {
      toast.success("Transaction deleted successfully");
      setIsEditOpen(false);
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to delete transaction");
    },
  });

  const reviewMutation = useMutation({
    mutationFn: ({ id, categoryId }: { id: string; categoryId: string }) =>
      apiFetch(
        `/transactions/${id}/review`,
        {
          method: "PATCH",
          body: JSON.stringify({ category_id: categoryId }),
        },
        token
      ),
    onSuccess: () => {
      toast.success("Transaction categorized successfully");
      setQuickEditTxnId(null);
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
      queryClient.invalidateQueries({ queryKey: ["unreviewed"] });
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to categorize transaction");
    },
  });

  const handleReview = useCallback((id: string, categoryId: string) => {
    reviewMutation.mutate({ id, categoryId });
  }, [reviewMutation]);

  // Removed unused approveMutation as it's now handled in /review page

  const createMutation = useMutation({
    mutationFn: (data: any) =>
      apiFetch("/transactions", { method: "POST", body: JSON.stringify(data) }, token),
    onSuccess: () => {
      toast.success("Transaction created successfully");
      setIsAddOpen(false);
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to create transaction");
    },
  });

  const updateMutation = useMutation({
    mutationFn: (data: any) =>
      apiFetch(`/transactions/${data.id}`, { method: "PUT", body: JSON.stringify(data) }, token),
    onSuccess: () => {
      toast.success("Transaction updated successfully");
      setIsEditOpen(false);
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to update transaction");
    },
  });

  const handleDelete = useCallback((id: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (confirm("Are you sure you want to delete this transaction?")) {
      deleteMutation.mutate(id);
    }
  }, [deleteMutation]);

  const handleCreateSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    createMutation.mutate({
      account_id: formData.get("accountId"),
      amount: Math.round(parseFloat(formData.get("amount") as string) * 100), // convert to cents
      date: formData.get("date"),
      description: formData.get("description"),
      notes: formData.get("notes") || null,
      category_id: formData.get("categoryId") || null,
      subscription_id: formData.get("subscriptionId") === "none" ? null : formData.get("subscriptionId") || null,
      pays_for: paysFor,
      paid_by: paidBy,
    });
  };

  const handleUpdateSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!selectedTxn) return;
    const formData = new FormData(e.currentTarget);
    updateMutation.mutate({
      id: selectedTxn.id,
      account_id: formData.get("accountId"),
      amount: Math.round(parseFloat(formData.get("amount") as string) * 100),
      date: formData.get("date"),
      description: formData.get("description"),
      notes: formData.get("notes") || null,
      category_id: formData.get("categoryId") || null,
      subscription_id: formData.get("subscriptionId") === "none" ? null : formData.get("subscriptionId") || null,
      pays_for: paysFor, paid_by: paidBy,
    });
  };

  const handleRowClick = useCallback((txn: Transaction) => {
    setSelectedTxn(txn);
    setIsEditOpen(true);
  }, []);

  // Hands the transaction off to the rules page, which opens its create dialog
  // prefilled and then strips these params.
  const handleCreateRule = useCallback((txn: Transaction) => {
    const params = new URLSearchParams({ new: "1", description: txn.description });
    if (txn.account_id) params.set("account", txn.account_id);
    if (txn.category_id) params.set("category", txn.category_id);
    router.push(`/settings/rules?${params.toString()}`);
  }, [router]);

  const handleNextPage = () => {
    if (transactions && transactions.length === 100) {
      const last = transactions[transactions.length - 1];
      setCursorStack([...cursorStack, { date: last.date, id: last.id }]);
    }
  };

  const handlePrevPage = () => {
    if (cursorStack.length > 0) {
      setCursorStack(cursorStack.slice(0, -1));
    }
  };
  const rowVirtualizer = useVirtualizer({
    count: filteredTransactions.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 64, // Approximate row height
    overscan: 10,
  });

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.5 }} className="space-y-6 pb-10">
      <PageHeader title="Transactions" description="View and manage your transactions.">
        <Button 
          variant="outline"
          asChild
          className="rounded-xl shadow-sm text-amber-600 border-amber-200 hover:bg-amber-50 flex items-center gap-2 px-5 h-11 bg-white dark:bg-slate-900 transition-all active:scale-95"
        >
          <a href="/review">
            <Inbox className="h-4 w-4" /> 
            Needs Review {unreviewedTransactions?.length ? `(${unreviewedTransactions.length})` : ''}
          </a>
        </Button>
        <AddTransactionDialog
          isOpen={isAddOpen}
          onOpenChange={setIsAddOpen}
          onSubmit={handleCreateSubmit}
          isPending={createMutation.isPending}
          accounts={accounts}
          categories={categories}
          subscriptions={subscriptions}
        />
      </PageHeader>

      <TransactionFilters 
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        selectedCategories={selectedCategories}
        toggleCategory={toggleCategory}
        selectedAccounts={selectedAccounts}
        toggleAccount={toggleAccount}
        selectedType={selectedType}
        setSelectedType={setSelectedType}
        hasActiveFilters={hasActiveFilters}
        clearFilters={clearFilters}
        categories={categories}
        accounts={accounts}
      />

      <div ref={parentRef} className="h-[600px] overflow-auto relative rounded-3xl border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 bg-white dark:bg-slate-900/80 dark:bg-slate-900/80 backdrop-blur-xl shadow-sm">
        <Table className="relative w-full">
          <TableHeader>
            <TableRow className="hover:bg-transparent border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60">
              <TableHead className="w-12 py-4 text-center">
                <input 
                  type="checkbox" 
                  className="w-4 h-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer"
                  checked={filteredTransactions.length ? selectedIds.length === filteredTransactions.length : false}
                  onChange={(e) => handleSelectAll(e.target.checked)}
                />
              </TableHead>
              <TableHead className="font-semibold text-slate-500 py-4">Description</TableHead>
              <TableHead className="font-semibold text-slate-500 text-right py-4">Amount</TableHead>
              <TableHead className="font-semibold text-slate-500 py-4">Date</TableHead>
              <TableHead className="font-semibold text-slate-500 py-4">Account</TableHead>
              <TableHead className="font-semibold text-slate-500 py-4">Category</TableHead>
              <TableHead className="font-semibold text-slate-500 text-right py-4">Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell><Skeleton className="h-4 w-4" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-48" /></TableCell>
                  <TableCell className="text-right"><Skeleton className="h-4 w-16 ml-auto" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                  <TableCell><Skeleton className="h-8 w-24 rounded-lg" /></TableCell>
                  <TableCell><Skeleton className="h-8 w-16 ml-auto rounded-lg" /></TableCell>
                </TableRow>
              ))
            ) : filteredTransactions.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="h-64 p-0">
                  <EmptyState 
                    icon={SearchX} 
                    title={hasActiveFilters ? "No matching transactions" : "No transactions found"} 
                    description={hasActiveFilters ? "Try adjusting your filters to find what you're looking for." : "You haven't added any transactions yet. Get started by creating your first transaction."} 
                    action={
                      hasActiveFilters ? (
                        <Button onClick={clearFilters} variant="outline" className="rounded-xl shadow-sm">
                          Clear Filters
                        </Button>
                      ) : (
                        <Button onClick={() => setIsAddOpen(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl px-5 h-11 shadow-md shadow-indigo-500/20 transition-all active:scale-95 flex items-center gap-2">
                          <Plus className="h-4 w-4" /> Add Transaction
                        </Button>
                      )
                    }
                  />
                </TableCell>
              </TableRow>
            ) : (
              
              <>
                {rowVirtualizer.getVirtualItems().length > 0 && (
                  <TableRow style={{ height: `${rowVirtualizer.getVirtualItems()[0]?.start || 0}px` }} className="hover:bg-transparent pointer-events-none border-none">
                    <TableCell colSpan={7} className="p-0 border-none" />
                  </TableRow>
                )}
                {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                  const txn = filteredTransactions[virtualRow.index];
                  return (
                  <TransactionTableRow
                    key={virtualRow.key}
                    txn={txn}
                    index={virtualRow.index}
                    measureRef={rowVirtualizer.measureElement}
                    isSelected={selectedIds.includes(txn.id)}
                    accounts={accounts}
                    categories={categories}
                    quickEditTxnId={quickEditTxnId}
                    onQuickEditTxnIdChange={setQuickEditTxnId}
                    onSelectRow={handleSelectRow}
                    onRowClick={handleRowClick}
                    onDelete={handleDelete}
                    onCreateRule={handleCreateRule}
                    onReview={handleReview}
                    isDeleting={deleteMutation.isPending}
                  />
                );
              })}
              {rowVirtualizer.getVirtualItems().length > 0 && (
                <TableRow style={{ height: `${rowVirtualizer.getTotalSize() - (rowVirtualizer.getVirtualItems()[rowVirtualizer.getVirtualItems().length - 1]?.end || 0)}px` }} className="hover:bg-transparent pointer-events-none border-none">
                  <TableCell colSpan={7} className="p-0 border-none" />
                </TableRow>
              )}
            </>
            )}
          </TableBody>
        </Table>
        
        {/* Pagination Controls */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 bg-slate-50 dark:bg-slate-900/50">
          <p className="text-sm text-slate-500">
            {transactions?.length === 50 ? "Showing 50 transactions" : `Showing ${transactions?.length || 0} transactions`}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handlePrevPage}
              disabled={cursorStack.length === 0 || isLoading}
              className="text-slate-600 dark:text-slate-300"
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleNextPage}
              disabled={!transactions || transactions.length < 100 || isLoading}
              className="text-slate-600 dark:text-slate-300"
            >
              Next
            </Button>
          </div>
        </div>
      </div>

      <EditTransactionDialog
        isOpen={isEditOpen}
        onOpenChange={setIsEditOpen}
        selectedTxn={selectedTxn}
        onSubmit={handleUpdateSubmit}
        isPending={updateMutation.isPending}
        accounts={accounts}
        categories={categories}
        subscriptions={subscriptions}
        paysFor={paysFor}
        setPaysFor={setPaysFor}
        paidBy={paidBy}
        setPaidBy={setPaidBy}
        transactions={transactions}
        onCreateRule={handleCreateRule}
      />

      {selectedIds.length > 0 && (
        <motion.div
          initial={{ opacity: 0, y: 50 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 50 }}
          className="fixed bottom-8 left-1/2 -translate-x-1/2 z-50 flex items-center gap-4 bg-slate-900/90 dark:bg-slate-800/90 backdrop-blur-md px-6 py-4 rounded-2xl shadow-2xl border border-slate-700/50"
        >
          <span className="text-white font-medium">
            {selectedIds.length} selected
          </span>
          <div className="h-6 w-px bg-slate-700 mx-2" />
          <Button 
            variant="ghost" 
            className="text-white hover:bg-slate-800 hover:text-white"
            onClick={() => setIsBulkCategoryOpen(true)}
          >
            <Edit2 className="w-4 h-4 mr-2" />
            Set Category
          </Button>
          <Button 
            variant="destructive" 
            className="bg-rose-500/20 text-rose-300 hover:bg-rose-500 hover:text-white"
            onClick={() => setIsBulkDeleteOpen(true)}
          >
            <Trash2 className="w-4 h-4 mr-2" />
            Delete
          </Button>
        </motion.div>
      )}

      {/* Bulk Category Modal */}
      <Dialog open={isBulkCategoryOpen} onOpenChange={setIsBulkCategoryOpen}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>Update Category</DialogTitle>
            <DialogDescription>
              Select a category for {selectedIds.length} transactions.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <Label>Category</Label>
            <Select value={bulkCategoryId} onValueChange={setBulkCategoryId}>
              <SelectTrigger>
                <SelectValue placeholder="Select category" />
              </SelectTrigger>
              <SelectContent>
                {categories?.map((cat) => (
                  <SelectItem key={cat.id} value={cat.id}>
                    {cat.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex justify-end gap-3 mt-4">
            <Button variant="outline" onClick={() => setIsBulkCategoryOpen(false)}>
              Cancel
            </Button>
            <Button 
              className="bg-indigo-600 hover:bg-indigo-700 text-white"
              onClick={() => bulkCategoryMutation.mutate({ ids: selectedIds, catId: bulkCategoryId })}
              disabled={bulkCategoryMutation.isPending || !bulkCategoryId}
            >
              {bulkCategoryMutation.isPending ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : "Save"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Bulk Delete Modal */}
      <Dialog open={isBulkDeleteOpen} onOpenChange={setIsBulkDeleteOpen}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle className="text-rose-600">Delete Transactions</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete {selectedIds.length} transactions? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-3 mt-4">
            <Button variant="outline" onClick={() => setIsBulkDeleteOpen(false)}>
              Cancel
            </Button>
            <Button 
              variant="destructive"
              onClick={() => bulkDeleteMutation.mutate(selectedIds)}
              disabled={bulkDeleteMutation.isPending}
            >
              {bulkDeleteMutation.isPending ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : "Delete"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

    </motion.div>
  );
}
