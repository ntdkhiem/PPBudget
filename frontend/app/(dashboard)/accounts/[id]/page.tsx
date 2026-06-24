"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useState, useMemo } from "react";
import { apiFetch, Transaction, Category } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/utils";
import { Plus, Search, Calendar, Filter, X } from "lucide-react";

export default function AccountDetailPage() {
  const params = useParams();
  const accountId = params.id as string;
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const [search, setSearch] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [categoryId, setCategoryId] = useState("");
  
  const [editingTxn, setEditingTxn] = useState<string | null>(null);
  const [editDesc, setEditDesc] = useState("");
  const [editCatId, setEditCatId] = useState("");

  const [isModalOpen, setIsModalOpen] = useState(false);

  const { data: transactions, isLoading } = useQuery<Transaction[]>({
    queryKey: ["transactions", accountId],
    queryFn: () => apiFetch<Transaction[]>(`/transactions?account_id=${accountId}`, {}, token),
    enabled: !!accountId,
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const updateMutation = useMutation({
    mutationFn: (data: { id: string, description: string, category_id: string }) => 
      apiFetch(`/transactions/${data.id}`, {
        method: "PATCH",
        body: JSON.stringify({ description: data.description, category_id: data.category_id || null }),
      }, token),
    onMutate: async (newData) => {
      await queryClient.cancelQueries({ queryKey: ["transactions", accountId] });
      const previousTxns = queryClient.getQueryData<Transaction[]>(["transactions", accountId]);
      queryClient.setQueryData<Transaction[]>(["transactions", accountId], (old) => 
        old?.map(t => t.id === newData.id ? { ...t, description: newData.description, category_id: newData.category_id } : t) || []
      );
      return { previousTxns };
    },
    onError: (err, newData, context) => {
      if (context?.previousTxns) {
        queryClient.setQueryData(["transactions", accountId], context.previousTxns);
      }
      alert("Failed to update transaction.");
    },
    onSuccess: () => setEditingTxn(null),
  });

  const startEdit = (txn: Transaction) => {
    setEditingTxn(txn.id);
    setEditDesc(txn.description);
    setEditCatId(txn.category_id || "");
  };

  const saveEdit = (id: string) => {
    updateMutation.mutate({ id, description: editDesc, category_id: editCatId });
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

  if (isLoading) return <div>Loading account history...</div>;

  return (
    <div className="pb-24 relative min-h-screen">
      <h1 className="text-3xl font-bold mb-6">Account History</h1>
      
      {/* Filtering Bar */}
      <div className="bg-white dark:bg-slate-900 p-4 rounded-lg shadow mb-6 flex flex-wrap gap-4 items-center">
        <div className="flex items-center bg-gray-100 rounded px-3 py-2 flex-1 min-w-[200px]">
          <Search size={18} className="text-gray-500 mr-2" />
          <input 
            type="text" 
            placeholder="Search description..." 
            className="bg-transparent outline-none w-full text-sm"
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
        </div>
        <div className="flex items-center gap-2">
          <Calendar size={18} className="text-gray-500" />
          <input type="date" value={fromDate} onChange={e => setFromDate(e.target.value)} className="border rounded px-2 py-1 text-sm" />
          <span className="text-gray-500">-</span>
          <input type="date" value={toDate} onChange={e => setToDate(e.target.value)} className="border rounded px-2 py-1 text-sm" />
        </div>
        <div className="flex items-center gap-2 border rounded px-3 py-1 bg-white dark:bg-slate-900">
          <Filter size={18} className="text-gray-500" />
          <select value={categoryId} onChange={e => setCategoryId(e.target.value)} className="outline-none text-sm bg-transparent">
            <option value="">All Categories</option>
            {categories?.map(c => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Transactions Table */}
      <div className="bg-white dark:bg-slate-900 rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Date</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Description</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Category</th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Amount</th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Balance</th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-slate-900 divide-y divide-gray-200">
            {filteredTxns.length === 0 && (
              <tr>
                <td colSpan={5} className="px-6 py-12 text-center text-gray-500">No transactions found.</td>
              </tr>
            )}
            {filteredTxns.map((txn) => {
              const isEditing = editingTxn === txn.id;
              return (
                <tr key={txn.id} className="hover:bg-gray-50 cursor-pointer" onClick={() => !isEditing && startEdit(txn)}>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    {formatDate(txn.date)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    {isEditing ? (
                      <input 
                        type="text" 
                        value={editDesc} 
                        onChange={e => setEditDesc(e.target.value)} 
                        className="border rounded px-2 py-1 w-full"
                        autoFocus
                      />
                    ) : (
                      txn.description
                    )}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    {isEditing ? (
                      <select 
                        value={editCatId} 
                        onChange={e => setEditCatId(e.target.value)}
                        className="border rounded px-2 py-1 w-full"
                      >
                        <option value="">Uncategorized</option>
                        {categories?.map(c => (
                          <option key={c.id} value={c.id}>{c.name}</option>
                        ))}
                      </select>
                    ) : (
                      <span className="bg-gray-100 text-gray-700 px-2 py-1 rounded text-xs">
                        {categories?.find(c => c.id === txn.category_id)?.name || "Uncategorized"}
                      </span>
                    )}
                  </td>
                  <td className={`px-6 py-4 whitespace-nowrap text-sm text-right font-medium ${txn.amount < 0 ? "text-red-600" : "text-green-600"}`}>
                    {formatCurrency(txn.amount)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-right font-bold text-gray-900">
                    {formatCurrency(txn.running_balance || 0)}
                  </td>
                  {isEditing && (
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-right">
                      <div className="flex gap-2 justify-end">
                        <button onClick={(e) => { e.stopPropagation(); saveEdit(txn.id); }} className="text-green-600 font-bold hover:underline">Save</button>
                        <button onClick={(e) => { e.stopPropagation(); setEditingTxn(null); }} className="text-gray-500 font-bold hover:underline">Cancel</button>
                      </div>
                    </td>
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* FAB */}
      <button 
        onClick={() => setIsModalOpen(true)}
        className="fixed bottom-8 right-8 bg-blue-600 text-white p-4 rounded-full shadow-lg hover:bg-blue-700 transition-colors z-10"
      >
        <Plus size={24} />
      </button>

      {/* Manual Entry Modal */}
      {isModalOpen && <ManualEntryModal accountId={accountId} onClose={() => setIsModalOpen(false)} token={token} />}
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
      queryClient.invalidateQueries({ queryKey: ["transactions", accountId] });
      onClose();
    },
  });

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-slate-900 p-6 rounded-lg shadow-xl w-96 relative">
        <button onClick={onClose} className="absolute top-4 right-4 text-gray-400 hover:text-gray-800"><X size={20}/></button>
        <h2 className="text-xl font-bold mb-4">Add Transaction</h2>
        <div className="flex flex-col gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
            <input type="text" value={desc} onChange={e => setDesc(e.target.value)} className="w-full border rounded p-2" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Amount ($)</label>
            <input type="number" step="0.01" value={amt} onChange={e => setAmt(e.target.value)} className="w-full border rounded p-2" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Date</label>
            <input type="date" value={date} onChange={e => setDate(e.target.value)} className="w-full border rounded p-2" />
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
