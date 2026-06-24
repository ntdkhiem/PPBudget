"use client";

import { useState, useMemo, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, Transaction, Category, Account, Subscription } from "@/lib/api";
import { formatCurrency, formatDate, cn } from "@/lib/utils";
import { format, parseISO } from "date-fns";
import { useDateRange } from "@/app/contexts/DateRangeContext";
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
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
import { toast } from "sonner";
import { Plus, Trash2, Loader2, Edit2, CheckCircle2, SearchX, Inbox, ExternalLink, Check, ChevronsUpDown, Repeat } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export default function TransactionsPage() {
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

  const [isLinkedTxnOpen, setIsLinkedTxnOpen] = useState(false);
  const [linkedTxnId, setLinkedTxnId] = useState<string | null>(null);

  useEffect(() => {
    if (selectedTxn) {
      setLinkedTxnId(selectedTxn.linked_transaction_id || null);
    }
  }, [selectedTxn]);

  useEffect(() => {
    const editId = searchParams?.get("edit_id");
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

  const truncateText = (text: string, maxLength: number = 100) => {
    if (!text) return "";
    const cleaned = text.replace(/\s+/g, ' ').trim();
    return cleaned.length > maxLength ? cleaned.substring(0, maxLength) + "..." : cleaned;
  };

  const { date } = useDateRange();

  useEffect(() => {
    setCursorStack([]);
  }, [date]);

  const currentCursor = cursorStack[cursorStack.length - 1];

  const queryParams = useMemo(() => {
    const params = new URLSearchParams();
    if (date?.from) params.append("start_date", format(date.from, "yyyy-MM-dd"));
    if (date?.to) params.append("end_date", format(date.to, "yyyy-MM-dd"));
    if (currentCursor) {
      params.append("cursor_date", currentCursor.date);
      params.append("cursor_id", currentCursor.id);
    }
    return params.toString();
  }, [date, currentCursor]);

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

  const handleSelectRow = (id: string, checked: boolean) => {
    if (checked) {
      setSelectedIds(prev => [...prev, id]);
    } else {
      setSelectedIds(prev => prev.filter(x => x !== id));
    }
  };

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

  const handleDelete = (id: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (confirm("Are you sure you want to delete this transaction?")) {
      deleteMutation.mutate(id);
    }
  };

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
      linked_transaction_id: formData.get("linkedTransactionId") || null,
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
      linked_transaction_id: formData.get("linkedTransactionId") || null,
    });
  };

  const handleRowClick = (txn: Transaction) => {
    setSelectedTxn(txn);
    setIsEditOpen(true);
  };

  const handleNextPage = () => {
    if (transactions && transactions.length === 50) {
      const last = transactions[transactions.length - 1];
      setCursorStack([...cursorStack, { date: last.date, id: last.id }]);
    }
  };

  const handlePrevPage = () => {
    if (cursorStack.length > 0) {
      setCursorStack(cursorStack.slice(0, -1));
    }
  };

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.5 }} className="space-y-6 pb-10">
      <div className="flex flex-col md:flex-row md:items-end justify-between mb-8 gap-4">
        <div>
          <h1 className="text-4xl font-bold font-heading text-slate-900 dark:text-white mb-2">Transactions</h1>
          <p className="text-slate-500 dark:text-slate-400">View and manage your transactions.</p>
        </div>
        
        <div className="flex gap-3">
          <Button 
            variant="outline"
            asChild
            className="rounded-full shadow-md text-amber-600 border-amber-200 hover:bg-amber-50 flex items-center gap-2 px-6 bg-white dark:bg-slate-900"
          >
            <a href="/review">
              <Inbox className="h-4 w-4" /> 
              Needs Review {unreviewedTransactions?.length ? `(${unreviewedTransactions.length})` : ''}
            </a>
          </Button>

          <Dialog open={isAddOpen} onOpenChange={setIsAddOpen}>
            <DialogTrigger asChild>
              <Button className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-full px-6 shadow-md shadow-indigo-500/20 flex items-center gap-2">
                <Plus className="h-4 w-4" /> Add Transaction
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-[425px] rounded-3xl border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 backdrop-blur-xl bg-white dark:bg-slate-900/90 dark:bg-slate-900/90 shadow-2xl">
            <DialogHeader>
              <DialogTitle className="text-2xl font-bold font-heading text-slate-900 dark:text-white">Add Transaction</DialogTitle>
              <DialogDescription>Create a new manual transaction.</DialogDescription>
            </DialogHeader>
            <form onSubmit={handleCreateSubmit} className="space-y-4 mt-4">
              <div className="space-y-2">
                <Label htmlFor="accountId" className="text-slate-700 dark:text-slate-300">Account</Label>
                <Select name="accountId" required>
                  <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700">
                    <SelectValue placeholder="Select an account" />
                  </SelectTrigger>
                  <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                    {accounts?.map((acc) => (
                      <SelectItem key={acc.id} value={acc.id}>{acc.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="date" className="text-slate-700 dark:text-slate-300">Date</Label>
                <Input id="date" name="date" type="date" required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="description" className="text-slate-700 dark:text-slate-300">Description</Label>
                <Input id="description" name="description" required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="amount" className="text-slate-700 dark:text-slate-300">Amount ($)</Label>
                <Input id="amount" name="amount" type="number" step="0.01" required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="notes" className="text-slate-700 dark:text-slate-300">Notes (Optional)</Label>
                <Input id="notes" name="notes" className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="categoryId" className="text-slate-700 dark:text-slate-300">Category</Label>
                <Select name="categoryId">
                  <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700">
                    <SelectValue placeholder="Select a category" />
                  </SelectTrigger>
                  <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                    {categories?.map((cat) => (
                      <SelectItem key={cat.id} value={cat.id}>{cat.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="subscriptionId" className="text-slate-700 dark:text-slate-300">Subscription</Label>
                <Select name="subscriptionId" defaultValue="none">
                  <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700">
                    <SelectValue placeholder="Select a subscription" />
                  </SelectTrigger>
                  <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                    <SelectItem value="none">None</SelectItem>
                    {subscriptions?.map((sub) => (
                      <SelectItem key={sub.id} value={sub.id}>{sub.name} ({formatCurrency(sub.amount)})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="pt-4">
                <Button type="submit" disabled={createMutation.isPending} className="w-full bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl py-6 text-lg font-medium shadow-md shadow-indigo-500/20">
                  {createMutation.isPending ? <Loader2 className="h-5 w-5 animate-spin mx-auto" /> : "Save"}
                </Button>
              </div>
            </form>
          </DialogContent>
        </Dialog>
        </div>
      </div>

      <div className="rounded-3xl border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 bg-white dark:bg-slate-900/80 dark:bg-slate-900/80 backdrop-blur-xl shadow-sm overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60">
              <TableHead className="w-12 py-4 text-center">
                <input 
                  type="checkbox" 
                  className="w-4 h-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer"
                  checked={transactions?.length ? selectedIds.length === transactions.length : false}
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
            ) : transactions?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="h-64 p-0">
                  <EmptyState 
                    icon={SearchX} 
                    title="No transactions found" 
                    description="You haven't added any transactions yet. Get started by creating your first transaction." 
                    action={
                      <Button onClick={() => setIsAddOpen(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl shadow-md">
                        <Plus className="mr-2 h-4 w-4" /> Add Transaction
                      </Button>
                    }
                  />
                </TableCell>
              </TableRow>
            ) : (
              transactions?.map((txn) => (
                <TableRow 
                  key={txn.id} 
                  className={`group cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 dark:bg-slate-800/50/50 dark:hover:bg-slate-800/50 transition-colors border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 ${selectedIds.includes(txn.id) ? 'bg-indigo-50 dark:bg-indigo-900/30/30 dark:bg-indigo-900/10' : ''}`}
                  onClick={() => handleRowClick(txn)}
                >
                  <TableCell className="w-12 py-4 text-center" onClick={(e) => e.stopPropagation()}>
                    <input 
                      type="checkbox" 
                      className="w-4 h-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600 cursor-pointer"
                      checked={selectedIds.includes(txn.id)}
                      onChange={(e) => handleSelectRow(txn.id, e.target.checked)}
                    />
                  </TableCell>
                  <TableCell className="py-4 font-medium text-slate-900 dark:text-slate-100 max-w-xs" title={txn.description}>
                    <div className="line-clamp-3 whitespace-pre-wrap break-words">{truncateText(txn.description)}</div>
                    <div className="mt-2 flex flex-wrap gap-1.5 items-center">
                      {!txn.is_reviewed && (
                        <span className="inline-flex items-center rounded-full bg-amber-100/80 dark:bg-amber-500/20 border border-amber-200 dark:border-amber-500/30 px-2.5 py-0.5 text-xs font-semibold text-amber-700 dark:text-amber-400">
                          Needs Review
                        </span>
                      )}
                      {txn.subscription_id && (
                        <span className="inline-flex items-center gap-1 rounded-full bg-indigo-100/80 dark:bg-indigo-500/20 border border-indigo-200 dark:border-indigo-500/30 px-2.5 py-0.5 text-xs font-semibold text-indigo-700 dark:text-indigo-400" title="Subscription Payment">
                          <Repeat size={12} /> Subscription
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-right py-4 font-semibold">
                    <div className="flex flex-col items-end gap-0.5">
                      {txn.linked_by && txn.linked_by.length > 0 ? (
                        <>
                          <span className={txn.effective_amount! < 0 ? "text-rose-600 dark:text-rose-400" : "text-emerald-600 dark:text-emerald-400"}>
                            {formatCurrency(txn.effective_amount!)}
                          </span>
                          <span className="text-xs font-medium text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/30 px-1.5 py-0.5 rounded" title={`Original: ${formatCurrency(txn.amount)}. Paid by ${txn.linked_by.length} transaction(s).`}>
                            Remaining
                          </span>
                        </>
                      ) : (
                        <>
                          <span className={cn(
                            txn.amount < 0 ? "text-rose-600 dark:text-rose-400" : "text-emerald-600 dark:text-emerald-400",
                            txn.linked_transaction_id ? "line-through opacity-50 text-sm" : ""
                          )}>
                            {formatCurrency(txn.amount)}
                          </span>
                          {txn.linked_transaction_id && (
                            <span className="text-xs font-medium text-slate-500 bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded" title={`Pays for ID: ${txn.linked_transaction_id}`}>
                              Effective: $0.00
                            </span>
                          )}
                        </>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="py-4 text-slate-600 dark:text-slate-300 whitespace-nowrap">
                    {formatDate(txn.date)}
                  </TableCell>
                  <TableCell className="py-4 text-slate-600 dark:text-slate-300 max-w-[150px]" title={accounts?.find(a => a.id === txn.account_id)?.name || "Unknown"}>
                    <div className="line-clamp-3 whitespace-pre-wrap break-words">{truncateText(accounts?.find(a => a.id === txn.account_id)?.name || "Unknown")}</div>
                  </TableCell>
                  <TableCell className="py-4" onClick={(e) => e.stopPropagation()}>
                    <Popover open={quickEditTxnId === txn.id} onOpenChange={(open) => setQuickEditTxnId(open ? txn.id : null)}>
                      <PopoverTrigger asChild>
                        <Button variant="ghost" className={`h-8 px-3 rounded-lg text-sm font-medium ${txn.category_id ? 'text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700' : 'text-indigo-600 bg-indigo-50 dark:bg-indigo-900/30 hover:bg-indigo-100 dark:text-indigo-400 dark:bg-indigo-500/10 dark:hover:bg-indigo-500/20'}`}>
                          {categories?.find((c) => c.id === txn.category_id)?.name || "Uncategorized"}
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className="w-56 p-2 rounded-xl" align="start">
                        <div className="space-y-1">
                          <h4 className="font-medium text-sm px-2 py-1.5 text-slate-500">Quick Edit Category</h4>
                          <div className="max-h-60 overflow-y-auto">
                            {categories?.map((cat) => (
                              <div
                                key={cat.id}
                                className={`px-2 py-1.5 text-sm rounded-md cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 dark:bg-slate-800 dark:hover:bg-slate-800 flex items-center justify-between ${txn.category_id === cat.id ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400' : ''}`}
                                onClick={() => reviewMutation.mutate({ id: txn.id, categoryId: cat.id })}
                              >
                                {cat.name}
                                {txn.category_id === cat.id && <CheckCircle2 className="h-4 w-4" />}
                              </div>
                            ))}
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </TableCell>
                  <TableCell className="text-right py-4" onClick={(e) => e.stopPropagation()}>
                    <div className="flex justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Edit transaction"
                        className="text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 dark:bg-indigo-900/30 dark:hover:text-indigo-400 dark:hover:bg-indigo-500/10 h-8 w-8 rounded-lg"
                        onClick={(e) => { e.stopPropagation(); handleRowClick(txn); }}
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Delete transaction"
                        className="text-slate-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:text-rose-400 dark:hover:bg-rose-500/10 h-8 w-8 rounded-lg"
                        onClick={(e) => handleDelete(txn.id, e)}
                        disabled={deleteMutation.isPending}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
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
              disabled={!transactions || transactions.length < 50 || isLoading}
              className="text-slate-600 dark:text-slate-300"
            >
              Next
            </Button>
          </div>
        </div>
      </div>

      <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
        <DialogContent className="max-w-[95vw] w-full h-[95vh] sm:max-w-5xl rounded-3xl border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 bg-white dark:bg-slate-900/95 dark:bg-slate-900/95 backdrop-blur-xl flex flex-col p-8 overflow-hidden">
          <DialogHeader className="mb-6 shrink-0">
            <DialogTitle className="text-3xl font-bold font-heading text-slate-900 dark:text-white">Edit Transaction</DialogTitle>
            <DialogDescription className="text-lg">Update the details of this transaction.</DialogDescription>
          </DialogHeader>
          {selectedTxn && (
            <div className="flex-1 overflow-y-auto pr-4 custom-scrollbar">
              <form onSubmit={handleUpdateSubmit} className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div className="space-y-2">
                    <Label htmlFor="editAccountId" className="text-slate-700 dark:text-slate-300 text-lg">Account</Label>
                    <Select name="accountId" required defaultValue={selectedTxn.account_id}>
                      <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700 h-14 text-lg">
                        <SelectValue placeholder="Select an account" />
                      </SelectTrigger>
                      <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                        {accounts?.map((acc) => (
                          <SelectItem key={acc.id} value={acc.id}>{acc.name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="editDate" className="text-slate-700 dark:text-slate-300 text-lg">Date</Label>
                    <Input id="editDate" name="date" type="date" required defaultValue={selectedTxn.date.split("T")[0]} className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500 h-14 text-lg" />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="editDescription" className="text-slate-700 dark:text-slate-300 text-lg">Description</Label>
                    <Input id="editDescription" name="description" required defaultValue={selectedTxn.description} className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500 h-14 text-lg" />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="editAmount" className="text-slate-700 dark:text-slate-300 text-lg">Amount ($)</Label>
                    <Input id="editAmount" name="amount" type="number" step="0.01" required defaultValue={(selectedTxn.amount / 100).toFixed(2)} className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500 h-14 text-lg" />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="editNotes" className="text-slate-700 dark:text-slate-300 text-lg">Notes (Optional)</Label>
                    <Input id="editNotes" name="notes" defaultValue={selectedTxn.notes || ""} className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500 h-14 text-lg" />
                  </div>
                  <div className="space-y-2 md:col-span-1">
                    <Label htmlFor="editCategoryId" className="text-slate-700 dark:text-slate-300 text-lg">Category</Label>
                    <Select name="categoryId" defaultValue={selectedTxn.category_id || undefined}>
                      <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700 h-14 text-lg">
                        <SelectValue placeholder="Select a category" />
                      </SelectTrigger>
                      <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                        {categories?.map((cat) => (
                          <SelectItem key={cat.id} value={cat.id}>{cat.name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2 md:col-span-1">
                    <Label htmlFor="editSubscriptionId" className="text-slate-700 dark:text-slate-300 text-lg">Subscription</Label>
                    <Select name="subscriptionId" defaultValue={selectedTxn.subscription_id || "none"}>
                      <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700 h-14 text-lg">
                        <SelectValue placeholder="Select a subscription" />
                      </SelectTrigger>
                      <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                        <SelectItem value="none">None</SelectItem>
                        {subscriptions?.map((sub) => (
                          <SelectItem key={sub.id} value={sub.id}>{sub.name} ({formatCurrency(sub.amount)})</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2 md:col-span-2 flex flex-col">
                    <Label className="text-slate-700 dark:text-slate-300 text-lg">Pays for (Link to Transaction)</Label>
                    <input type="hidden" name="linkedTransactionId" value={linkedTxnId || ""} />
                    <Popover open={isLinkedTxnOpen} onOpenChange={setIsLinkedTxnOpen}>
                      <PopoverTrigger asChild>
                        <Button
                          variant="outline"
                          role="combobox"
                          aria-expanded={isLinkedTxnOpen}
                          className="justify-between rounded-xl border-slate-200 dark:border-slate-700 h-14 text-lg font-normal bg-white dark:bg-slate-900 overflow-hidden"
                        >
                          <span className="truncate">
                            {linkedTxnId
                              ? transactions?.find((t) => t.id === linkedTxnId)?.description || `ID: ${linkedTxnId}`
                              : "Select recent transaction..."}
                          </span>
                          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className="w-[500px] p-0 rounded-xl max-w-[90vw]" align="start">
                        <Command>
                          <CommandInput placeholder="Search recent transactions..." />
                          <CommandList className="max-h-[300px]">
                            <CommandEmpty>No recent transaction found.</CommandEmpty>
                            <CommandGroup>
                              {transactions?.slice(0, 50).map((txn) => (
                                <CommandItem
                                  key={txn.id}
                                  value={`${txn.description} ${txn.amount} ${txn.date} ${txn.id}`}
                                  onSelect={() => {
                                    setLinkedTxnId(txn.id === linkedTxnId ? null : txn.id);
                                    setIsLinkedTxnOpen(false);
                                  }}
                                >
                                  <Check
                                    className={cn(
                                      "mr-2 h-4 w-4 shrink-0",
                                      linkedTxnId === txn.id ? "opacity-100" : "opacity-0"
                                    )}
                                  />
                                  <div className="flex w-full justify-between items-center pr-2 gap-2 overflow-hidden">
                                    <div className="flex flex-col overflow-hidden">
                                      <span className="font-medium text-base truncate">{txn.description.replace(/\s+/g, ' ').trim()}</span>
                                      <span className="text-xs text-muted-foreground">{formatDate(txn.date)}</span>
                                    </div>
                                    <span className="font-semibold text-right whitespace-nowrap">{formatCurrency(txn.amount)}</span>
                                  </div>
                                </CommandItem>
                              ))}
                            </CommandGroup>
                          </CommandList>
                        </Command>
                      </PopoverContent>
                    </Popover>
                  </div>
                </div>
                <div className="pt-8 flex gap-4 shrink-0">
                  <Button type="button" variant="outline" onClick={() => setIsEditOpen(false)} className="flex-1 rounded-xl py-8 text-xl border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 dark:bg-slate-800 dark:hover:bg-slate-800">
                    Cancel
                  </Button>
                  <Button type="submit" disabled={updateMutation.isPending} className="flex-1 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl py-8 text-xl shadow-md shadow-indigo-500/20">
                    {updateMutation.isPending ? <Loader2 className="h-6 w-6 animate-spin mx-auto" /> : "Save Changes"}
                  </Button>
                </div>
              </form>
            </div>
          )}
        </DialogContent>
      </Dialog>

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
