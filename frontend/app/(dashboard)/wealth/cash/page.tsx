"use client";

import { useMemo } from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { Compass, SlidersHorizontal } from "lucide-react";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { apiFetch, getStoredToken, type PlanningBaseline } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import {
  useGoals,
  useQuests,
  useSaveGoal,
  useDeleteGoal,
  useSaveProfile,
  useWealthProfile,
} from "../_components/wealth-data";
import {
  toBaseline,
  toFinancialPlan,
  toHeadline,
  toWaterfall,
} from "../_components/plan-adapters";
import { computeTrends } from "./_components/planning-math";
import { HeadlineAction } from "./_components/headline-action";
import { AllocationPlan } from "./_components/allocation-plan";
import { PositionCards } from "./_components/position-cards";
import { TrendStrip } from "./_components/trend-strip";
import { GoalsProgress } from "./_components/goals-progress";
import { ScenarioExplorer } from "./_components/scenario-explorer";

/**
 * The cash surface, now a VIEW of the server's plan rather than a second
 * implementation of it.
 *
 * What changed: the ordering, the headline and the dollar figures come from the
 * Go catalog. What stayed: the trend strip, the what-if sliders and the goal
 * schedule, which are presentation over the same numbers and have no business
 * round-tripping to the server on every drag.
 *
 * The old version computed its own waterfall, and that waterfall disagreed with
 * the engine on something that mattered -- it put high-APR debt ahead of the
 * starter cushion. Two implementations of one sequence is how a user ends up
 * with two answers to the same question.
 */
export default function CashPage() {
  const { data: baselineData, isLoading: baselineLoading } = useQuery<PlanningBaseline>({
    queryKey: ["reports", "planning", 6],
    queryFn: () =>
      apiFetch<PlanningBaseline>("/reports/planning?months=6", {}, getStoredToken()),
  });
  const { data: questData, isLoading: questsLoading } = useQuests();
  const { data: profile, isLoading: profileLoading } = useWealthProfile();
  const { data: goals } = useGoals();
  const saveGoal = useSaveGoal();
  const deleteGoal = useDeleteGoal();
  const saveProfile = useSaveProfile();

  const summary = questData?.summary;
  const quests = questData?.quests;

  const plan = useMemo(() => toFinancialPlan(profile, goals), [profile, goals]);
  const baseline = useMemo(() => toBaseline(summary, profile), [summary, profile]);
  const waterfall = useMemo(() => toWaterfall(quests, summary), [quests, summary]);
  const headline = useMemo(() => toHeadline(quests, summary), [quests, summary]);

  const accounts = useMemo(() => baselineData?.accounts ?? [], [baselineData]);
  const months = useMemo(() => baselineData?.months ?? [], [baselineData]);
  const trends = useMemo(() => computeTrends(months), [months]);

  const loading = baselineLoading || questsLoading || profileLoading;

  // Goal edits write straight through to their own endpoint. The old page
  // rewrote the entire plan blob to add one goal, which meant a concurrent
  // edit anywhere else in the plan silently lost.
  const handleGoalChange = (next: typeof plan) => {
    const existing = new Map((goals ?? []).map((g) => [g.id, g]));
    for (const g of next.goals) {
      const prior = existing.get(g.id);
      if (
        !prior ||
        prior.name !== g.name ||
        prior.target_amount !== g.target_cents ||
        prior.priority !== g.priority ||
        (prior.target_date ?? "").slice(0, 10) !== (g.target_date ?? "")
      ) {
        saveGoal.mutate({
          id: existing.has(g.id) ? g.id : undefined,
          name: g.name,
          target_amount: g.target_cents,
          target_date: g.target_date,
          linked_account_id: g.linked_account_id,
          current_amount: g.current_cents ?? 0,
          priority: g.priority,
        });
      }
      existing.delete(g.id);
    }
    // Anything left in the map was removed from the list.
    for (const id of existing.keys()) deleteGoal.mutate(id);
  };

  if (!loading && (summary?.months_of_data ?? 0) === 0 && (quests?.length ?? 0) === 0) {
    return (
      <PageContainer maxWidth="6xl">
        <EmptyState
          icon={Compass}
          title="Nothing to plan against yet"
          description="Connect an account or add a few transactions, and this page will start describing where your next dollar should go."
        />
      </PageContainer>
    );
  }

  return (
    <PageContainer maxWidth="6xl">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="space-y-12"
      >
        <PageHeader
          title="Cash"
          description="Where the next dollar goes, and how long each stage takes."
          className="mb-2"
        >
          {/* The settings sheet is gone: every input now lives in one place,
              with its provenance and its age visible beside it. */}
          <Link href="/wealth/profile">
            <Button variant="outline" className="text-slate-700 dark:text-slate-200">
              <SlidersHorizontal className="mr-2 h-4 w-4" />
              Plan inputs
            </Button>
          </Link>
        </PageHeader>

        <HeadlineAction headline={headline} waterfall={waterfall} loading={loading} />

        <AllocationPlan
          plan={plan}
          waterfall={waterfall}
          loading={loading}
          onStrategyChange={(strategy) =>
            saveProfile.mutate({ fields: { strategy }, source: "entered" })
          }
          saving={saveProfile.isPending}
        />

        <section className="space-y-6">
          <h2 className="text-lg font-bold font-heading text-slate-900 dark:text-white">
            Where you stand
          </h2>
          <TrendStrip trends={trends} plan={plan} loading={loading} />
          <PositionCards baseline={baseline} plan={plan} months={months} loading={loading} />
        </section>

        <GoalsProgress
          baseline={baseline}
          plan={plan}
          accounts={accounts}
          waterfall={waterfall}
          loading={loading}
          onPlanChange={handleGoalChange}
        />

        <ScenarioExplorer baseline={baseline} plan={plan} loading={loading} />
      </motion.div>
    </PageContainer>
  );
}
