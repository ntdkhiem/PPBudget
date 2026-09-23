"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { motion } from "framer-motion";
import { Check, CircleAlert, Sparkles, TriangleAlert } from "lucide-react";
import { cn } from "@/lib/utils";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { IntakeCore } from "../_components/intake-core";
import { FocusedQuestions } from "../_components/focused-questions";
import {
  useDerivedProfile,
  useFieldRegistry,
  useSaveProfile,
  useWealthProfile,
  usd,
  type DerivedValue,
  type FieldSource,
} from "../_components/wealth-data";

/**
 * The profile surface.
 *
 * Two modes. A user who has answered nothing gets the eight-screen core; one
 * who has gets their answers back, with three things the old settings sheets
 * could not show:
 *
 *   - PROVENANCE. Whether a figure was typed, confirmed, or quietly computed.
 *     This is not decoration: high-stakes rules refuse to run off a derived
 *     value, so "we worked this out, is it right?" is a real question with a
 *     real consequence for answering.
 *   - AGE. An answer past its natural cadence is flagged rather than trusted,
 *     because a plan built on last year's salary is the failure mode that is
 *     hardest to notice.
 *   - WHAT IS STILL MISSING, so the remaining questions are visible without
 *     being a wall.
 */

const SOURCE_LABELS: Record<FieldSource, string> = {
  entered: "you told us",
  confirmed: "you confirmed",
  derived: "we worked this out",
  default: "a standard assumption",
};

const SOURCE_STYLES: Record<FieldSource, string> = {
  entered: "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300",
  confirmed: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300",
  derived: "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300",
  default: "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300",
};

