"use client";

import { useMemo } from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { PiggyBank, SlidersHorizontal } from "lucide-react";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { useQuery } from "@tanstack/react-query";
import { apiFetch, getStoredToken, type PlanningBaseline } from "@/lib/api";
import {
  useQuests,
  useRetirementAccounts,
  useWealthProfile,
} from "../_components/wealth-data";
import { toRetirementFlags, toRetirementProfile } from "../_components/plan-adapters";
import { computeInputs, isConfigured, project } from "./_components/retirement-math";
import { RetirementHeadline } from "./_components/retirement-headline";
import { YourNumber } from "./_components/your-number";
import { AccountsTable } from "./_components/accounts-table";

/**
 * The retirement surface, off its blob.
 *
 * Age, income, assumptions and account mappings all come from the unified
 * profile now, so this page and the cash page can no longer disagree about what
 * the user earns. Its warnings come from the action engine rather than from a
 * second implementation of the same checks — `computeFlags` used to be the only
 * thing in the app that caught unclaimed employer match, and that check now
 * lives in the catalog where it is ordered against everything else and tested.
 */
export default function RetirementPage() {
  const { data: baselineData, isLoading, isError } = useQuery<PlanningBaseline>({
    queryKey: ["reports", "planning", 6],
    queryFn: () =>
      apiFetch<PlanningBaseline>("/reports/planning?months=6", {}, getStoredToken()),
  });
  const { data: wealthProfile, isLoading: profileLoading } = useWealthProfile();
  const { data: retirementAccounts } = useRetirementAccounts();
  const { data: questData } = useQuests();

  const accounts = useMemo(() => baselineData?.accounts ?? [], [baselineData]);
  const summary = questData?.summary;

  const profile = useMemo(
    () => toRetirementProfile(wealthProfile, retirementAccounts, accounts),
    [wealthProfile, retirementAccounts, accounts],
  );
  const inputs = useMemo(
    () => computeInputs(profile, summary),
    [profile, summary],
  );
  const projection = useMemo(() => project(profile, inputs), [profile, inputs]);
  const flags = useMemo(
    () => toRetirementFlags(questData?.quests, profile.accounts),
    [questData, profile.accounts],
  );

  const loading = isLoading || profileLoading;
  const configured = isConfigured(profile);

  const editInputs = (
    <Link href="/wealth/profile">
      <Button variant="outline" className="text-slate-700 dark:text-slate-200">
        <SlidersHorizontal className="mr-2 h-4 w-4" />
        Plan inputs
      </Button>
    </Link>
  );

  return (
    <PageContainer maxWidth="6xl">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="space-y-12"
      >
        <PageHeader
          title="Retirement"
          description="What you need, what you are on course for, and the gap between them."
          className="mb-2"
        >
          {configured && editInputs}
        </PageHeader>

        {isError ? (
          <EmptyState
            icon={PiggyBank}
            title="Could not load your accounts"
            description="The planning report failed to load. Refresh the page to try again."
          />
        ) : loading ? (
          <RetirementHeadline
            profile={profile}
            inputs={inputs}
            projection={projection}
            flags={flags}
            loading
          />
        ) : !configured ? (
          /* Without a date of birth and an income there is nothing honest to
             show, so this is a prompt rather than a page full of zeroes. */
          <EmptyState
            icon={PiggyBank}
            title="Answer a few questions first"
            description="Your bank data cannot supply your date of birth, your gross pay, or what you contribute each month. Those are part of the main setup, and once they are answered this page can tell you whether you are on track."
            action={
              <Link href="/wealth/profile">
                <Button>
                  <PiggyBank className="mr-2 h-4 w-4" />
                  Go to setup
                </Button>
              </Link>
            }
          />
        ) : (
          <>
            <RetirementHeadline
              profile={profile}
              inputs={inputs}
              projection={projection}
              flags={flags}
              loading={false}
            />

            <YourNumber profile={profile} inputs={inputs} loading={false} />

            <AccountsTable
              profile={profile}
              accounts={accounts}
              summary={summary}
              flags={flags}
              loading={false}
            />
          </>
        )}
      </motion.div>
    </PageContainer>
  );
}
