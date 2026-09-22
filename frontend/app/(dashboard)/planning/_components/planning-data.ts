"use client";

import { useCallback, useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { apiFetch, getStoredToken, type PlanningBaseline } from "@/lib/api";
import { DEFAULT_PLAN, type FinancialPlan } from "./planning-math";

/** How many trailing months the baseline averages over. */
export const BASELINE_MONTHS = 6;

const SETTING_KEY = "financial_plan";
const PLAN_QUERY_KEY = ["setting", SETTING_KEY] as const;

/**
 * The plan lives in user_settings.value, a free-form TEXT column, so anything
 * could be in there — an older shape, a half-written edit, plain garbage. Every
 * field is optional with a default so a bad blob degrades to defaults instead
 * of white-screening the page.
 */
const goalSchema = z.object({
  id: z.string(),
  name: z.string(),
  target_cents: z.number().int().nonnegative(),
  target_date: z.string().optional(),
  linked_account_id: z.string().optional(),
  current_cents: z.number().int().optional(),
  priority: z.number().int().default(0),
});

const planSchema = z.object({
  version: z.literal(1).default(1),
  strategy: z.enum(["aggressive", "balanced", "flexible"]).default("balanced"),
  overrides: z
    .object({
      monthly_income_cents: z.number().int().nonnegative().optional(),
      essential_expenses_cents: z.number().int().nonnegative().optional(),
      liquid_account_ids: z.array(z.string()).optional(),
    })
    .default({}),
  emergency_fund: z
    .object({ target_months: z.number().min(1).max(24).default(6) })
    .default({ target_months: 6 }),
  goals: z.array(goalSchema).default([]),
  debts: z
    .array(
      z.object({
        account_id: z.string(),
        apr: z.number().min(0).max(1),
        min_payment_cents: z.number().int().nonnegative().default(0),
      }),
    )
    .default([]),
  assumptions: z
    .object({ invest_return_apr: z.number().min(0).max(0.5).default(0) })
    .default({ invest_return_apr: 0 }),
});

export function parsePlan(raw: string | undefined): FinancialPlan {
  if (!raw) return DEFAULT_PLAN;
  try {
    const result = planSchema.safeParse(JSON.parse(raw));
    return result.success ? (result.data as FinancialPlan) : DEFAULT_PLAN;
  } catch {
    return DEFAULT_PLAN;
  }
}

/** Trailing baseline. Independent of the sidebar date range by design. */
export function useBaseline(months: number = BASELINE_MONTHS) {
  return useQuery<PlanningBaseline>({
    queryKey: ["reports", "planning", months],
    queryFn: () =>
      apiFetch<PlanningBaseline>(`/reports/planning?months=${months}`, {}, getStoredToken()),
  });
}

export function usePlan() {
  const query = useQuery<{ value: string }>({
    queryKey: PLAN_QUERY_KEY,
    queryFn: () =>
      apiFetch<{ value: string }>(`/settings/values/${SETTING_KEY}`, {}, getStoredToken()),
  });

  const plan = useMemo(() => parsePlan(query.data?.value), [query.data?.value]);

  return { ...query, plan };
}

export function useSavePlan() {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: (plan: FinancialPlan) =>
      apiFetch(
        `/settings/values/${SETTING_KEY}`,
        { method: "PUT", body: JSON.stringify({ value: JSON.stringify(plan) }) },
        getStoredToken(),
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
    },
  });

  return mutation;
}

/**
 * Convenience wrapper for the common "change one part of the plan" edit,
 * so callers never have to spread the whole object by hand.
 */
export function usePlanEditor() {
  const { plan, isLoading } = usePlan();
  const save = useSavePlan();

  const update = useCallback(
    (patch: Partial<FinancialPlan>) => save.mutate({ ...plan, ...patch }),
    [plan, save],
  );

  const updateOverrides = useCallback(
    (patch: Partial<FinancialPlan["overrides"]>) =>
      save.mutate({ ...plan, overrides: { ...plan.overrides, ...patch } }),
    [plan, save],
  );

  return { plan, isLoading, update, updateOverrides, isSaving: save.isPending };
}
