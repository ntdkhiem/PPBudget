"use client";

import { useState } from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight, CalendarClock, ChevronDown, ChevronRight, Compass, Lock } from "lucide-react";
import { cn } from "@/lib/utils";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ActionCard } from "./_components/action-card";
import {
  useFieldRegistry,
  useGroupedQuests,
  useQuests,
  useSetQuestStatus,
  useWealthProfile,
  type Quest,
} from "./_components/wealth-data";

/**
 * The Wealth Strategy overview.
 *
 * Three bands, in the order they demand attention:
 *
 *   1. Deadlines, above everything. These are the actions with a cost for
 *      waiting, and they deliberately ignore the phase order -- payroll closes
 *      on 31 December whatever stage the user has reached, and a vest lands
 *      when the grant says so.
 *   2. The current phase, expanded.
 *   3. Every other phase, collapsed to a row, so the shape of the plan stays
 *      visible without burying the part that matters today.
 */
export default function WealthOverviewPage() {
  const { data, isLoading, isError } = useQuests();
  const { data: profile } = useWealthProfile();
  const { data: registry } = useFieldRegistry();
  const setStatus = useSetQuestStatus();

  const { dated, byPhase, current } = useGroupedQuests(data?.quests, data?.phases);
  const [expanded, setExpanded] = useState<number | null>(null);

  const labels = registry?.labels ?? {};
  const answeredCount = Object.keys(profile?.fields ?? {}).length;

  const handlers = {
    fieldLabels: labels,
    onComplete: (q: Quest) => setStatus.mutate({ questId: q.id, status: "complete" }),
    onSkip: (q: Quest) => setStatus.mutate({ questId: q.id, status: "skipped" }),
    onReopen: (q: Quest) => setStatus.mutate({ questId: q.id, status: "available" }),
    busy: setStatus.isPending,
  };

  if (isLoading) {
    return (
      <PageContainer maxWidth="5xl">
        <Skeleton className="mb-8 h-24 w-full rounded-3xl" />
        <div className="space-y-4">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-32 w-full rounded-2xl" />
          ))}
        </div>
      </PageContainer>
    );
  }

  if (isError) {
    return (
      <PageContainer maxWidth="5xl">
        <EmptyState
          icon={Compass}
          title="Your plan could not be built"
          description="Something went wrong assembling your action list. Refreshing usually sorts it."
        />
      </PageContainer>
    );
  }

  // A brand-new account gets an invitation rather than a wall of blocked
  // cards: the list is technically correct but says nothing until it has
  // something to work from.
  if (answeredCount === 0) {
    return (
      <PageContainer maxWidth="5xl">
        <PageHeader
          title="Wealth Strategy"
          description="An ordered plan built from your own numbers."
        />
        <EmptyState
          icon={Compass}
          title="Answer eight questions to get your plan"
          description="Most of what a plan needs is already in your transactions. The rest — your pay, your employer's match, how you file — takes about five minutes and produces a real list of what to do next."
          action={
            <Link href="/wealth/profile">
              <Button>
                Start
                <ArrowRight className="h-4 w-4" />
              </Button>
            </Link>
          }
        />
      </PageContainer>
    );
  }

  const currentPhase = byPhase.find((p) => p.phase.number === current);
  const protection = byPhase.find((p) => p.phase.number === 0);

  return (
    <PageContainer maxWidth="5xl">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="space-y-10"
      >
        <PageHeader
          title="Wealth Strategy"
          description="An ordered plan built from your own numbers."
        />

        {/* 1. Deadlines. Above the phases on purpose. */}
        {dated.length > 0 && (
          <section>
            <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
              <CalendarClock className="h-4 w-4" />
              Dated
            </h2>
            <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
              These have deadlines that do not wait for the rest of the plan.
            </p>
            <div className="space-y-3">
              {dated.map((q) => (
                <ActionCard key={q.id} quest={q} {...handlers} />
              ))}
            </div>
          </section>
        )}

        {/* Protection sits outside the phase order: it guards the income that
            funds every other action rather than advancing the plan. */}
        {protection && protection.items.length > 0 && (
          <section>
            <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
              {protection.phase.name}
            </h2>
            <div className="space-y-3">
              {protection.items
                .filter((q) => !q.due_date)
                .map((q) => (
                  <ActionCard key={q.id} quest={q} {...handlers} />
                ))}
            </div>
          </section>
        )}

        {/* 2. The current phase, expanded. */}
        {currentPhase && (
          <section>
            <div className="mb-4 flex items-baseline justify-between gap-4">
              <h2 className="text-lg font-bold font-heading text-slate-900 dark:text-white">
                Phase {currentPhase.phase.number} · {currentPhase.phase.name}
              </h2>
              <span className="shrink-0 text-sm text-slate-500 dark:text-slate-400">
                {currentPhase.done} of {currentPhase.items.length} done
              </span>
            </div>
            <div className="space-y-3">
              {currentPhase.items
                .filter((q) => !q.due_date)
                .map((q) => (
                  <ActionCard key={q.id} quest={q} {...handlers} />
                ))}
            </div>
          </section>
        )}

        {/* 3. The rest of the plan, collapsed. */}
        <section>
          <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
            The rest of the plan
          </h2>
          <div className="space-y-2">
            {byPhase
              .filter((p) => p.phase.number > 0 && p.phase.number !== current)
              .map((p) => {
                const isOpen = expanded === p.phase.number;
                const allDone = p.items.length > 0 && p.done === p.items.length;

                return (
                  <div
                    key={p.phase.number}
                    className="overflow-hidden rounded-2xl border border-slate-200 bg-white/60 dark:border-slate-800 dark:bg-slate-900/40"
                  >
                    <button
                      type="button"
                      onClick={() => setExpanded(isOpen ? null : p.phase.number)}
                      aria-expanded={isOpen}
                      className="flex w-full items-center gap-3 px-5 py-4 text-left transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/40"
                    >
                      {isOpen ? (
                        <ChevronDown className="h-4 w-4 shrink-0 text-slate-400" />
                      ) : (
                        <ChevronRight className="h-4 w-4 shrink-0 text-slate-400" />
                      )}

                      <span
                        className={cn(
                          "flex-1 text-sm font-semibold",
                          p.locked
                            ? "text-slate-500 dark:text-slate-400"
                            : "text-slate-900 dark:text-white",
                        )}
                      >
                        Phase {p.phase.number} · {p.phase.name}
                      </span>

                      {p.locked && (
                        <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                          <Lock className="h-3 w-3" />
                          finish the phase before it
                        </span>
                      )}
                      {allDone && (
                        <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-300">
                          done
                        </span>
                      )}
                      {!p.locked && !allDone && p.actionable > 0 && (
                        <span className="text-xs text-slate-500 dark:text-slate-400">
                          {p.actionable} to do
                        </span>
                      )}
                    </button>

                    {isOpen && (
                      <div className="space-y-3 border-t border-slate-200 p-4 dark:border-slate-800">
                        {p.items.length === 0 ? (
                          <p className="px-1 py-2 text-sm text-slate-500 dark:text-slate-400">
                            Nothing here applies to you.
                          </p>
                        ) : (
                          p.items.map((q) => (
                            <ActionCard key={q.id} quest={q} {...handlers} />
                          ))
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
          </div>
        </section>
      </motion.div>
    </PageContainer>
  );
}
