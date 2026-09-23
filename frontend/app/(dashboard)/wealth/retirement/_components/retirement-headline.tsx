"use client";

import { CircleAlert, CircleCheck, TriangleAlert } from "lucide-react";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import {
  centsToDollars,
  formatPct,
  type Flag,
  type RetirementInputs,
  type RetirementProfile,
  type RetirementProjection,
} from "./retirement-math";

const CARD = {
  good: "border-emerald-200 bg-emerald-50/70 dark:border-emerald-500/30 dark:bg-emerald-500/10",
  warn: "border-amber-200 bg-amber-50/70 dark:border-amber-500/30 dark:bg-amber-500/10",
  bad: "border-rose-200 bg-rose-50/70 dark:border-rose-500/30 dark:bg-rose-500/10",
} as const;

const ICON_WRAP = {
  good: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400",
  warn: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400",
  bad: "bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-400",
} as const;

export function RetirementHeadline({
  profile,
  inputs,
  projection,
  flags,
  loading,
}: {
  profile: RetirementProfile;
  inputs: RetirementInputs;
  projection: RetirementProjection;
  flags: Flag[];
  loading: boolean;
}) {
  if (loading) return <Skeleton className="h-40 w-full rounded-3xl" />;

  // An unclaimed match outranks the on-track verdict: it is free money, and
  // fixing it changes the projection the verdict is based on.
  const matchFlag = flags.find((f) => f.kind === "unclaimed_match");

  const tone = matchFlag ? "bad" : projection.onTrack ? "good" : "warn";
  const Icon = tone === "good" ? CircleCheck : tone === "warn" ? TriangleAlert : CircleAlert;

  const action = matchFlag
    ? "Claim your full employer match"
    : projection.onTrack
      ? `On track to retire at ${profile.target_retirement_age}`
      : `Short by ${centsToDollars(projection.gapCents)} at ${profile.target_retirement_age}`;

  const detail = matchFlag
    ? matchFlag.detail
    : projection.onTrack
      ? `Projected ${centsToDollars(projection.projectedCents)} against a ${centsToDollars(
          projection.targetCents,
        )} target — ${formatPct(projection.fundedShare)} of what you need.`
      : projection.requiredMonthlyCents !== null
        ? `Projected ${centsToDollars(projection.projectedCents)} against ${centsToDollars(
            projection.targetCents,
          )}. Contributing ${centsToDollars(projection.requiredMonthlyCents)} a month of your own would close it.`
        : `Projected ${centsToDollars(projection.projectedCents)} against ${centsToDollars(
            projection.targetCents,
          )}. With no years left to compound, contributions alone cannot close the gap.`;

  return (
    <section className={cn("rounded-3xl border p-6 sm:p-8", CARD[tone])}>
      <div className="flex items-start gap-4">
        <div className={cn("shrink-0 rounded-2xl p-2.5", ICON_WRAP[tone])}>
          <Icon className="h-6 w-6" />
        </div>

        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">
            {inputs.yearsToRetirement} {inputs.yearsToRetirement === 1 ? "year" : "years"} to go
          </p>

          <h2 className="mt-1.5 text-2xl font-bold font-heading tracking-tight text-slate-900 sm:text-3xl dark:text-white">
            {action}
          </h2>

          <p className="mt-2 max-w-2xl text-sm text-slate-600 sm:text-base dark:text-slate-300">
            {detail}
          </p>

          <p className="mt-4 text-xs font-medium text-slate-600 dark:text-slate-400">
            In today&rsquo;s dollars, at a {formatPct(inputs.realReturn, 1)} real return
            ({formatPct(profile.expected_return_apr, 1)} growth less{" "}
            {formatPct(profile.inflation_apr, 1)} inflation).
          </p>
        </div>
      </div>
    </section>
  );
}
