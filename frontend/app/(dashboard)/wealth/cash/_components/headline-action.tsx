"use client";

import Link from "next/link";
import { ArrowRight, CircleAlert, CircleCheck, Target, TriangleAlert } from "lucide-react";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { formatMonths, type Headline, type Health, type Waterfall } from "./planning-math";

const TONE_STYLES: Record<Health, { card: string; icon: string; accent: string }> = {
  good: {
    card: "border-emerald-200 bg-emerald-50/70 dark:border-emerald-500/30 dark:bg-emerald-500/10",
    icon: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400",
    accent: "text-emerald-700 dark:text-emerald-300",
  },
  warn: {
    card: "border-amber-200 bg-amber-50/70 dark:border-amber-500/30 dark:bg-amber-500/10",
    icon: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400",
    accent: "text-amber-700 dark:text-amber-300",
  },
  bad: {
    card: "border-rose-200 bg-rose-50/70 dark:border-rose-500/30 dark:bg-rose-500/10",
    icon: "bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-400",
    accent: "text-rose-700 dark:text-rose-300",
  },
  unknown: {
    card: "border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/60",
    icon: "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300",
    accent: "text-slate-700 dark:text-slate-300",
  },
};

const TONE_ICON: Record<Health, typeof Target> = {
  good: CircleCheck,
  warn: TriangleAlert,
  bad: CircleAlert,
  unknown: Target,
};

export function HeadlineAction({
  headline,
  waterfall,
  loading,
}: {
  headline: Headline;
  waterfall: Waterfall;
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-40 w-full rounded-3xl" />;
  }

  const tone = TONE_STYLES[headline.tone];
  const Icon = TONE_ICON[headline.tone];

  // Only worth saying once there is a sequence left to run.
  const showCrossover =
    headline.kind !== "no_essentials" &&
    headline.kind !== "no_surplus" &&
    waterfall.crossoverMonths !== null &&
    waterfall.crossoverMonths > 0;

  return (
    <section className={cn("rounded-3xl border p-6 sm:p-8", tone.card)}>
      <div className="flex items-start gap-4">
        <div className={cn("rounded-2xl p-2.5 shrink-0", tone.icon)}>
          <Icon className="h-6 w-6" />
        </div>

        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">
            Do this next
          </p>

          <h2 className="mt-1.5 text-2xl sm:text-3xl font-bold font-heading tracking-tight text-slate-900 dark:text-white">
            {headline.action}
          </h2>

          <p className="mt-2 max-w-2xl text-sm sm:text-base text-slate-600 dark:text-slate-300">
            {headline.detail}
          </p>

          <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2">
            {headline.href && headline.hrefLabel && (
              <Link
                href={headline.href}
                className={cn(
                  "inline-flex items-center gap-1.5 text-sm font-semibold underline underline-offset-4",
                  tone.accent,
                )}
              >
                {headline.hrefLabel}
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            )}

            {showCrossover && (
              <span className="text-xs font-medium text-slate-600 dark:text-slate-400">
                Everything below is funded in {formatMonths(waterfall.crossoverMonths)}, after
                which the whole surplus is invested.
              </span>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
