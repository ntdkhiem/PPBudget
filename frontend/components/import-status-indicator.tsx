"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { RefreshCw, CheckCircle2, XCircle, Clock } from "lucide-react";
import { cn } from "@/lib/utils";
import { format } from "date-fns";

export function ImportStatusIndicator() {
  const token = typeof window !== "undefined" ? localStorage.getItem("ppbudget_token") || "" : "";

  const { data: statusData } = useQuery({
    queryKey: ["importStatus"],
    queryFn: () => apiFetch<{ status: string; current: number; total: number; error: string }>("/import/simplefin/status", {}, token),
    enabled: !!token,
    refetchInterval: (query: any) => (query.state?.data?.status === "running" ? 2000 : 30000), // poll every 2s if running, else every 30s
  });

  const { data: configData } = useQuery({
    queryKey: ["importConfig"],
    queryFn: () => apiFetch<{ next_sync_time?: string; auto_sync?: boolean }>("/import/simplefin/config", {}, token),
    enabled: !!token,
    refetchInterval: 60000, // every minute
  });

  if (!statusData || !configData) return null;

  const isRunning = statusData.status === "running";
  const isError = statusData.status === "error";
  const isCompleted = statusData.status === "completed";
  
  let nextSyncStr = "";
  if (configData.auto_sync && configData.next_sync_time && new Date(configData.next_sync_time).getFullYear() > 2000) {
    nextSyncStr = format(new Date(configData.next_sync_time), "MMM d, h:mm a");
  }

  return (
    <div className="flex items-center gap-2 text-xs font-medium px-3 py-1.5 rounded-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-sm">
      {isRunning && (
        <>
          <RefreshCw className="h-3.5 w-3.5 text-indigo-500 animate-spin" />
          <span className="text-indigo-600 dark:text-indigo-400">Syncing... {statusData.total > 0 ? `${statusData.current}/${statusData.total}` : ''}</span>
        </>
      )}
      {!isRunning && isError && (
        <>
          <XCircle className="h-3.5 w-3.5 text-red-500" />
          <span className="text-red-600 dark:text-red-400 truncate max-w-[150px]" title={statusData.error || "Sync failed"}>Failed</span>
        </>
      )}
      {!isRunning && !isError && isCompleted && (
        <>
          <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
          <span className="text-emerald-600 dark:text-emerald-400">Synced</span>
        </>
      )}
      {!isRunning && !isError && !isCompleted && (
        <>
          <Clock className="h-3.5 w-3.5 text-slate-400" />
          <span className="text-slate-500">Idle</span>
        </>
      )}
      
      {nextSyncStr && !isRunning && (
        <>
          <span className="text-slate-300 dark:text-slate-700 mx-1">|</span>
          <span className="text-slate-400 dark:text-slate-500 font-normal">Next: {nextSyncStr}</span>
        </>
      )}
    </div>
  );
}
