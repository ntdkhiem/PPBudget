"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { apiFetch, getStoredToken } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * The on/off switch for deadline reminders: an email a week before each dated
 * action in the wealth plan (the ESPP sale, the 401(k) rate before payroll
 * closes, the IRA before the filing deadline).
 *
 * On unless turned off -- the server reads anything but "off" as on, which
 * includes never having been set.
 */

const KEY = "deadline_reminders";
const QUERY_KEY = ["settings", "values", KEY] as const;

export function DeadlineRemindersToggle() {
  const queryClient = useQueryClient();
  const { data: value, isLoading } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: async () => {
      const res = await apiFetch<{ value: string }>(`/settings/values/${KEY}`, {}, getStoredToken());
      return res.value;
    },
  });
  const save = useMutation({
    mutationFn: (next: "on" | "off") =>
      apiFetch(
        `/settings/values/${KEY}`,
        { method: "PUT", body: JSON.stringify({ value: next }) },
        getStoredToken(),
      ),
    onSuccess: (_, next) => {
      queryClient.setQueryData(QUERY_KEY, next);
      toast.success(next === "on" ? "Deadline reminders are on" : "Deadline reminders are off");
    },
    onError: (err: Error) => toast.error(err.message || "Failed to save the setting"),
  });

  const on = value !== "off";

  return (
    <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
      <div className="text-sm text-slate-600 dark:text-slate-400 max-w-lg leading-relaxed">
        <span className="font-medium text-slate-900 dark:text-white">Deadline reminders.</span>{" "}
        An email a week before each dated action in your wealth plan, such as selling ESPP shares
        or raising your 401(k) before payroll closes for the year.
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={on}
        aria-label="Deadline reminders"
        disabled={isLoading || save.isPending}
        onClick={() => save.mutate(on ? "off" : "on")}
        className={cn(
          "relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors disabled:opacity-50",
          on ? "bg-emerald-600" : "bg-slate-300 dark:bg-slate-700",
        )}
      >
        <span
          className={cn(
            "inline-block h-5 w-5 rounded-full bg-white shadow transition-transform",
            on ? "translate-x-5" : "translate-x-0.5",
          )}
        />
      </button>
    </div>
  );
}
