"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { useState, useMemo, useCallback } from "react";
import dynamic from "next/dynamic";
import { apiFetch, Transaction, Category, Account, Subscription, BalanceSnapshot } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/utils";
import { Plus, Search, Calendar, Filter, X, Trash2, History, SearchX, PiggyBank } from "lucide-react";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DashboardCard } from "@/components/dashboard-card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { TransactionTableRow } from "@/components/transaction-table-row";
import { toast } from "sonner";

const EditTransactionDialog = dynamic(() => import("../../transactions/EditTransactionDialog"), { ssr: false });

type Allocation = { transaction_id: string; amount: number; description?: string; date?: string };

const fieldClass =
  "border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white rounded dark:[color-scheme:dark] [&>option]:bg-white [&>option]:text-slate-900 dark:[&>option]:bg-slate-800 dark:[&>option]:text-white";

const COLUMN_COUNT = 6;

export default function AccountDetailPage() {
  const params = useParams();
  const router = useRouter();
  const accountId = params.id as string;
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const [search, setSearch] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [categoryId, setCategoryId] = useState("");

  const [selectedTxn, setSelectedTxn] = useState<Transaction | null>(null);
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [quickEditTxnId, setQuickEditTxnId] = useState<string | null>(null);
  const [txnToDelete, setTxnToDelete] = useState<string | null>(null);
  const [paysFor, setPaysFor] = useState<Allocation[]>([]);
  const [paidBy, setPaidBy] = useState<Allocation[]>([]);

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [snapshotToDelete, setSnapshotToDelete] = useState<BalanceSnapshot | null>(null);

  const { data: transactions, isLoading } = useQuery<Transaction[]>({
    queryKey: ["transactions", accountId],
    queryFn: () => apiFetch<Transaction[]>(`/transactions?account_id=${accountId}`, {}, token),
    enabled: !!accountId,
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const { data: accounts } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, token),
    enabled: !!token,
  });

  const { data: subscriptions } = useQuery<Subscription[]>({
    queryKey: ["subscriptions"],
    queryFn: () => apiFetch<Subscription[]>("/subscriptions", {}, token),
    enabled: !!token,
  });

  const { data: balanceSnapshots, isLoading: loadingBalances } = useQuery<BalanceSnapshot[]>({
    queryKey: ["accounts", accountId, "balances"],
    queryFn: () => apiFetch<BalanceSnapshot[]>(`/accounts/${accountId}/balances`, {}, token),
    enabled: !!accountId,
  });

  const invalidateBalances = () => {
    queryClient.invalidateQueries({ queryKey: ["transactions"] });
    queryClient.invalidateQueries({ queryKey: ["accounts"] });
    queryClient.invalidateQueries({ queryKey: ["reports"] });
  };

  const deleteSnapshotMutation = useMutation({
    mutationFn: (snapshotId: string) =>
      apiFetch(`/accounts/${accountId}/balances/${snapshotId}`, { method: "DELETE" }, token),
    onSuccess: () => {
      invalidateBalances();
      setSnapshotToDelete(null);
      toast.success("Balance snapshot deleted");
    },
  });

  const updateMutation = useMutation({
    mutationFn: (data: Record<string, unknown>) =>
      apiFetch(`/transactions/${data.id}`, { method: "PUT", body: JSON.stringify(data) }, token),
    onSuccess: () => {
      toast.success("Transaction updated successfully");
      setIsEditOpen(false);
      invalidateBalances();
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiFetch(`/transactions/${id}`, { method: "DELETE" }, token),
    onSuccess: () => {
      toast.success("Transaction deleted successfully");
      setTxnToDelete(null);
      invalidateBalances();
    },
  });

  const reviewMutation = useMutation({
    mutationFn: ({ id, categoryId }: { id: string; categoryId: string }) =>
      apiFetch(`/transactions/${id}/review`, { method: "PATCH", body: JSON.stringify({ category_id: categoryId }) }, token),
    onSuccess: () => {
      toast.success("Transaction categorized successfully");
      setQuickEditTxnId(null);
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
      queryClient.invalidateQueries({ queryKey: ["unreviewed"] });
    },
  });

  const handleCreateRule = useCallback((txn: Transaction) => {
    const params = new URLSearchParams({ new: "1", description: txn.description });
    if (txn.account_id) params.set("account", txn.account_id);
    if (txn.category_id) params.set("category", txn.category_id);
    router.push(`/settings/rules?${params.toString()}`);
  }, [router]);

  const handleRowClick = useCallback((txn: Transaction) => {
    setSelectedTxn(txn);
    setPaidBy(txn.paid_by || []);
    setPaysFor(txn.pays_for || []);
    setIsEditOpen(true);
  }, []);

  const handleDelete = useCallback((id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setTxnToDelete(id);
  }, []);

  const { mutate: reviewTransaction } = reviewMutation;
  const handleReview = useCallback(
    (id: string, categoryId: string) => reviewTransaction({ id, categoryId }),
    [reviewTransaction]
  );

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
      pays_for: paysFor,
      paid_by: paidBy,
    });
  };

  const filteredTxns = useMemo(() => {
    if (!transactions) return [];
    return transactions.filter(t => {
      const matchSearch = t.description.toLowerCase().includes(search.toLowerCase());
      const matchFrom = fromDate ? new Date(t.date) >= new Date(fromDate) : true;
      const matchTo = toDate ? new Date(t.date) <= new Date(toDate) : true;
      const matchCat = categoryId ? t.category_id === categoryId : true;
      return matchSearch && matchFrom && matchTo && matchCat;
    });
  }, [transactions, search, fromDate, toDate, categoryId]);

  const hasActiveFilters = !!(search || fromDate || toDate || categoryId);
  const clearFilters = () => {
    setSearch("");
    setFromDate("");
    setToDate("");
    setCategoryId("");
  };

  const accountsLoaded = accounts !== undefined;
  const account = accounts?.find(a => a.id === accountId);
  const accountName = account?.name;
  // Unknown while accounts haven't loaded yet — treat as "not balance-only" for loading purposes,
  // but don't render the real transactions UI until we know for sure (avoids a flash).
  const isBalanceOnly = accountsLoaded && !!account?.balance_only;
  const showTransactionsUI = !accountsLoaded || !isBalanceOnly;
  const showTransactionsSkeleton = isLoading || !accountsLoaded;

  return (
    <div className="pb-24 relative min-h-screen">
      <h1 className="text-3xl font-bold mb-6 text-slate-900 dark:text-white">
        {accountName ? `${accountName} History` : "Account History"}
      </h1>

      {showTransactionsUI ? (
        <>
          {/* Filtering Bar */}
          <div className="bg-white dark:bg-slate-900 p-4 rounded-lg shadow mb-6 flex flex-wrap gap-4 items-center">
            <div className="flex items-center bg-gray-100 dark:bg-slate-800 rounded px-3 py-2 flex-1 min-w-[200px]">
              <Search size={18} className="text-slate-500 dark:text-slate-400 mr-2" />
              <input
                type="text"
                placeholder="Search description..."
                className="bg-transparent outline-none w-full text-sm text-slate-900 dark:text-white"
                value={search}
                onChange={e => setSearch(e.target.value)}
              />
            </div>
            <div className="flex items-center gap-2">
              <Calendar size={18} className="text-slate-500 dark:text-slate-400" />
              <input type="date" value={fromDate} onChange={e => setFromDate(e.target.value)} className={`${fieldClass} px-2 py-1 text-sm`} />
              <span className="text-slate-500 dark:text-slate-400">-</span>
              <input type="date" value={toDate} onChange={e => setToDate(e.target.value)} className={`${fieldClass} px-2 py-1 text-sm`} />
            </div>
            <div className="flex items-center gap-2 border border-slate-200 dark:border-slate-700 rounded px-3 py-1 bg-white dark:bg-slate-900">
              <Filter size={18} className="text-slate-500 dark:text-slate-400" />
              <select value={categoryId} onChange={e => setCategoryId(e.target.value)} className="outline-none text-sm bg-white dark:bg-slate-900 text-slate-900 dark:text-white dark:[color-scheme:dark] [&>option]:bg-white [&>option]:text-slate-900 dark:[&>option]:bg-slate-900 dark:[&>option]:text-white">
                <option value="">All Categories</option>
                {categories?.map(c => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            </div>
          </div>

          {/* Transactions Table (same row component as the Transactions page) */}
          <div className="overflow-auto relative rounded-3xl border border-slate-200 dark:border-slate-800/60 bg-white dark:bg-slate-900/80 backdrop-blur-xl shadow-sm">
            <Table className="relative w-full">
              <TableHeader>
                <TableRow className="hover:bg-transparent border-slate-200 dark:border-slate-800/60">
                  <TableHead className="font-semibold text-slate-500 dark:text-slate-400 py-4">Description</TableHead>
                  <TableHead className="font-semibold text-slate-500 dark:text-slate-400 text-right py-4">Amount</TableHead>
                  <TableHead className="font-semibold text-slate-500 dark:text-slate-400 text-right py-4">Balance</TableHead>
                  <TableHead className="font-semibold text-slate-500 dark:text-slate-400 py-4">Date</TableHead>
                  <TableHead className="font-semibold text-slate-500 dark:text-slate-400 py-4">Category</TableHead>
                  <TableHead className="font-semibold text-slate-500 dark:text-slate-400 text-right py-4">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {showTransactionsSkeleton ? (
                  Array.from({ length: 5 }).map((_, i) => (
                    <TableRow key={i}>
                      <TableCell><Skeleton className="h-4 w-48" /></TableCell>
                      <TableCell className="text-right"><Skeleton className="h-4 w-16 ml-auto" /></TableCell>
                      <TableCell className="text-right"><Skeleton className="h-4 w-16 ml-auto" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                      <TableCell><Skeleton className="h-8 w-24 rounded-lg" /></TableCell>
                      <TableCell><Skeleton className="h-8 w-16 ml-auto rounded-lg" /></TableCell>
                    </TableRow>
                  ))
                ) : filteredTxns.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={COLUMN_COUNT} className="h-64 p-0">
                      <EmptyState
                        icon={SearchX}
                        title={hasActiveFilters ? "No matching transactions" : "No transactions found"}
                        description={hasActiveFilters ? "Try adjusting your filters to find what you're looking for." : "This account has no transactions yet."}
                        action={
                          hasActiveFilters ? (
                            <button onClick={clearFilters} className="rounded-xl border border-slate-200 dark:border-slate-700 px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800">
                              Clear Filters
                            </button>
                          ) : undefined
                        }
                      />
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredTxns.map(txn => (
                    <TransactionTableRow
                      key={txn.id}
                      txn={txn}
                      accounts={accounts}
                      categories={categories}
                      quickEditTxnId={quickEditTxnId}
                      onQuickEditTxnIdChange={setQuickEditTxnId}
                      onRowClick={handleRowClick}
                      onDelete={handleDelete}
                      onCreateRule={handleCreateRule}
                      onReview={handleReview}
                      isDeleting={deleteMutation.isPending}
                      showAccount={false}
                      showRunningBalance
                    />
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </>
      ) : (
        <DashboardCard className="flex items-center gap-4">
          <div className="p-3 rounded-2xl bg-indigo-100 dark:bg-indigo-500/20 text-indigo-600 dark:text-indigo-400 shrink-0">
            <PiggyBank className="h-6 w-6" />
          </div>
          <p className="text-sm text-slate-600 dark:text-slate-300">
            This account tracks balance only. Its balance comes from SimpleFin syncs and manual updates, so transactions aren&apos;t shown.
          </p>
        </DashboardCard>
      )}

      {/* Balance History */}
      <DashboardCard className="mt-6">
        <div className="flex items-center gap-2 mb-4 text-slate-800 dark:text-slate-200">
          <History className="h-5 w-5 text-indigo-600 dark:text-indigo-400" />
          <h2 className="text-lg font-bold font-heading">Balance History</h2>
        </div>
        {loadingBalances ? (
          <p className="text-sm text-slate-500 dark:text-slate-400 py-4">Loading balance history...</p>
        ) : !balanceSnapshots || balanceSnapshots.length === 0 ? (
          <p className="text-sm text-slate-500 dark:text-slate-400 py-4">No balance snapshots yet.</p>
        ) : (
          <div className="divide-y divide-slate-200 dark:divide-slate-800">
            {balanceSnapshots.map((snap) => (
              <div key={snap.id} className="flex items-center justify-between py-3 gap-4">
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-slate-900 dark:text-white">
                    {snap.as_of_date ? formatDate(snap.as_of_date) : "Opening balance"}
                  </p>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    {snap.source === "simplefin" ? "SimpleFin sync" : snap.source === "manual" ? "Manual update" : "Before first transaction"}
                  </p>
                </div>
                <span className="font-bold font-heading text-slate-900 dark:text-white shrink-0">
                  {formatCurrency(snap.balance)}
                </span>
                {snap.source !== "opening" && (
                  <button
                    type="button"
                    onClick={() => setSnapshotToDelete(snap)}
                    className="shrink-0 text-slate-500 dark:text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors"
                    aria-label="Delete balance snapshot"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </DashboardCard>

      {showTransactionsUI && (
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
        />
      )}

      <ConfirmDialog
        open={!!txnToDelete}
        onOpenChange={(open) => !open && setTxnToDelete(null)}
        title="Delete Transaction"
        description="Are you sure you want to delete this transaction? This cannot be undone."
        confirmText="Delete"
        isDestructive
        isLoading={deleteMutation.isPending}
        onConfirm={() => txnToDelete && deleteMutation.mutate(txnToDelete)}
      />

      <ConfirmDialog
        open={!!snapshotToDelete}
        onOpenChange={(open) => !open && setSnapshotToDelete(null)}
        title="Delete Balance Snapshot"
        description={`Are you sure you want to delete this balance snapshot${snapshotToDelete ? ` from ${snapshotToDelete.as_of_date ? formatDate(snapshotToDelete.as_of_date) : "opening"}` : ""}? This cannot be undone.`}
        confirmText="Delete"
        isDestructive
        isLoading={deleteSnapshotMutation.isPending}
        onConfirm={() => snapshotToDelete && deleteSnapshotMutation.mutate(snapshotToDelete.id)}
      />

      {accountsLoaded && !isBalanceOnly && (
        <>
          {/* FAB */}
          <button
            onClick={() => setIsModalOpen(true)}
            className="fixed bottom-8 right-8 bg-blue-600 text-white p-4 rounded-full shadow-lg hover:bg-blue-700 transition-colors z-10"
          >
            <Plus size={24} />
          </button>

          {/* Manual Entry Modal */}
          {isModalOpen && <ManualEntryModal accountId={accountId} onClose={() => setIsModalOpen(false)} token={token} />}
        </>
      )}
    </div>
  );
}

function ManualEntryModal({ accountId, onClose, token }: { accountId: string, onClose: () => void, token: string }) {
  const queryClient = useQueryClient();
  const [desc, setDesc] = useState("");
  const [amt, setAmt] = useState("");
  const [date, setDate] = useState("");

  const createMutation = useMutation({
    mutationFn: () => apiFetch("/transactions", {
      method: "POST",
      body: JSON.stringify({ account_id: accountId, description: desc, amount: Math.round(parseFloat(amt) * 100), date }),
    }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
      onClose();
    },
  });

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-slate-900 p-6 rounded-lg shadow-xl w-96 relative">
        <button onClick={onClose} className="absolute top-4 right-4 text-gray-400 hover:text-slate-900 dark:hover:text-white"><X size={20}/></button>
        <h2 className="text-xl font-bold mb-4 text-slate-900 dark:text-white">Add Transaction</h2>
        <div className="flex flex-col gap-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Description</label>
            <input type="text" value={desc} onChange={e => setDesc(e.target.value)} className={`${fieldClass} w-full p-2`} />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Amount ($)</label>
            <input type="number" step="0.01" value={amt} onChange={e => setAmt(e.target.value)} className={`${fieldClass} w-full p-2`} />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Date</label>
            <input type="date" value={date} onChange={e => setDate(e.target.value)} className={`${fieldClass} w-full p-2`} />
          </div>
          <button
            onClick={() => createMutation.mutate()}
            disabled={createMutation.isPending}
            className="w-full bg-blue-600 text-white p-2 rounded hover:bg-blue-700 mt-2 disabled:opacity-50"
          >
            Create
          </button>
        </div>
      </div>
    </div>
  );
}
