"use client";

import Link from "next/link";
import {
  CalendarClock,
  Check,
  CircleCheck,
  CircleHelp,
  Lock,
  RotateCcw,
  ShieldCheck,
  TriangleAlert,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  daysUntil,
  formatDate,
  usd,
  type Quest,
  type QuestStatus,
} from "./wealth-data";

/**
 * One generated action.
 *
 * The four states are visually distinct because they ask different things of
 * the reader:
 *
 *   available  do this
 *   blocked    answer something first -- and the card says what, beside what
 *              the action is worth, so the question has a visible price
 *   locked     not yet; shown rather than hidden so the path stays legible
 *   complete   done
 *
 * A blocked card never shows a dollar figure. The figure is precisely what is
 * missing, and inventing one would undo the point of blocking.
 */

const STATUS_STYLES: Record<
  QuestStatus,
  { card: string; badge: string; icon: typeof Check | null }
> = {
  available: {
    card: "border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900/60",
    badge: "",
    icon: null,
  },
  blocked: {
    card: "border-dashed border-amber-300 bg-amber-50/50 dark:border-amber-500/30 dark:bg-amber-500/5",
    badge: "bg-amber-100 text-amber-800 dark:bg-amber-500/15 dark:text-amber-300",
    icon: CircleHelp,
  },
  locked: {
    card: "border-slate-200 bg-slate-50/60 dark:border-slate-800 dark:bg-slate-900/30",
    badge: "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400",
    icon: Lock,
  },
  complete: {
    card: "border-emerald-200 bg-emerald-50/50 dark:border-emerald-500/30 dark:bg-emerald-500/5",
    badge: "bg-emerald-100 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-300",
    icon: CircleCheck,
  },
  skipped: {
    card: "border-slate-200 bg-slate-50/60 dark:border-slate-800 dark:bg-slate-900/30",
    badge: "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400",
    icon: X,
  },
  not_applicable: { card: "hidden", badge: "", icon: null },
};

export function ActionCard({
  quest,
  fieldLabels,
  onComplete,
  onSkip,
  onReopen,
  busy,
}: {
  quest: Quest;
  /** Human labels for missing field keys, keyed by key. */
  fieldLabels: Record<string, string>;
  onComplete: (q: Quest) => void;
  onSkip: (q: Quest) => void;
  onReopen: (q: Quest) => void;
  busy: boolean;
}) {
  const style = STATUS_STYLES[quest.status];
  const StatusIcon = style.icon;
  const days = daysUntil(quest.due_date);

  // A deadline is only worth shouting about once it is close enough to act on.
  const urgent = days !== null && days <= 30;
  const overdue = days !== null && days < 0;

  const isDone = quest.status === "complete" || quest.status === "skipped";
  const isOpen = quest.status === "available";

  return (
    <div className={cn("rounded-2xl border p-5 transition-colors", style.card)}>
      <div className="flex items-start gap-3">
        {StatusIcon && (
          <span className={cn("mt-0.5 rounded-lg p-1.5", style.badge)}>
            <StatusIcon className="h-4 w-4" />
          </span>
        )}

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
            <h3
              className={cn(
                "text-base font-semibold text-slate-900 dark:text-white",
                isDone && "text-slate-500 dark:text-slate-400",
              )}
            >
              {quest.title}
            </h3>

            {quest.due_date && !isDone && (
              <span
                className={cn(
                  "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium",
                  overdue
                    ? "bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300"
                    : urgent
                      ? "bg-amber-100 text-amber-800 dark:bg-amber-500/15 dark:text-amber-300"
                      : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300",
                )}
              >
                <CalendarClock className="h-3 w-3" />
                {overdue
                  ? `overdue since ${formatDate(quest.due_date)}`
                  : days !== null && days <= 60
                    ? `${days} days — ${formatDate(quest.due_date)}`
                    : formatDate(quest.due_date)}
              </span>
            )}

            {/* Who closed it. A figure the app verified against real balances
                and a claim it has no way to check are different kinds of
                "done", and collapsing them would hide which ones are worth
                revisiting. */}
            {isDone && quest.status === "complete" && (
              <span
                className={cn(
                  "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium",
                  quest.completed_source === "auto"
                    ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300"
                    : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300",
                )}
              >
                {quest.completed_source === "auto" ? (
                  <>
                    <ShieldCheck className="h-3 w-3" />
                    checked against your accounts
                  </>
                ) : (
                  "you marked this done"
                )}
              </span>
            )}

            {/* A stale action still renders. Hiding one built on last year's
                salary would conceal the problem as effectively as trusting it. */}
            {quest.stale && (
              <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                <TriangleAlert className="h-3 w-3" />
                based on an old answer
              </span>
            )}
          </div>

          {quest.detail && (
            <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-300">
              {quest.detail}
            </p>
          )}

          {/* The unlock prompt. Naming the questions beside the action's value
              is what makes answering feel worth the interruption. */}
          {quest.status === "blocked" && quest.missing_fields.length > 0 && (
            <div className="mt-3 flex flex-wrap items-center gap-2">
              {/* Deep-links to the specific questions, not the whole profile.
                  "Answer 2 questions" that lands on a page of forty is a worse
                  promise than not making one. */}
              <Link
                href={`/wealth/profile?ask=${encodeURIComponent(quest.missing_fields.join(","))}`}
                className="inline-flex items-center gap-1.5 rounded-lg bg-amber-100 px-3 py-1.5 text-xs font-semibold text-amber-900 transition-colors hover:bg-amber-200 dark:bg-amber-500/15 dark:text-amber-200 dark:hover:bg-amber-500/25"
              >
                Answer {quest.missing_fields.length}{" "}
                {quest.missing_fields.length === 1 ? "question" : "questions"} to unlock
              </Link>
              <span className="text-xs text-slate-600 dark:text-slate-400">
                {quest.missing_fields
                  .map((f) => fieldLabels[f] ?? f.replace(/_/g, " "))
                  .join(", ")}
              </span>
            </div>
          )}

          {quest.target_amount != null && quest.status !== "blocked" && (
            <p className="mt-3 text-sm font-semibold text-slate-900 dark:text-white">
              {usd(quest.target_amount)}
            </p>
          )}

          {(isOpen || isDone) && (
            <div className="mt-4 flex flex-wrap items-center gap-2">
              {isOpen && (
                <>
                  {/* An action the app checks for itself cannot be ticked by
                      hand: the next refresh reopens it from the same data. Say
                      how it closes rather than offer a button that undoes
                      itself. */}
                  {quest.verification === "manual" ? (
                    <Button size="sm" onClick={() => onComplete(quest)} disabled={busy}>
                      <Check className="h-3.5 w-3.5" />
                      Mark done
                    </Button>
                  ) : (
                    <span className="inline-flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                      <ShieldCheck className="h-3.5 w-3.5" />
                      Ticks itself off once your accounts or answers show it is done
                    </span>
                  )}
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => onSkip(quest)}
                    disabled={busy}
                  >
                    Not for me
                  </Button>
                </>
              )}
              {isDone && quest.completed_source !== "auto" && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onReopen(quest)}
                  disabled={busy}
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                  Reopen
                </Button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
