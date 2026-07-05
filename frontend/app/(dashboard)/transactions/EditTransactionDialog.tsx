import { useState, useEffect } from "react";
import { useTransactionSearch } from "@/hooks/useTransactionSearch";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Loader2, Link, Check, ChevronsUpDown, X } from "lucide-react";
import { Account, Category, Subscription, Transaction } from "@/lib/api";
import { formatCurrency, formatDate, cn } from "@/lib/utils";

function AllocationAmountInput({ amount, maxAllowed, onChange }: { amount: number, maxAllowed?: number, onChange: (amount: number) => void }) {
  const [inputValue, setInputValue] = useState(amount ? (amount / 100).toString() : '');

  // Keep string state in sync with external amount changes, 
  // but only if mathematically different to avoid cursor jumping while typing decimals
  useEffect(() => {
    const parsed = parseFloat(inputValue || '0');
    if (Math.round(parsed * 100) !== amount) {
      setInputValue(amount ? (amount / 100).toString() : '');
    }
  }, [amount, inputValue]);

  const hasError = maxAllowed !== undefined && amount > maxAllowed;

  return (
    <div className="relative flex flex-col items-end">
      <Input 
        type="number" 
        step="0.01" 
        min="0"
        className={cn("w-24 h-8 text-right", hasError && "border-rose-500 text-rose-500 focus-visible:ring-rose-500")}
        value={inputValue}
        onChange={(e) => {
          setInputValue(e.target.value);
          const parsed = parseFloat(e.target.value);
          if (!isNaN(parsed)) {
            onChange(Math.round(parsed * 100));
          } else if (e.target.value === '') {
            onChange(0);
          }
        }}
        onBlur={() => {
          const parsed = parseFloat(inputValue);
          if (!isNaN(parsed)) {
            setInputValue(parsed.toFixed(2));
          }
        }}
      />
      {hasError && (
        <div className="absolute top-9 right-0 text-[10px] text-rose-500 font-medium whitespace-nowrap bg-white dark:bg-slate-950 px-1 rounded shadow-sm border border-rose-100 dark:border-rose-900/50 z-10 flex items-center gap-1">
          Max: {formatCurrency(maxAllowed)}
          <button 
            type="button" 
            onClick={() => {
              setInputValue((maxAllowed / 100).toString());
              onChange(maxAllowed);
            }}
            className="text-indigo-600 dark:text-indigo-400 underline hover:text-indigo-700 ml-1"
          >
            Fix
          </button>
        </div>
      )}
    </div>
  );
}


function TransactionAllocationList({ 
  allocations, 
  setAllocations, 
  transactions,
  parentAmount
}: { 
  allocations: {transaction_id: string, amount: number, description?: string, date?: string}[], 
  setAllocations: (val: {transaction_id: string, amount: number}[]) => void,
  transactions: Transaction[] | undefined,
  parentAmount?: number
}) {
  const [open, setOpen] = useState(false);
  
  // Use centralized transaction search hook
  const { searchQuery, setSearchQuery, transactions: searchResults, isLoading } = useTransactionSearch();

  // Parent available is the absolute total of the parent minus the sum of ALL allocations
  const totalAllocated = allocations.reduce((sum, a) => sum + a.amount, 0);
  const parentRemaining = parentAmount !== undefined ? Math.abs(parentAmount) - totalAllocated : Infinity;

  return (
    <div className="space-y-3">
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            role="combobox"
            aria-expanded={open}
            className="w-full justify-between rounded-xl border-slate-200 dark:border-slate-700 h-12 text-base font-normal bg-white dark:bg-slate-900"
          >
            <span className="truncate text-slate-500">
              Select transaction to link...
            </span>
            <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[500px] p-0 rounded-xl max-w-[90vw]" align="start">
          <Command shouldFilter={false}>
            <CommandInput 
              placeholder="Search transactions..." 
              value={searchQuery}
              onValueChange={setSearchQuery}
            />
            <CommandList className="max-h-[300px]">
              <CommandEmpty>{isLoading ? 'Searching...' : 'No transaction found.'}</CommandEmpty>
              <CommandGroup>
                {searchResults?.slice(0, 50)?.map((txn) => {
                  const isSelected = allocations.some(a => a.transaction_id === txn.id);
                  return (
                    <CommandItem
                      key={txn.id}
                      value={`${txn.description} ${txn.amount} ${txn.date} ${txn.id}`}
                      onSelect={() => {
                        if (!isSelected) {
                          setAllocations([...allocations, { transaction_id: txn.id, amount: Math.abs(txn.amount), description: txn.description, date: txn.date }]);
                        }
                        setOpen(false);
                      }}
                    >
                      <Check
                        className={cn(
                          "mr-2 h-4 w-4 shrink-0",
                          isSelected ? "opacity-100" : "opacity-0"
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
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      
      {allocations.length > 0 && (
        <div className="space-y-2">
          {allocations.map((alloc, idx) => {
            const txn = transactions?.find(t => t.id === alloc.transaction_id);
            return (
              <div key={alloc.transaction_id} className="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/50">
                <div className="flex-1 overflow-hidden">
                  <div className="font-medium text-sm truncate">{alloc.description || txn?.description || 'Unknown Transaction'}</div>
                  <div className="text-xs text-slate-500">{alloc.date ? formatDate(alloc.date) : (txn ? formatDate(txn.date) : '')}</div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-sm text-slate-500">$</span>
                  <AllocationAmountInput 
                    amount={alloc.amount} 
                    maxAllowed={parentAmount !== undefined ? parentRemaining + alloc.amount : undefined}
                    onChange={(newAmount) => {
                      const newAllocs = [...allocations];
                      newAllocs[idx] = { ...newAllocs[idx], amount: newAmount };
                      setAllocations(newAllocs);
                    }}
                  />
                  <Button 
                    type="button" 
                    variant="ghost" 
                    size="icon" 
                    className="h-8 w-8 text-rose-500 hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-500/10 shrink-0"
                    onClick={() => setAllocations(allocations.filter(a => a.transaction_id !== alloc.transaction_id))}
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}


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
