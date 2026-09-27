"use client";

import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { CircleCheck, CircleSlash, History, RotateCcw, Undo2 } from "lucide-react";
import { apiFetch, getStoredToken } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { useQuests } from "./wealth-data";

/**
 * What the user has done, read back out of the action history.
 *
 * Every completion was already recorded -- when, and whether the accounts or
 * the user settled it -- but nothing showed it, so the plan only ever listed
 * what was left. Titles come from the server in the past tense, fixed per
 * action: the live list's "Clear the $3,824 balance" reads wrong once paid.
 */

export type ProgressKind = "done" | "reopened" | "skipped" | "restored";

export interface ProgressEvent {
  id: string;
  catalog_key: string;
  kind: ProgressKind;
  source: "auto" | "manual";
  title: string;
  /** What settled it: the user's data, or the user. */
  how: string;
  note?: string;
  at: string;
}

/** Enough to show the latest run of activity without becoming the page. */
const SHOWN = 5;
const FETCHED = 30;

export function useProgress() {
  // History is written as the action list is read -- that is when the engine
  // records what moved -- and when the user marks one, which refetches the
  // list. Keyed to the list's last fetch, the timeline follows both.
  const { dataUpdatedAt } = useQuests();
  return useQuery({
    queryKey: ["wealth", "progress", FETCHED, dataUpdatedAt],
    enabled: dataUpdatedAt > 0,
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const res = await apiFetch<{ events: ProgressEvent[] | null }>(
        `/wealth/progress?limit=${FETCHED}`,
        {},
        getStoredToken(),
      );
      return res.events ?? [];
    },
  });
}

const KIND: Record<ProgressKind, { icon: typeof CircleCheck; tone: string }> = {
  done: { icon: CircleCheck, tone: "text-emerald-600 dark:text-emerald-400" },
  reopened: { icon: RotateCcw, tone: "text-amber-600 dark:text-amber-400" },
  skipped: { icon: CircleSlash, tone: "text-slate-400 dark:text-slate-500" },
  restored: { icon: Undo2, tone: "text-indigo-600 dark:text-indigo-400" },
};

const when = (iso: string) =>
  new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });

export function ProgressTimeline() {
  const { data: events } = useProgress();
  const [all, setAll] = useState(false);

  // Nothing recorded yet is the normal state for a new plan; an empty section
  // saying so would only push the plan down.
  if (!events || events.length === 0) return null;

  const shown = all ? events : events.slice(0, SHOWN);

  return (
    <section>
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        <History className="h-4 w-4" />
        Your progress
      </h2>
      <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
        What has changed in your plan, newest first.
      </p>
      <ol className="overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-800">
        {shown.map((e, i) => {
          const { icon: Icon, tone } = KIND[e.kind] ?? KIND.done;
          return (
            <li
              key={e.id}
              className={cn(
                "flex items-start gap-3 px-5 py-3",
                i > 0 && "border-t border-slate-200 dark:border-slate-800",
              )}
            >
              <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", tone)} />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-slate-900 dark:text-white">{e.title}</p>
                <p className="text-xs text-slate-500 dark:text-slate-400">
                  {e.how}
                  {e.note && <> · &ldquo;{e.note}&rdquo;</>}
                </p>
              </div>
              <time dateTime={e.at} className="shrink-0 text-xs text-slate-500 dark:text-slate-400">
                {when(e.at)}
              </time>
            </li>
          );
        })}
      </ol>
      {events.length > SHOWN && (
        <Button
          variant="ghost"
          size="sm"
          className="mt-2 text-slate-600 dark:text-slate-300"
          onClick={() => setAll((a) => !a)}
        >
          {all ? "Show less" : `Show all ${events.length}`}
        </Button>
      )}
    </section>
  );
}
