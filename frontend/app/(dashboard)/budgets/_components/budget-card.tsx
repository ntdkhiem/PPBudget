"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Check, Edit2, Trash2, X } from "lucide-react";
import { formatCurrency, parseDollarsToCents } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { BUCKETS, type Bucket, type BudgetCardData } from "./budget-data";

interface BudgetCardProps {
  card: BudgetCardData;
  index: number;
  isEditing: boolean;
  /** Future months are projections and can't be edited. */
  canEdit: boolean;
  isPastMonth: boolean;
  isSaving: boolean;
  onStartEdit: () => void;
  onCancelEdit: () => void;
  onSave: (change: { limitCents: number; bucket: Bucket }) => void;
  onDelete: () => void;
}

export function BudgetCard({
  card, index, isEditing, canEdit, isPastMonth, isSaving, onStartEdit, onCancelEdit, onSave, onDelete,
}: BudgetCardProps) {
  const [editLimit, setEditLimit] = useState("");
  const [editBucket, setEditBucket] = useState<Bucket>(card.bucket);
  const editLimitCents = parseDollarsToCents(editLimit);

  const progress = card.limit > 0 ? Math.min((card.spent / card.limit) * 100, 100) : (card.spent > 0 ? 100 : 0);
  // For Savings the limit is a goal: going past it is good, not overspending.
  const isSavings = card.bucket === "savings";
  const isOverLimit = !isSavings && card.spent > card.limit;
  const isGoalMet = isSavings && card.limit > 0 && card.spent >= card.limit;
  const remaining = Math.max(0, card.left);

  const barColor = isSavings
    ? (isGoalMet ? 'bg-emerald-500' : 'bg-indigo-500')
    : (isOverLimit ? 'bg-red-500' : progress > 80 ? 'bg-amber-400' : 'bg-indigo-500');

  const startEdit = () => {
    setEditLimit((card.limit / 100).toString());
    setEditBucket(card.bucket);
    onStartEdit();
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: index * 0.05 }}
      className={`p-5 rounded-2xl shadow-sm flex flex-col border transition-all ${
        isOverLimit && card.limit > 0
          ? 'bg-red-50/30 dark:bg-red-950/20 border-red-200 dark:border-red-900/60 shadow-[0_0_15px_rgba(239,68,68,0.05)]'
          : 'bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800/60 hover:shadow-md hover:border-slate-300'
      }`}
    >
      <div className="flex justify-between items-center mb-4">
        <div className="flex items-center gap-2 min-w-0">
          <h3 className="text-md font-bold text-slate-800 dark:text-slate-100 truncate pr-2">{card.catName}</h3>
          {isEditing && (
            <Select value={editBucket} onValueChange={(val) => setEditBucket(val as Bucket)}>
              <SelectTrigger className="h-7 w-24 text-xs px-2">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {BUCKETS.map((b) => (
                  <SelectItem key={b.key} value={b.key}>{b.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
        <AnimatePresence mode="wait">
          {isEditing ? (
            <motion.div
              key="edit"
              initial={{ opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.9 }}
              className="flex gap-1"
            >
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label="Save budget"
                disabled={editLimitCents === null || isSaving}
                onClick={() => editLimitCents !== null && onSave({ limitCents: editLimitCents, bucket: editBucket })}
                className="text-emerald-600 hover:bg-emerald-50 h-8 w-8"
              >
                <Check size={16} />
              </Button>
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label="Cancel editing"
                onClick={onCancelEdit}
                className="text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 h-8 w-8"
              >
                <X size={16} />
              </Button>
            </motion.div>
          ) : (
            canEdit && (
              <motion.div
                key="view"
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.9 }}
                className="flex gap-1"
              >
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={`Edit ${card.catName} budget`}
                  onClick={startEdit}
                  className="text-slate-400 hover:text-indigo-600 h-8 w-8"
                >
                  <Edit2 size={14} />
                </Button>
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={`Delete ${card.catName} budget`}
                  onClick={onDelete}
                  className="text-slate-400 hover:text-red-500 h-8 w-8"
                >
                  <Trash2 size={14} />
                </Button>
              </motion.div>
            )
          )}
        </AnimatePresence>
      </div>

      <div className="flex justify-between items-end mb-3">
        <div>
          <p className="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-0.5">{isSavings ? "Saved" : "Spent"}</p>
          <p className={`text-xl font-bold tracking-tight ${isOverLimit && card.limit > 0 ? "text-red-600" : isGoalMet ? "text-emerald-600 dark:text-emerald-400" : "text-slate-900 dark:text-slate-100"}`}>
            {formatCurrency(card.spent)}
          </p>
        </div>
        <div className="text-right">
          <p className="text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-0.5">{isSavings ? "Goal" : "Limit"}</p>
          <AnimatePresence mode="wait">
            {isEditing ? (
              <motion.div
                key="edit-input"
                initial={{ opacity: 0, x: 10 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -10 }}
                className="flex items-center justify-end h-7"
              >
                <span className="text-slate-500 mr-1 font-semibold">$</span>
                <Input
                  type="number"
                  aria-label={`${card.catName} limit`}
                  value={editLimit}
                  onChange={e => setEditLimit(e.target.value)}
                  className="w-20 h-7 px-2 text-right font-semibold text-slate-900 dark:text-slate-100"
                  autoFocus
                />
              </motion.div>
            ) : (
              <motion.p
                key="view-limit"
                initial={{ opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 10 }}
                className="text-md font-semibold text-slate-500 h-7 flex items-center justify-end"
              >
                {formatCurrency(card.limit)}
              </motion.p>
            )}
          </AnimatePresence>
        </div>
      </div>

      <div className="w-full bg-slate-100 dark:bg-slate-800 rounded-full h-1.5 mb-3 overflow-hidden">
        <motion.div
          className={`h-full rounded-full ${barColor}`}
          initial={{ width: 0 }}
          animate={{ width: `${progress}%` }}
          transition={{ duration: 1, ease: "easeOut" }}
        />
      </div>

      {card.limit > 0 && (
        <div className="flex justify-between mt-auto pt-3 border-t border-slate-100 dark:border-slate-800/50">
          <div className="text-left">
            <p className="text-[9px] uppercase font-bold text-slate-400 mb-0.5">{isSavings ? "To Go" : "Left"}</p>
            <p className={`text-xs font-semibold ${
              isSavings
                ? (isGoalMet ? 'text-emerald-600' : 'text-slate-700 dark:text-slate-300')
                : (remaining === 0 ? 'text-red-500' : 'text-slate-700 dark:text-slate-300')
            }`}>
              {isGoalMet ? "Goal met" : formatCurrency(remaining)}
            </p>
          </div>
          {isSavings ? (
            <CardStat label="Progress" className={isGoalMet ? 'text-emerald-600' : 'text-slate-700 dark:text-slate-300'}>
              {Math.round((card.spent / card.limit) * 100)}%
            </CardStat>
          ) : isPastMonth ? (
            // The month is over, so there is no daily allowance left to show.
            <CardStat label="Avg / Day" className="text-slate-700 dark:text-slate-300">
              {formatCurrency(card.spentPerDay)}
            </CardStat>
          ) : (
            <CardStat label="Safe / Day" className={card.leftPerDay === 0 ? 'text-red-500' : 'text-emerald-600'}>
              {formatCurrency(card.leftPerDay)}
            </CardStat>
          )}
        </div>
      )}
    </motion.div>
  );
}

function CardStat({ label, className, children }: { label: string; className: string; children: React.ReactNode }) {
  return (
    <div className="text-right">
      <p className="text-[9px] uppercase font-bold text-slate-400 mb-0.5">{label}</p>
      <p className={`text-xs font-semibold ${className}`}>{children}</p>
    </div>
  );
}

export function BudgetCardSkeleton() {
  return (
    <div className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800/60 flex flex-col gap-4">
      <Skeleton className="h-5 w-28" />
      <div className="flex justify-between items-end">
        <Skeleton className="h-7 w-24" />
        <Skeleton className="h-5 w-16" />
      </div>
      <Skeleton className="h-1.5 w-full rounded-full" />
      <div className="flex justify-between pt-3 border-t border-slate-100 dark:border-slate-800/50">
        <Skeleton className="h-6 w-14" />
        <Skeleton className="h-6 w-14" />
      </div>
    </div>
  );
}
