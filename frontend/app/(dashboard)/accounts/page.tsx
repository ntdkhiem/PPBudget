"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, Account, ACCOUNT_ROLES, type AccountRole } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/utils";
import { motion } from "framer-motion";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import Link from "next/link";
import { Wallet, CreditCard, Plus, ReceiptText, ArrowUpRight, RefreshCw, History } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/page-header";
import { UpdateBalanceDialog } from "@/components/update-balance-dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

const STALE_DAYS = 3;

function isStale(balanceAsOf: string): boolean {
  const asOf = new Date(balanceAsOf);
  const diffMs = Date.now() - asOf.getTime();
  return diffMs > STALE_DAYS * 24 * 60 * 60 * 1000;
}

type PendingBalanceOnlyEdit = { id: string; name: string; type: string; role: AccountRole | null };

const ROLE_LABEL: Record<string, string> = Object.fromEntries(
  ACCOUNT_ROLES.map((r) => [r.value, r.label]),
);

/**
 * One picker for what an account is. Assets and debts pick a role, which
 * decides the side of the balance sheet and is what the Wealth Strategy plan
 * reads -- checking and savings count as cash, investments as invested. The
 * "not sure yet" options keep an account unclassified, and the plan asks
 * about those rather than guessing.
 */
function KindSelect({ id, defaultValue }: { id: string; defaultValue: string }) {
  return (
    <Select name="kind" required defaultValue={defaultValue}>
      <SelectTrigger id={id} className="rounded-xl border-slate-200 dark:border-slate-700">
        <SelectValue placeholder="Select what it is" />
      </SelectTrigger>
      <SelectContent className="rounded-xl border-slate-200 dark:border-slate-700">
        <SelectGroup>
          <SelectLabel>Assets</SelectLabel>
          {ACCOUNT_ROLES.filter((r) => r.type === "asset").map((r) => (
            <SelectItem key={r.value} value={`role:${r.value}`}>{r.label}</SelectItem>
          ))}
          <SelectItem value="type:asset">Other asset (not sure yet)</SelectItem>
        </SelectGroup>
        <SelectGroup>
          <SelectLabel>Debts</SelectLabel>
          {ACCOUNT_ROLES.filter((r) => r.type === "liability").map((r) => (
            <SelectItem key={r.value} value={`role:${r.value}`}>{r.label}</SelectItem>
          ))}
          <SelectItem value="type:liability">Other debt (not sure yet)</SelectItem>
        </SelectGroup>
        <SelectGroup>
          <SelectLabel>Tracking only</SelectLabel>
          <SelectItem value="type:expense">Expense (Rent, Groceries)</SelectItem>
          <SelectItem value="type:income">Income (Salary, Bonus)</SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>
  );
}

/** The picker's value, split back into the type and role the API takes. */
function parseKind(value: string): { type: string; role: AccountRole | null } {
  const [kind, v] = value.split(":");
  const role = kind === "role" ? ACCOUNT_ROLES.find((r) => r.value === v) : undefined;
  return role ? { type: role.type, role: role.value } : { type: v || "asset", role: null };
}

function kindOf(account: Account): string {
  return account.role ? `role:${account.role}` : `type:${account.type}`;
}

