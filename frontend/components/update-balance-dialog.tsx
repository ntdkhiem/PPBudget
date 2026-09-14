"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { format } from "date-fns";

interface UpdateBalanceDialogProps {
  accountId: string;
  accountName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UpdateBalanceDialog({ accountId, accountName, open, onOpenChange }: UpdateBalanceDialogProps) {
  const queryClient = useQueryClient();
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";
  const today = format(new Date(), "yyyy-MM-dd");

  const updateBalanceMutation = useMutation({
    mutationFn: (data: { balance: number; as_of_date?: string }) =>
      apiFetch(`/accounts/${accountId}/balances`, {
        method: "POST",
        body: JSON.stringify(data),
      }, token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["accounts"] });
      queryClient.invalidateQueries({ queryKey: ["reports"] });
      queryClient.invalidateQueries({ queryKey: ["transactions"] });
      onOpenChange(false);
      toast.success("Balance updated");
    },
  });

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const amountStr = formData.get("amount") as string;
    const asOfDate = formData.get("as_of_date") as string;
    const balance = Math.round(parseFloat(amountStr) * 100);

    updateBalanceMutation.mutate({ balance, as_of_date: asOfDate || undefined });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px] rounded-3xl border-slate-200 dark:border-slate-700/60 backdrop-blur-xl bg-white dark:bg-slate-900/90 shadow-2xl">
        <DialogHeader>
          <DialogTitle className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
            Update Balance
          </DialogTitle>
        </DialogHeader>
        <p className="text-sm text-slate-500 dark:text-slate-400 -mt-2">
          Record the current balance for {accountName}.
        </p>
        <form onSubmit={handleSubmit} className="space-y-6 mt-4" key={open ? "open" : "closed"}>
          <div className="space-y-2">
            <Label htmlFor="amount" className="text-slate-700 dark:text-slate-300">Balance ($)</Label>
            <Input
              id="amount"
              name="amount"
              type="number"
              step="0.01"
              placeholder="0.00"
              required
              className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="as_of_date" className="text-slate-700 dark:text-slate-300">Date</Label>
            <Input
              id="as_of_date"
              name="as_of_date"
              type="date"
              defaultValue={today}
              max={today}
              className="rounded-xl border-slate-200 dark:border-slate-700 focus:ring-indigo-500"
            />
          </div>
          <div className="pt-2">
            <Button
              type="submit"
              disabled={updateBalanceMutation.isPending}
              className="w-full bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl py-6 text-lg font-medium shadow-md shadow-indigo-500/20"
            >
              {updateBalanceMutation.isPending ? "Saving..." : "Save Balance"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