export default function WealthProfilePage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: profile, isLoading } = useWealthProfile();
  const { data: derived } = useDerivedProfile();
  const { data: registry } = useFieldRegistry();
  const save = useSaveProfile();

  /**
   * Which mode to show, LATCHED on first load rather than derived each render.
   *
   * The wizard saves each step as it goes, so a mode computed live from
   * "has the user answered anything?" flips to the answers view the instant
   * step one is submitted and throws the user out of their own setup. The
   * decision belongs to the moment the page opened; only finishing or asking
   * for it again may change it.
   */
  const [showIntake, setShowIntake] = useState<boolean | null>(null);

  // ?ask=a,b arrives from an action card's "answer 2 questions to unlock".
  // Honouring it is what keeps that promise honest: the alternative is landing
  // someone on a page of forty fields after telling them there were two.
  const askParam = searchParams.get("ask");
  const askKeys = askParam ? askParam.split(",").filter(Boolean) : [];

  const answered = profile?.fields ?? {};
  const answeredKeys = Object.keys(answered);
  const labels = registry?.labels ?? {};
  const staleKeys = new Set(derived?.stale ?? []);

  useEffect(() => {
    if (isLoading || showIntake !== null) return;
    setShowIntake(answeredKeys.length === 0);
  }, [isLoading, showIntake, answeredKeys.length]);

  // Rendered after the hooks above so their order never varies between renders.
  if (isLoading || showIntake === null) {
    return (
      <PageContainer maxWidth="4xl">
        <Skeleton className="h-96 w-full rounded-3xl" />
      </PageContainer>
    );
  }

  // A focused ask outranks everything, including a first run: someone who
  // followed a specific prompt should get that prompt, not the whole wizard.
  if (askKeys.length > 0) {
    return (
      <PageContainer maxWidth="4xl">
        <FocusedQuestions
          fieldKeys={askKeys}
          labels={labels}
          knownFields={registry?.fields ?? []}
          onDone={() => router.push("/wealth")}
        />
      </PageContainer>
    );
  }

  if (showIntake) {
    return (
      <PageContainer maxWidth="4xl">
        <IntakeCore profile={profile} onDone={() => setShowIntake(false)} />
      </PageContainer>
    );
  }

  /** Accepting a derived figure promotes it from 'derived' to 'confirmed'. */
  const confirm = (v: DerivedValue) => {
    const value = v.amount ?? v.number ?? v.text;
    if (value === undefined) return;
    save.mutate({ fields: { [v.field_key]: value }, source: "confirmed" });
  };

  const unconfirmed = (derived?.values ?? []).filter(
    (v) => !v.already_answered || answered[v.field_key]?.source === "derived",
  );

  return (
    <PageContainer maxWidth="4xl">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="space-y-10"
      >
        <PageHeader
          title="Profile"
          description="What your plan is built on, and where each number came from."
        >
          <Button variant="outline" onClick={() => setShowIntake(true)}>
            Run setup again
          </Button>
        </PageHeader>

        {/* Figures we computed, offered for confirmation. This screen replaces
            seven separate expense questions that people answered worse than the
            transaction data already answers them. */}
        {unconfirmed.length > 0 && (
          <section>
            <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
              <Sparkles className="h-4 w-4" />
              Worked out from your spending
            </h2>
            <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
              Confirming these lets the plan rely on them. Until you do, a few actions stay
              cautious.
            </p>

            {derived?.low_confidence && (
              <div className="mb-4 flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50/60 p-4 dark:border-amber-500/30 dark:bg-amber-500/5">
                <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
                <p className="text-sm text-amber-900 dark:text-amber-200">
                  {/* No data and thin data need different things from the
                      reader: transactions in the first case, categories in the
                      second. One message for both would ask for the wrong one. */}
                  {derived.no_spending_data
                    ? "There is no complete month of spending to work from yet. Connect an account or add a few transactions and these figures will fill in on their own."
                    : `Only ${Math.round((derived.bucket_coverage ?? 0) * 100)}% of your spending is categorised across ${derived.months_of_data} ${derived.months_of_data === 1 ? "month" : "months"}, so treat these as rough. Categorising more transactions sharpens every figure below.`}
                </p>
              </div>
            )}

            <div className="space-y-3">
              {unconfirmed.map((v) => (
                <div
                  key={v.field_key}
                  className="rounded-2xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900/60"
                >
                  <div className="flex flex-wrap items-baseline justify-between gap-3">
                    <h3 className="font-semibold text-slate-900 dark:text-white">
                      {labels[v.field_key] ?? v.field_key.replace(/_/g, " ")}
                    </h3>
                    <span className="text-lg font-bold text-slate-900 dark:text-white">
                      {v.amount != null ? usd(v.amount) : (v.number ?? v.text ?? "—")}
                    </span>
                  </div>
                  <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">{v.basis}</p>
                  <Button
                    size="sm"
                    className="mt-3"
                    onClick={() => confirm(v)}
                    disabled={save.isPending}
                  >
                    <Check className="h-3.5 w-3.5" />
                    That looks right
                  </Button>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* What is on file. */}
        <section>
          <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
            Your answers
          </h2>
          <div className="overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-800">
            {answeredKeys.map((key, i) => {
              const field = answered[key];
              const raw = (profile as unknown as Record<string, unknown>)[key];
              const isStale = staleKeys.has(key);

              return (
                <div
                  key={key}
                  className={cn(
                    "flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3.5",
                    i > 0 && "border-t border-slate-200 dark:border-slate-800",
                    isStale && "bg-amber-50/40 dark:bg-amber-500/5",
                  )}
                >
                  <span className="min-w-0 flex-1 text-sm text-slate-700 dark:text-slate-300">
                    {labels[key] ?? key.replace(/_/g, " ")}
                  </span>

                  <span className="text-sm font-medium text-slate-900 dark:text-white">
                    {formatAnswer(key, raw)}
                  </span>

                  <span
                    className={cn(
                      "rounded-full px-2 py-0.5 text-xs font-medium",
                      SOURCE_STYLES[field.source],
                    )}
                  >
                    {SOURCE_LABELS[field.source]}
                  </span>

                  {isStale && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-500/15 dark:text-amber-300">
                      <TriangleAlert className="h-3 w-3" />
                      worth rechecking
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        </section>

        {/* What remains. Listed, not demanded: each one unlocks a specific
            action, and the overview shows which. */}
        {derived && derived.unanswered.length > 0 && (
          <section>
            <h2 className="mb-1 text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
              Still unanswered
            </h2>
            <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
              None of these are required. Each one unlocks a particular action, and the
              overview shows which.
            </p>
            <div className="flex flex-wrap gap-2">
              {derived.unanswered.map((key) => (
                <span
                  key={key}
                  className="rounded-lg border border-dashed border-slate-300 px-3 py-1.5 text-xs text-slate-600 dark:border-slate-700 dark:text-slate-400"
                >
                  {labels[key] ?? key.replace(/_/g, " ")}
                </span>
              ))}
            </div>
          </section>
        )}
      </motion.div>
    </PageContainer>
  );
}

/** Renders a stored answer in the shape its field actually has. */
function formatAnswer(key: string, value: unknown): string {
  if (value === undefined || value === null) return "—";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  if (typeof value === "string") {
    // ISO dates come back as timestamps; show the day.
    if (/^\d{4}-\d{2}-\d{2}T/.test(value)) return new Date(value).toLocaleDateString();
    return value;
  }
  if (typeof value === "number") {
    // Rates are stored as fractions, money as cents. The key tells them apart.
    if (key.endsWith("_pct") || key.endsWith("_apr") || key.endsWith("_rate") ||
        key.endsWith("_share")) {
      return `${(value * 100).toFixed(2).replace(/\.?0+$/, "")}%`;
    }
    if (key.endsWith("_months") || key.endsWith("_age") || key.endsWith("_month")) {
      return String(value);
    }
    return usd(value);
  }
  return String(value);
}