export default function AccountsPage() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [selectedAccount, setSelectedAccount] = useState<Account | null>(null);
  const [balanceAccount, setBalanceAccount] = useState<Account | null>(null);
  const [pendingBalanceOnlyEdit, setPendingBalanceOnlyEdit] = useState<PendingBalanceOnlyEdit | null>(null);
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const { data: accounts, isLoading } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, token),
  });

  const createAccountMutation = useMutation({
    mutationFn: (newAccount: { name: string; type: string; role: AccountRole | null; opening_balance: number }) =>
      apiFetch<Account>("/accounts", {
        method: "POST",
        body: JSON.stringify(newAccount),
      }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      setIsDialogOpen(false);
      toast.success("Account created successfully");
    },
    onError: (error: Error) => {
      toast.error(error.message || "Failed to create account");
    },
  });

  const updateAccountMutation = useMutation({
    mutationFn: async (data: { id: string; name: string; type: string; role: AccountRole | null; balanceOnly: boolean; prevBalanceOnly: boolean }) => {
      // The role is always sent: null is "not sure yet", which clears it.
      await apiFetch<Account>(`/accounts/${data.id}`, {
        method: "PUT",
        body: JSON.stringify({ name: data.name, type: data.type, role: data.role }),
      }, token);

      if (data.balanceOnly === data.prevBalanceOnly) {
        return { balanceOnlyChanged: false as const };
      }

      const res = await apiFetch<{ status: string; deleted_transactions: number }>(
        `/accounts/${data.id}/balance-only`,
        { method: "PUT", body: JSON.stringify({ enabled: data.balanceOnly }) },
        token
      );
      return { balanceOnlyChanged: true as const, enabled: data.balanceOnly, deletedTransactions: res.deleted_transactions };
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
      // Roles decide what the plan counts as cash, debt and investment.
      queryClient.invalidateQueries({ queryKey: ["wealth"] });
      setSelectedAccount(null);
      setPendingBalanceOnlyEdit(null);
      if (result.balanceOnlyChanged) {
        if (result.enabled) {
          toast.success(`Balance-only enabled · ${result.deletedTransactions} transactions deleted`);
        } else {
          toast.success("Balance-only disabled");
        }
      } else {
        toast.success("Account updated successfully");
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || "Failed to update account");
      setPendingBalanceOnlyEdit(null);
      // The name/type update may have succeeded before the balance-only request failed.
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
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
    onError: (error: Error) => {
      toast.error(error.message || "Failed to delete account");
    },
  });

  const handleAddAccount = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const name = formData.get("name") as string;
    const { type, role } = parseKind(formData.get("kind") as string);
    const balanceStr = formData.get("opening_balance") as string;
    const opening_balance = Math.round(parseFloat(balanceStr) * 100);

    createAccountMutation.mutate({ name, type, role, opening_balance });
  };

  const handleEditAccount = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!selectedAccount) return;
    const formData = new FormData(e.currentTarget);
    const name = formData.get("name") as string;
    const { type, role } = parseKind(formData.get("kind") as string);
    const balanceOnly = formData.get("balance_only") === "on";

    if (balanceOnly && !selectedAccount.balance_only) {
      // Turning balance-only on deletes transactions — confirm first.
      setPendingBalanceOnlyEdit({ id: selectedAccount.id, name, type, role });
      return;
    }

    updateAccountMutation.mutate({ id: selectedAccount.id, name, type, role, balanceOnly, prevBalanceOnly: selectedAccount.balance_only });
  };

  const assets = accounts?.filter((a) => a.type === "asset") || [];
  const liabilities = accounts?.filter((a) => a.type === "liability") || [];
  const expenses = accounts?.filter((a) => a.type === "expense") || [];
  const incomes = accounts?.filter((a) => a.type === "income") || [];

  const totalAssets = assets.reduce((sum, a) => sum + a.current_balance, 0);
  const totalLiabilities = liabilities.reduce((sum, a) => sum + a.current_balance, 0);
  const totalExpenses = expenses.reduce((sum, a) => sum + a.current_balance, 0);
  const totalIncomes = incomes.reduce((sum, a) => sum + a.current_balance, 0);

