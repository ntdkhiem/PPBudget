"use client";

import { Gift, PiggyBank, Target, Wallet } from "lucide-react";
import { cn } from "@/lib/utils";
import { StatCard } from "@/components/stat-card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  centsToDollars,
  formatPct,
  type RetirementInputs,
  type RetirementProfile,
} from "./retirement-math";

function Bar({ fill, className }: { fill: number; className: string }) {
  return (
    <div className="mt-auto pt-4">
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
        <div
          className={cn("h-full rounded-full transition-all", className)}
          style={{ width: `${Math.min(Math.max(fill * 100, 0), 100)}%` }}
        />
      </div>
    </div>
  );
}

export function YourNumber({
  profile,
  inputs,
  loading,
}: {
  profile: RetirementProfile;
  inputs: RetirementInputs;
  loading: boolean;
}) {
  if (loading) {
    return (
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-4">
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-44 w-full rounded-3xl" />
        ))}
      </div>
    );
  }

  const matchRate =
    inputs.ownAnnualContributionCents > 0
      ? inputs.employerMatchAnnualCents / inputs.ownAnnualContributionCents
      : 0;

  return (
    <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-4">
      {/* The target, and the arithmetic behind it stated plainly. */}
      <StatCard title="Your number" icon={Target} tone="indigo" action="the target">
        <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
          {centsToDollars(inputs.targetNestEggCents)}
        </div>
        <p className="mt-1 text-xs font-medium text-slate-600 dark:text-slate-400">
          {centsToDollars(inputs.targetAnnualSpendCents)} a year at a{" "}
          {formatPct(profile.withdrawal_rate, 1)} withdrawal rate
          {profile.withdrawal_rate > 0 && (
            <> — {Math.round(1 / profile.withdrawal_rate)}&times; annual spending</>
          )}
          .
        </p>
        <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
          {profile.target_annual_spend_cents
            ? "Based on the spending figure you set."
            : "Based on your current essentials from Cash Plan."}
        </p>
      </StatCard>

      {/* Where you are now. */}
      <StatCard title="Saved so far" icon={Wallet} tone="emerald" action="linked accounts">
        <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
          {centsToDollars(inputs.currentAssetsCents)}
        </div>
        <p className="mt-1 text-xs font-medium text-slate-600 dark:text-slate-400">
          {inputs.targetNestEggCents > 0
            ? `${formatPct(inputs.currentAssetsCents / inputs.targetNestEggCents, 1)} of the target, before any growth.`
            : "Set a target to measure this against."}
        </p>
        <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
          Live balances from {profile.accounts.length}{" "}
          {profile.accounts.length === 1 ? "account" : "accounts"}.
        </p>
        {inputs.targetNestEggCents > 0 && (
          <Bar
            fill={inputs.currentAssetsCents / inputs.targetNestEggCents}
            className="bg-emerald-500"
          />
        )}
      </StatCard>

      {/* Rate of saving, stated on gross so it is comparable to the usual guidance. */}
      <StatCard title="Savings rate" icon={PiggyBank} tone="amber" action="of gross pay">
        <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
          {formatPct(inputs.grossSavingsRateWithMatch, 1)}
        </div>
        <p className="mt-1 text-xs font-medium text-slate-600 dark:text-slate-400">
          {formatPct(inputs.grossSavingsRate, 1)} from you, the rest from employer match.
        </p>
        <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
          Measured on gross pay — not the take-home figure on Cash Plan.
        </p>
        <Bar fill={inputs.grossSavingsRateWithMatch / 0.15} className="bg-amber-500" />
      </StatCard>

      {/* Free money, and whether it is actually being taken. */}
      <StatCard title="Employer match" icon={Gift} tone="purple" action="per year">
        <div className="text-2xl font-bold font-heading text-slate-900 dark:text-white">
          {centsToDollars(inputs.employerMatchAnnualCents)}
        </div>
        <p className="mt-1 text-xs font-medium text-slate-600 dark:text-slate-400">
          {inputs.employerMatchAnnualCents > 0
            ? `An extra ${formatPct(matchRate)} on top of what you put in.`
            : "No match recorded. Add your employer's terms in setup."}
        </p>
        <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
          {inputs.employerMatchAnnualCents > 0
            ? `${centsToDollars(inputs.employerMatchAnnualCents * inputs.yearsToRetirement)} over ${inputs.yearsToRetirement} years, before any growth on it.`
            : "Nothing to lose if there is no match on offer."}
        </p>
      </StatCard>
    </div>
  );
}
