import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch, getStoredToken, Insight } from "@/lib/api";

const SEVERITY_SCORE: Record<string, number> = { high: 3, medium: 2, low: 1 };

export function useInsights({ enabled = true }: { enabled?: boolean } = {}) {
  const queryClient = useQueryClient();
  const token = getStoredToken();

  const { data, isLoading } = useQuery<Insight[]>({
    queryKey: ["reports", "insights"],
    queryFn: () => apiFetch<Insight[]>("/reports/insights", {}, token),
    enabled: enabled && !!token,
  });

  const dismissMutation = useMutation({
    mutationFn: (insightId: string) => apiFetch(`/reports/insights/${insightId}/dismiss`, { method: "POST" }, token),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["reports", "insights"] }),
  });

  const insights = useMemo(
    () => [...(data ?? [])].sort((a, b) => (SEVERITY_SCORE[b.severity] ?? 0) - (SEVERITY_SCORE[a.severity] ?? 0)),
    [data]
  );

  return { insights, isLoading, dismiss: dismissMutation.mutate };
}