const AccountCard = ({ account, idx, onClick, onUpdateBalance }: { account: Account; idx: number; onClick: () => void; onUpdateBalance: () => void }) => {
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

  const showStale = account.balance_source === "simplefin" && !!account.balance_as_of && isStale(account.balance_as_of);
  // Only assets and debts need a role; the plan leaves an unclassified one out.
  const needsRole = !account.role && (account.type === "asset" || account.type === "liability");

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
        <span className="text-xs font-semibold text-slate-500 uppercase tracking-wider bg-slate-100 dark:bg-slate-800 px-3 py-1 rounded-full">
          {account.role ? ROLE_LABEL[account.role] : account.type}
        </span>
      </div>
      <div>
        <h3 className="text-lg font-medium mb-2 text-slate-600 dark:text-slate-400 group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">{account.name}</h3>
        <p className={`text-3xl font-bold font-heading tracking-tight text-slate-900 dark:text-white`}>
          {formatCurrency(account.current_balance)}
        </p>
        {(account.balance_as_of || account.balance_only || needsRole) && (
          <div className="mt-1 flex items-center gap-2 flex-wrap">
            {needsRole && (
              <Badge
                className="bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400 border-0"
                title="Edit the account to say what it is. Until then the Wealth Strategy plan leaves it out of your cash and your investments."
              >
                No role yet
              </Badge>
            )}
            {account.balance_as_of && (
              <p className="text-xs text-slate-500 dark:text-slate-400">
                as of {formatDate(account.balance_as_of)} &middot; {account.balance_source === "simplefin" ? "SimpleFin" : "Manual"}
              </p>
            )}
            {showStale && (
              <Badge className="bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400 border-0">
                Stale
              </Badge>
            )}
            {account.balance_only && (
              <Badge className="bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400 border-0">
                Balance only
              </Badge>
            )}
          </div>
        )}
        <div className="mt-3 flex items-center gap-4">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); onUpdateBalance(); }}
            className="inline-flex items-center gap-1.5 text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:text-indigo-700 dark:hover:text-indigo-300"
          >
            <RefreshCw className="h-3 w-3" /> Update balance
          </button>
          <Link
            href={`/accounts/${account.id}`}
            onClick={(e) => e.stopPropagation()}
            className="inline-flex items-center gap-1.5 text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:text-indigo-700 dark:hover:text-indigo-300"
          >
            <History className="h-3 w-3" /> View history
          </Link>
        </div>
      </div>
    </motion.div>
  );
};

  if (isLoading) return <div className="flex h-[50vh] items-center justify-center text-slate-500">Loading accounts...</div>;

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.5 }} className="pb-10">
      <PageHeader title="Accounts" description="Manage your assets and liabilities.">
        
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl px-5 h-11 shadow-md shadow-indigo-500/20 flex items-center gap-2 transition-all active:scale-95">
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
                <Label htmlFor="kind" className="text-slate-700 dark:text-slate-300">What is it?</Label>
                <KindSelect id="kind" defaultValue="role:checking" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="opening_balance" className="text-slate-700 dark:text-slate-300">Opening Balance ($)</Label>
                <Input id="opening_balance" name="opening_balance" type="number" step="0.01" placeholder="0.00" required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
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
              <form key={selectedAccount.id} onSubmit={handleEditAccount} className="space-y-6 mt-4">
                <div className="space-y-2">
                  <Label htmlFor="edit-name" className="text-slate-700 dark:text-slate-300">Account Name</Label>
                  <Input id="edit-name" name="name" defaultValue={selectedAccount.name} required className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500" />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-kind" className="text-slate-700 dark:text-slate-300">What is it?</Label>
                  <KindSelect id="edit-kind" defaultValue={kindOf(selectedAccount)} />
                </div>
                <label
                  htmlFor="edit-balance-only"
                  className="flex items-start gap-3 rounded-xl border border-slate-200 dark:border-slate-700 p-3 cursor-pointer hover:border-indigo-200 dark:hover:border-indigo-800/60 transition-colors"
                >
                  <input
                    id="edit-balance-only"
                    name="balance_only"
                    type="checkbox"
                    defaultChecked={selectedAccount.balance_only}
                    className="mt-0.5 h-4 w-4 rounded border-slate-300 dark:border-slate-600 text-indigo-600 focus:ring-indigo-500 bg-white dark:bg-slate-900 cursor-pointer dark:checked:bg-indigo-500"
                  />
                  <div>
                    <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Balance only</span>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Track only this account&apos;s balance. Transactions won&apos;t be imported.
                    </p>
                  </div>
                </label>
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

        <ConfirmDialog
          open={!!pendingBalanceOnlyEdit}
          onOpenChange={(open) => !open && setPendingBalanceOnlyEdit(null)}
          title="Track balance only?"
          description={`All transactions in ${pendingBalanceOnlyEdit?.name ?? "this account"} will be deleted and future syncs won't import its transactions. This can't be undone.`}
          confirmText="Enable"
          isDestructive
          isLoading={updateAccountMutation.isPending}
          onConfirm={() => {
            if (!pendingBalanceOnlyEdit || !selectedAccount) return;
            updateAccountMutation.mutate({
              id: pendingBalanceOnlyEdit.id,
              name: pendingBalanceOnlyEdit.name,
              type: pendingBalanceOnlyEdit.type,
              role: pendingBalanceOnlyEdit.role,
              balanceOnly: true,
              prevBalanceOnly: selectedAccount.balance_only,
            });
          }}
        />
      </PageHeader>

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
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} onUpdateBalance={() => setBalanceAccount(account)} />
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
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} onUpdateBalance={() => setBalanceAccount(account)} />
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
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} onUpdateBalance={() => setBalanceAccount(account)} />
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
                <AccountCard key={account.id} account={account} idx={idx} onClick={() => setSelectedAccount(account)} onUpdateBalance={() => setBalanceAccount(account)} />
              ))}
            </div>
          )}
        </section>

      </div>

      {balanceAccount && (
        <UpdateBalanceDialog
          accountId={balanceAccount.id}
          accountName={balanceAccount.name}
          open={!!balanceAccount}
          onOpenChange={(open) => !open && setBalanceAccount(null)}
        />
      )}
    </motion.div>
  );
}
