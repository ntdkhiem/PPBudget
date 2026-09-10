import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogTrigger } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Plus, Loader2 } from "lucide-react";
import { Account, Category, Subscription } from "@/lib/api";
import { formatCurrency } from "@/lib/utils";

interface AddTransactionDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (e: React.FormEvent<HTMLFormElement>) => void;
  isPending: boolean;
  accounts: Account[] | undefined;
  categories: Category[] | undefined;
  subscriptions: Subscription[] | undefined;
}

export default function AddTransactionDialog({
  isOpen,
  onOpenChange,
  onSubmit,
  isPending,
  accounts,
  categories,
  subscriptions,
}: AddTransactionDialogProps) {
  const handleOpenChange = (open: boolean) => {
    if (!open) {
      if (window.confirm("Are you sure you want to cancel? Any unsaved changes will be lost.")) {
        onOpenChange(false);
      }
    } else {
      onOpenChange(true);
    }
  };

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl px-5 h-11 shadow-md shadow-indigo-500/20 flex items-center gap-2 transition-all active:scale-95">
          <Plus className="h-4 w-4" /> Add Transaction
        </Button>
      </DialogTrigger>
      <DialogContent 
        onInteractOutside={(e) => e.preventDefault()}
        className="sm:max-w-[425px] rounded-3xl border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 backdrop-blur-xl bg-white dark:bg-slate-900/90 shadow-2xl"
      >
        <DialogHeader>
          <DialogTitle className="text-2xl font-bold font-heading text-slate-900 dark:text-white">Add Transaction</DialogTitle>
          <DialogDescription>Create a new manual transaction.</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4 mt-4">
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
            <Button type="submit" disabled={isPending} className="w-full bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl py-6 text-lg font-medium shadow-md shadow-indigo-500/20">
              {isPending ? <Loader2 className="h-5 w-5 animate-spin mx-auto" /> : "Save"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
