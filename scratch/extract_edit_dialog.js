const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// 1. Extract TransactionAllocationList
const allocationListTarget = /function TransactionAllocationList\(\{[\s\S]*?\}\) \{[\s\S]*?return \([\s\S]*?<\/div>\n  \);\n\}\n/;
const allocationMatch = code.match(allocationListTarget);
let allocationListCode = '';
if (allocationMatch) {
  allocationListCode = allocationMatch[0];
  code = code.replace(allocationListTarget, '');
}

// 2. Extract EditTransactionDialog
const editDialogTarget = /<Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>[\s\S]*?<\/form>\s*<\/div>\s*\)\}\s*<\/DialogContent>\s*<\/Dialog>/;

const editDialogReplacement = `<EditTransactionDialog
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
      />`;

code = code.replace(editDialogTarget, editDialogReplacement);

// 3. Add Import
const importTarget = `const AddTransactionDialog = dynamic(() => import('./AddTransactionDialog'), { ssr: false });`;
const importReplacement = `const AddTransactionDialog = dynamic(() => import('./AddTransactionDialog'), { ssr: false });\nconst EditTransactionDialog = dynamic(() => import('./EditTransactionDialog'), { ssr: false });`;

if (!code.includes('import EditTransactionDialog')) {
    code = code.replace(importTarget, importReplacement);
}

fs.writeFileSync(path, code);

// 4. Create EditTransactionDialog.tsx
const editDialogCode = `import { useState } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Loader2, Link, Check, ChevronsUpDown } from "lucide-react";
import { Account, Category, Subscription, Transaction } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/utils";

${allocationListCode}

interface EditTransactionDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  selectedTxn: Transaction | null;
  onSubmit: (e: React.FormEvent<HTMLFormElement>) => void;
  isPending: boolean;
  accounts: Account[] | undefined;
  categories: Category[] | undefined;
  subscriptions: Subscription[] | undefined;
  paysFor: {transaction_id: string, amount: number}[];
  setPaysFor: (val: {transaction_id: string, amount: number}[]) => void;
  paidBy: {transaction_id: string, amount: number}[];
  setPaidBy: (val: {transaction_id: string, amount: number}[]) => void;
  transactions: Transaction[] | undefined;
}

export default function EditTransactionDialog({
  isOpen,
  onOpenChange,
  selectedTxn,
  onSubmit,
  isPending,
  accounts,
  categories,
  subscriptions,
  paysFor,
  setPaysFor,
  paidBy,
  setPaidBy,
  transactions,
}: EditTransactionDialogProps) {
  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-[95vw] w-full h-[95vh] sm:max-w-5xl rounded-3xl border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 bg-white dark:bg-slate-900/95 dark:bg-slate-900/95 backdrop-blur-xl flex flex-col p-8 overflow-hidden">
          <DialogHeader className="mb-6 shrink-0">
            <DialogTitle className="text-3xl font-bold font-heading text-slate-900 dark:text-white">Edit Transaction</DialogTitle>
            <DialogDescription className="text-lg">Update the details of this transaction.</DialogDescription>
          </DialogHeader>
          {selectedTxn && (
            <div className="flex-1 overflow-y-auto pr-4 custom-scrollbar">
              <form onSubmit={onSubmit} className="space-y-6">
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
                  <div className="space-y-4 md:col-span-2 mt-4 pt-4 border-t border-slate-200 dark:border-slate-800">
                    <h3 className="text-lg font-semibold text-slate-800 dark:text-slate-200 mb-4 flex items-center gap-2">
                      <Link className="w-5 h-5 text-indigo-500" />
                      Transaction Links
                    </h3>
                    
                    <div className="space-y-2">
                      <Label className="text-slate-700 dark:text-slate-300 font-medium">(Partially) Pays for (Expenses this covered)</Label>
                      <TransactionAllocationList allocations={paysFor} setAllocations={setPaysFor} transactions={transactions} parentAmount={selectedTxn?.amount} />
                    </div>

                    <div className="space-y-2 mt-6">
                      <Label className="text-slate-700 dark:text-slate-300 font-medium">(Partially) Paid by (Revenues that covered this)</Label>
                      <TransactionAllocationList allocations={paidBy} setAllocations={setPaidBy} transactions={transactions} parentAmount={selectedTxn?.amount} />
                    </div>
                  </div>
                </div>
                <div className="pt-8 flex gap-4 shrink-0">
                  <Button type="button" variant="outline" onClick={() => onOpenChange(false)} className="flex-1 rounded-xl py-8 text-xl border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 dark:bg-slate-800 dark:hover:bg-slate-800">
                    Cancel
                  </Button>
                  <Button 
                    type="submit" 
                    disabled={isPending || (selectedTxn ? paysFor.reduce((s, a) => s + a.amount, 0) > Math.abs(selectedTxn.amount) || paidBy.reduce((s, a) => s + a.amount, 0) > Math.abs(selectedTxn.amount) : false)} 
                    className="flex-1 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl py-8 text-xl shadow-md shadow-indigo-500/20 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isPending ? <Loader2 className="h-6 w-6 animate-spin mx-auto" /> : "Save Changes"}
                  </Button>
                </div>
              </form>
            </div>
          )}
        </DialogContent>
      </Dialog>
  );
}
`;

fs.writeFileSync('frontend/app/(dashboard)/transactions/EditTransactionDialog.tsx', editDialogCode);
console.log("Extracted EditTransactionDialog!");
