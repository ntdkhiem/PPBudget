"use client";

import { useMemo } from "react";
import { motion } from "framer-motion";
import { Compass } from "lucide-react";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { useBaseline, usePlan, useSavePlan } from "./_components/planning-data";
import {
  computeBaseline,
  computeHeadline,
  computeTrends,
  computeWaterfall,
  type FinancialPlan,
  type Strategy,
} from "./_components/planning-math";
import { HeadlineAction } from "./_components/headline-action";
import { AllocationPlan } from "./_components/allocation-plan";
import { PositionCards } from "./_components/position-cards";
import { TrendStrip } from "./_components/trend-strip";
import { GoalsProgress } from "./_components/goals-progress";
import { ScenarioExplorer } from "./_components/scenario-explorer";
import { PlanSettingsSheet } from "./_components/plan-settings-sheet";

export default function PlanningPage() {
  const { data, isLoading, isError } = useBaseline();
  const { plan, isLoading: planLoading } = usePlan();
  const savePlan = useSavePlan();

  const baseline = useMemo(() => computeBaseline(data, plan), [data, plan]);
  const accounts = useMemo(() => data?.accounts ?? [], [data]);
  const months = useMemo(() => data?.months ?? [], [data]);

  // The waterfall is computed once here and handed down: the headline, the
  // allocation section and the goal schedule must all describe the same plan.
  const waterfall = useMemo(
    () => computeWaterfall(baseline, plan, accounts),
    [baseline, plan, accounts],
  );
  const headline = useMemo(
    () => computeHeadline(baseline, plan, waterfall, accounts),
    [baseline, plan, waterfall, accounts],
  );
  const trends = useMemo(() => computeTrends(months), [months]);

  const loading = isLoading || planLoading;

  const handleSave = (next: FinancialPlan) => savePlan.mutate(next);
  const handleStrategy = (strategy: Strategy) => savePlan.mutate({ ...plan, strategy });

  return (
    <PageContainer maxWidth="6xl">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="space-y-12"
      >
        <PageHeader
          title="Planning"
          description="What to do next, and what it costs you to wait."
          className="mb-2"
        >
          <PlanSettingsSheet
            plan={plan}
            baseline={baseline}
            accounts={accounts}
            onSave={handleSave}
            saving={savePlan.isPending}
          />
        </PageHeader>

        {isError ? (
          <EmptyState
            icon={Compass}
            title="Could not load your planning baseline"
            description="The planning report failed to load. Refresh the page to try again."
          />
        ) : (
          <>
            {/* Answer first. Everything below is the evidence for it. */}
            <HeadlineAction headline={headline} waterfall={waterfall} loading={loading} />

            <AllocationPlan
              plan={plan}
              waterfall={waterfall}
              loading={loading}
              onStrategyChange={handleStrategy}
              saving={savePlan.isPending}
            />

            <section className="space-y-4">
              <div>
                <h2 className="text-xl font-semibold font-heading text-slate-900 dark:text-white">
                  Where you stand
                </h2>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                  Averaged over the last {baseline.monthsOfData}{" "}
                  {baseline.monthsOfData === 1 ? "complete month" : "complete months"}. The current
                  month is excluded while it is still in progress.
                </p>
              </div>
              <TrendStrip trends={trends} plan={plan} loading={loading} />
              <PositionCards
                baseline={baseline}
                plan={plan}
                months={months}
                loading={loading}
              />
            </section>

            <GoalsProgress
              baseline={baseline}
              plan={plan}
              accounts={accounts}
              waterfall={waterfall}
              loading={loading}
              onPlanChange={handleSave}
            />

            <ScenarioExplorer baseline={baseline} plan={plan} loading={loading} />
          </>
        )}
      </motion.div>
    </PageContainer>
  );
}
