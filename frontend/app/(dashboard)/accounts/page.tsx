"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, Account } from "@/lib/api";
import { formatCurrency } from "@/lib/utils";
import { motion } from "framer-motion";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Wallet, CreditCard, Plus, ReceiptText, ArrowUpRight } from "lucide-react";
import { toast } from "sonner";

export default function AccountsPage() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [selectedAccount, setSelectedAccount] = useState<Account | null>(null);
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const { data: accounts, isLoading } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, token),
  });

  const createAccountMutation = useMutation({
    mutationFn: (newAccount: { name: string; type: string; initial_balance: number }) =>
      apiFetch<Account>("/accounts", {
        method: "POST",
        body: JSON.stringify(newAccount),
      }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      setIsDialogOpen(false);
      toast.success("Account created successfully");
    },
    onError: (error: any) => {
      toast.error(error.message || "Failed to create account");
    },
  });

  const updateAccountMutation = useMutation({
    mutationFn: (data: { id: string; name: string; type: string; initial_balance: number }) =>
      apiFetch<Account>(`/accounts/${data.id}`, {
        method: "PUT",
        body: JSON.stringify({ name: data.name, type: data.type, initial_balance: data.initial_balance }),
      }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      setSelectedAccount(null);
      toast.success("Account updated successfully");
    },
    onError: (error: any) => {
      toast.error(error.message || "Failed to update account");
    },
  });

  const deleteAccountMutation = useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/accounts/${id}`, {
        method: "DELETE",
      }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      setSelectedAccount(null);
      toast.success("Account deleted successfully");
    },
    onError: (error: any) => {
      toast.error(error.message || "Failed to delete account");
    },
  });

  const handleAddAccount = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const name = formData.get("name") as string;
    const type = formData.get("type") as string;
    const balanceStr = formData.get("initial_balance") as string;
    const initial_balance = Math.round(parseFloat(balanceStr) * 100);

    createAccountMutation.mutate({ name, type, initial_balance });
  };

  const handleEditAccount = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!selectedAccount) return;
    const formData = new FormData(e.currentTarget);
    const name = formData.get("name") as string;
    const type = formData.get("type") as string;
    const balanceStr = formData.get("initial_balance") as string;
    const initial_balance = Math.round(parseFloat(balanceStr) * 100);

    updateAccountMutation.mutate({ id: selectedAccount.id, name, type, initial_balance });
  };

  const assets = accounts?.filter((a) => a.type === "asset") || [];
  const liabilities = accounts?.filter((a) => a.type === "liability") || [];
  const expenses = accounts?.filter((a) => a.type === "expense") || [];
  const incomes = accounts?.filter((a) => a.type === "income") || [];

  const totalAssets = assets.reduce((sum, a) => sum + (a.current_balance ?? a.initial_balance), 0);
  const totalLiabilities = liabilities.reduce((sum, a) => sum + (a.current_balance ?? a.initial_balance), 0);
  const totalExpenses = expenses.reduce((sum, a) => sum + (a.current_balance ?? a.initial_balance), 0);
  const totalIncomes = incomes.reduce((sum, a) => sum + (a.current_balance ?? a.initial_balance), 0);

const AccountCard = ({ account, idx, onClick }: { account: Account; idx: number; onClick: () => void }) => {
  let colorClass = "bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 dark:bg-slate-800 dark:text-slate-400";
  let Icon = Wallet;
  if (account.type === 'asset') {
    colorClass = "bg-emerald-100 text-emerald-600 dark:bg-emerald-500/20 dark:text-emerald-400";
    Icon = Wallet;
  } else if (account.type === 'liability') {
    colorClass = "bg-rose-100 text-rose-600 dark:bg-rose-500/20 dark:text-rose-400";
    Icon = CreditCard;
  } else if (account.type === 'expense') {
    colorClass = "bg-orange-100 text-orange-600 dark:bg-orange-500/20 dark:text-orange-400";
    Icon = ReceiptText;
  } else if (account.type === 'income') {
    colorClass = "bg-blue-100 text-blue-600 dark:bg-blue-500/20 dark:text-blue-400";
    Icon = ArrowUpRight;
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: idx * 0.05 }}
      onClick={onClick}
      className="bg-white dark:bg-slate-900/80 dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm hover:shadow-xl hover:-translate-y-1 transition-all duration-300 border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 group flex flex-col justify-between cursor-pointer"
    >
      <div className="flex justify-between items-start mb-4">
        <div className={`p-3 rounded-2xl ${colorClass}`}>
          <Icon className="h-6 w-6" />
        </div>
        <span className="text-xs font-semibold text-slate-500 uppercase tracking-wider bg-slate-100 dark:bg-slate-800 px-3 py-1 rounded-full">{account.type}</span>
      </div>
      <div>
        <h3 className="text-lg font-medium mb-2 text-slate-600 dark:text-slate-400 group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">{account.name}</h3>
        <p className={`text-3xl font-bold font-heading tracking-tight text-slate-900 dark:text-white`}>
          {formatCurrency(account.current_balance ?? account.initial_balance)}
        </p>
      </div>
    </motion.div>
  );
};

  if (isLoading) return <div className="flex h-[50vh] items-center justify-center text-slate-500">Loading accounts...</div>;

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.5 }} className="pb-10">
      <div className="flex flex-col md:flex-row md:items-end justify-between mb-10 gap-4">
        <div>
          <h1 className="text-4xl font-bold font-heading text-slate-900 dark:text-white mb-2">Accounts</h1>
          <p className="text-slate-500 dark:text-slate-400">Manage your assets and liabilities.</p>
        </div>
        
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-full px-6 shadow-md shadow-indigo-500/20 flex items-center gap-2">
              <Plus className="h-4 w-4" /> Add Account
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[425px] rounded-3xl border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 backdrop-blur-xl bg-white dark:bg-slate-900/90 dark:bg-slate-900/90 shadow-2xl">
            <DialogHeader>
              <DialogTitle className="text-2xl font-bold font-heading text-slate-900 dark:text-white">New Account</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleAddAccount} className="space-y-6 mt-4">
              <div className="space-y-2">
                <Label htmlFor="name" className="text-slate-700 dark:text-slate-300">Account Name</Label>
                <Input id="name" name="name" placeholder="e.g. Chase Checking" required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="type" className="text-slate-700 dark:text-slate-300">Account Type</Label>
                <Select name="type" required defaultValue="asset">
                  <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700">
                    <SelectValue placeholder="Select type" />
                  </SelectTrigger>
                  <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                    <SelectItem value="asset">Asset (Bank, Cash, Investment)</SelectItem>
                    <SelectItem value="liability">Liability (Credit Card, Loan)</SelectItem>
                    <SelectItem value="expense">Expense (Rent, Groceries)</SelectItem>
                    <SelectItem value="income">Income (Salary, Bonus)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="initial_balance" className="text-slate-700 dark:text-slate-300">Initial Balance ($)</Label>
                <Input id="initial_balance" name="initial_balance" type="number" step="0.01" placeholder="0.00" required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
              </div>
              <div className="pt-4">
                <Button type="submit" disabled={createAccountMutation.isPending} className="w-full bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl py-6 text-lg font-medium shadow-md shadow-indigo-500/20">
                  {createAccountMutation.isPending ? "Creating..." : "Create Account"}
                </Button>
              </div>
            </form>
          </DialogContent>
        </Dialog>

        <Dialog open={!!selectedAccount} onOpenChange={(open) => !open && setSelectedAccount(null)}>
          <DialogContent className="sm:max-w-[425px] rounded-3xl border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 backdrop-blur-xl bg-white dark:bg-slate-900/90 dark:bg-slate-900/90 shadow-2xl">
            <DialogHeader>
              <DialogTitle className="text-2xl font-bold font-heading text-slate-900 dark:text-white">Edit Account</DialogTitle>
            </DialogHeader>
            {selectedAccount && (
              <form onSubmit={handleEditAccount} className="space-y-6 mt-4">
                <div className="space-y-2">
                  <Label htmlFor="edit-name" className="text-slate-700 dark:text-slate-300">Account Name</Label>
                  <Input id="edit-name" name="name" defaultValue={selectedAccount.name} required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-type" className="text-slate-700 dark:text-slate-300">Account Type</Label>
                  <Select name="type" required defaultValue={selectedAccount.type}>
                    <SelectTrigger className="rounded-xl border-slate-200 dark:border-slate-700">
                      <SelectValue placeholder="Select type" />
                    </SelectTrigger>
                    <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
                      <SelectItem value="asset">Asset (Bank, Cash, Investment)</SelectItem>
                      <SelectItem value="liability">Liability (Credit Card, Loan)</SelectItem>
                      <SelectItem value="expense">Expense (Rent, Groceries)</SelectItem>
                      <SelectItem value="income">Income (Salary, Bonus)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-initial_balance" className="text-slate-700 dark:text-slate-300">Initial Balance ($)</Label>
                  <Input id="edit-initial_balance" name="initial_balance" type="number" step="0.01" defaultValue={(selectedAccount.initial_balance / 100).toFixed(2)} required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
                </div>
                <div className="pt-4 flex gap-3">
                  <Button type="button" variant="destructive" onClick={() => deleteAccountMutation.mutate(selectedAccount.id)} disabled={deleteAccountMutation.isPending || updateAccountMutation.isPending} className="rounded-xl py-6 font-medium">
                    {deleteAccountMutation.isPending ? "..." : "Delete"}
                  </Button>
                  <Button type="submit" disabled={updateAccountMutation.isPending || deleteAccountMutation.isPending} className="flex-1 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl py-6 font-medium shadow-md shadow-indigo-500/20">
                    {updateAccountMutation.isPending ? "Updating..." : "Save Changes"}
                  </Button>
                </div>
              </form>
            )}
          </DialogContent>
        </Dialog>
      </div>

      <div className="space-y-12">
        <section>
          <div className="flex items-center justify-between mb-6 border-b border-slate-200 dark:border-slate-800 pb-4">
            <h2 className="text-2xl font-bold text-slate-800 dark:text-slate-200 font-heading flex items-center gap-3">
              <div className="p-2 bg-emerald-100 dark:bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 rounded-lg">
                <Wallet className="h-5 w-5" />
              </div>
              Assets
            </h2>
            <span className="text-xl font-bold text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-900/20 px-4 py-1 rounded-full">
              {formatCurrency(totalAssets)}
            </span>
          </div>
          {assets.length === 0 ? (
            <div className="text-center py-12 bg-slate-50 dark:bg-slate-800/30 rounded-3xl border border-dashed border-slate-200 dark:border-slate-700">
              <p className="text-slate-500">No assets found. Add an account to get started.</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {assets.map((account, idx) => (
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} />
              ))}
            </div>
          )}
        </section>

        <section>
          <div className="flex items-center justify-between mb-6 border-b border-slate-200 dark:border-slate-800 pb-4">
            <h2 className="text-2xl font-bold text-slate-800 dark:text-slate-200 font-heading flex items-center gap-3">
              <div className="p-2 bg-rose-100 dark:bg-rose-500/20 text-rose-600 dark:text-rose-400 rounded-lg">
                <CreditCard className="h-5 w-5" />
              </div>
              Liabilities
            </h2>
            <span className="text-xl font-bold text-rose-600 dark:text-rose-400 bg-rose-50 dark:bg-rose-900/20 px-4 py-1 rounded-full">
              {formatCurrency(totalLiabilities)}
            </span>
          </div>
          {liabilities.length === 0 ? (
            <div className="text-center py-12 bg-slate-50 dark:bg-slate-800/30 rounded-3xl border border-dashed border-slate-200 dark:border-slate-700">
              <p className="text-slate-500">No liabilities found.</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {liabilities.map((account, idx) => (
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} />
              ))}
            </div>
          )}
        </section>

        <section>
          <div className="flex items-center justify-between mb-6 border-b border-slate-200 dark:border-slate-800 pb-4">
            <h2 className="text-2xl font-bold text-slate-800 dark:text-slate-200 font-heading flex items-center gap-3">
              <div className="p-2 bg-blue-100 dark:bg-blue-500/20 text-blue-600 dark:text-blue-400 rounded-lg">
                <ArrowUpRight className="h-5 w-5" />
              </div>
              Income
            </h2>
            <span className="text-xl font-bold text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20 px-4 py-1 rounded-full">
              {formatCurrency(totalIncomes)}
            </span>
          </div>
          {incomes.length === 0 ? (
            <div className="text-center py-12 bg-slate-50 dark:bg-slate-800/30 rounded-3xl border border-dashed border-slate-200 dark:border-slate-700">
              <p className="text-slate-500">No income accounts found.</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {incomes.map((account, idx) => (
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} />
              ))}
            </div>
          )}
        </section>

        <section>
          <div className="flex items-center justify-between mb-6 border-b border-slate-200 dark:border-slate-800 pb-4">
            <h2 className="text-2xl font-bold text-slate-800 dark:text-slate-200 font-heading flex items-center gap-3">
              <div className="p-2 bg-orange-100 dark:bg-orange-500/20 text-orange-600 dark:text-orange-400 rounded-lg">
                <ReceiptText className="h-5 w-5" />
              </div>
              Expenses
            </h2>
            <span className="text-xl font-bold text-orange-600 dark:text-orange-400 bg-orange-50 dark:bg-orange-900/20 px-4 py-1 rounded-full">
              {formatCurrency(totalExpenses)}
            </span>
          </div>
          {expenses.length === 0 ? (
            <div className="text-center py-12 bg-slate-50 dark:bg-slate-800/30 rounded-3xl border border-dashed border-slate-200 dark:border-slate-700">
              <p className="text-slate-500">No expense accounts found.</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {expenses.map((account, idx) => (
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} />
              ))}
            </div>
          )}
        </section>

      </div>
    </motion.div>
  );
}
