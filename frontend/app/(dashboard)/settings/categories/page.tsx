"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { apiFetch, Category } from "@/lib/api";
import { Plus, Edit2, Check, X, Trash2, Tag, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { motion, AnimatePresence } from "framer-motion";

export default function CategoriesPage() {
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const [newCatName, setNewCatName] = useState("");
  const [newCatType, setNewCatType] = useState<"income" | "expense" | "transfer">("expense");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editType, setEditType] = useState<"income" | "expense" | "transfer">("expense");

  // Delete Dialog state
  const [catToDelete, setCatToDelete] = useState<Category | null>(null);

  const { data: categories, isLoading } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, token),
  });

  const createMutation = useMutation({
    mutationFn: (data: { name: string; type: string }) =>
      apiFetch(
        "/categories",
        {
          method: "POST",
          body: JSON.stringify(data),
        },
        token
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["categories"] });
      setNewCatName("");
      toast.success("Category created successfully");
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to create category");
    },
  });

  const updateMutation = useMutation({
    mutationFn: (data: { id: string; name: string; type: string }) =>
      apiFetch(
        `/categories/${data.id}`,
        {
          method: "PUT",
          body: JSON.stringify({ name: data.name, type: data.type }),
        },
        token
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["categories"] });
      setEditingId(null);
      toast.success("Category updated successfully");
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to update category");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      apiFetch(
        `/categories/${id}`,
        {
          method: "DELETE",
        },
        token
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["categories"] });
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      setCatToDelete(null);
      toast.success("Category deleted successfully");
    },
    onError: (err: any) => {
      toast.error(err.message || "Failed to delete category");
    },
  });

  if (isLoading) return <div className="p-8 text-center text-slate-500">Loading categories...</div>;

  return (
    <div className="max-w-4xl mx-auto space-y-8 pb-12">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-4xl font-extrabold tracking-tight text-slate-900 dark:text-white mb-2">Categories</h1>
          <p className="text-slate-500 text-lg">Organize your spending into custom groups.</p>
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-2xl border border-slate-200/60 dark:border-slate-800 shadow-sm mb-8 flex gap-4 items-center">
        <div className="p-3 bg-indigo-100 dark:bg-indigo-500/20 text-indigo-600 dark:text-indigo-400 rounded-xl hidden sm:block">
          <Tag className="h-5 w-5" />
        </div>
        <input
          type="text"
          placeholder="New Category Name"
          value={newCatName}
          onChange={(e) => setNewCatName(e.target.value)}
          className="flex-1 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
        <select
          value={newCatType}
          onChange={(e) => setNewCatType(e.target.value as any)}
          className="bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 focus:outline-none focus:ring-2 focus:ring-indigo-500"
        >
          <option value="expense">Expense</option>
          <option value="income">Income</option>
          <option value="transfer">Transfer</option>
        </select>
        <Button
          onClick={() => newCatName && createMutation.mutate({ name: newCatName, type: newCatType })}
          disabled={createMutation.isPending || !newCatName}
          className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl px-6 py-6"
        >
          <Plus size={20} className="mr-2" /> Add
        </Button>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl rounded-2xl border border-slate-200/60 dark:border-slate-800 shadow-sm overflow-hidden">
        <div className="grid grid-cols-1 divide-y divide-slate-100 dark:divide-slate-800">
          <AnimatePresence>
            {categories?.map((cat) => (
              <motion.div
                key={cat.id}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="p-5 flex items-center justify-between hover:bg-slate-50/50 dark:hover:bg-slate-800/40 transition-colors group"
              >
                {editingId === cat.id ? (
                  <div className="flex items-center gap-4 flex-1">
                    <input
                      type="text"
                      value={editName}
                      onChange={(e) => setEditName(e.target.value)}
                      className="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 flex-1 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                      autoFocus
                    />
                    <select
                      value={editType}
                      onChange={(e) => setEditType(e.target.value as any)}
                      className="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                    >
                      <option value="expense">Expense</option>
                      <option value="income">Income</option>
                      <option value="transfer">Transfer</option>
                    </select>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => updateMutation.mutate({ id: cat.id, name: editName, type: editType })}
                      className="text-emerald-600 hover:text-emerald-700 hover:bg-emerald-50 dark:hover:bg-emerald-500/10"
                    >
                      <Check size={18} />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => setEditingId(null)}
                      className="text-slate-400 hover:text-slate-600 hover:bg-slate-100 dark:hover:bg-slate-800"
                    >
                      <X size={18} />
                    </Button>
                  </div>
                ) : (
                  <>
                    <div className="flex items-center gap-4">
                      <span className="text-slate-900 dark:text-slate-100 font-semibold text-lg">{cat.name}</span>
                      <span
                        className={`text-[10px] font-bold px-2 py-1 rounded-md uppercase tracking-wide ${
                          cat.type === "income"
                            ? "bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400"
                            : cat.type === "transfer"
                            ? "bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-400"
                            : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400"
                        }`}
                      >
                        {cat.type}
                      </span>
                      {cat.transaction_count !== undefined && cat.transaction_count > 0 && (
                        <span className="text-xs text-slate-500 font-medium">
                          {cat.transaction_count} transaction{cat.transaction_count !== 1 ? "s" : ""}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Button
                        size="icon"
                        variant="ghost"
                        onClick={() => {
                          setEditingId(cat.id);
                          setEditName(cat.name);
                          setEditType(cat.type || "expense");
                        }}
                        className="text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 dark:hover:bg-indigo-500/10"
                      >
                        <Edit2 size={16} />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        onClick={() => setCatToDelete(cat)}
                        className="text-slate-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-500/10"
                      >
                        <Trash2 size={16} />
                      </Button>
                    </div>
                  </>
                )}
              </motion.div>
            ))}
          </AnimatePresence>
          {categories?.length === 0 && <div className="p-12 text-center text-slate-500">No categories found.</div>}
        </div>
      </div>

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!catToDelete} onOpenChange={(open) => !open && setCatToDelete(null)}>
        <DialogContent className="sm:max-w-[425px] rounded-3xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-rose-600">
              <AlertTriangle className="h-5 w-5" /> Delete Category
            </DialogTitle>
            <DialogDescription className="pt-4 text-base text-slate-600 dark:text-slate-300">
              Are you sure you want to delete the category <strong className="text-slate-900 dark:text-white">"{catToDelete?.name}"</strong>?
            </DialogDescription>
          </DialogHeader>
          
          {catToDelete && (catToDelete.transaction_count ?? 0) > 0 && (
            <div className="bg-amber-50 dark:bg-amber-500/10 border border-amber-200 dark:border-amber-500/20 p-4 rounded-xl mt-2">
              <p className="text-amber-800 dark:text-amber-400 text-sm font-medium">
                Warning: You have <strong className="font-bold">{catToDelete.transaction_count}</strong> transaction{catToDelete.transaction_count !== 1 ? "s" : ""} currently assigned to this category.
              </p>
              <p className="text-amber-700/80 dark:text-amber-500/80 text-xs mt-1">
                If you delete this category, these transactions will become uncategorized.
              </p>
            </div>
          )}

          <DialogFooter className="mt-6">
            <Button variant="outline" onClick={() => setCatToDelete(null)} className="rounded-xl">
              Cancel
            </Button>
            <Button
              variant="destructive"
              className="rounded-xl bg-rose-600 hover:bg-rose-700"
              onClick={() => {
                if (catToDelete) deleteMutation.mutate(catToDelete.id);
              }}
              disabled={deleteMutation.isPending}
            >
              Yes, delete category
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
