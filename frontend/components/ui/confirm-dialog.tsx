import * as React from "react"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => void;
  isDestructive?: boolean;
  isLoading?: boolean;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmText = "Confirm",
  cancelText = "Cancel",
  onConfirm,
  isDestructive = false,
  isLoading = false,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px] rounded-3xl border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 backdrop-blur-xl bg-white dark:bg-slate-900/90 shadow-2xl">
        <DialogHeader>
          <DialogTitle className="text-2xl font-bold font-heading text-slate-900 dark:text-white">{title}</DialogTitle>
          <DialogDescription className="text-slate-500 text-lg mt-2">{description}</DialogDescription>
        </DialogHeader>
        <div className="pt-4 flex gap-4 shrink-0 mt-4">
          <Button 
            variant="outline" 
            onClick={() => onOpenChange(false)} 
            disabled={isLoading}
            className="flex-1 rounded-xl py-6 text-lg border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
          >
            {cancelText}
          </Button>
          <Button 
            onClick={onConfirm} 
            disabled={isLoading}
            className={`flex-1 rounded-xl py-6 text-lg shadow-md text-white ${
              isDestructive 
                ? "bg-rose-600 hover:bg-rose-700 shadow-rose-500/20" 
                : "bg-indigo-600 hover:bg-indigo-700 shadow-indigo-500/20"
            }`}
          >
            {isLoading ? "..." : confirmText}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
