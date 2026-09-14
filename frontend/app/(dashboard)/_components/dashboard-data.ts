import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { format } from "date-fns";
import { apiFetch, getStoredToken, Category, DashboardSummary } from "@/lib/api";
import { useDateRange } from "@/app/contexts/DateRangeContext";

/** start_date/end_date query string for the sidebar's global date range. */
export function useRangeQueryParams() {
  const { date } = useDateRange();
  return useMemo(() => {
    const params = new URLSearchParams();
    if (date?.from) params.append("start_date", format(date.from, "yyyy-MM-dd"));
    if (date?.to) params.append("end_date", format(date.to, "yyyy-MM-dd"));
    return params.toString();
  }, [date]);
}

export function useSummary() {
  const params = useRangeQueryParams();
  return useQuery<DashboardSummary>({
    queryKey: ["reports", "summary", params],
    queryFn: () => apiFetch<DashboardSummary>(`/reports/summary?${params}`, {}, getStoredToken()),
  });
}

export function useCategoryNames() {
  const { data } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: () => apiFetch<Category[]>("/categories", {}, getStoredToken()),
  });
  return useMemo(() => new Map((data ?? []).map((c) => [c.id, c.name])), [data]);
}
