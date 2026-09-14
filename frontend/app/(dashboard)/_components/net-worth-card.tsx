"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { format, subMonths, subYears } from "date-fns";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Activity, TrendingUp } from "lucide-react";
import { apiFetch, getStoredToken, Account, NetWorthDataPoint } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/utils";
import { StatCard } from "@/components/stat-card";
import { Skeleton } from "@/components/ui/skeleton";

const RANGES = ["1M", "3M", "6M", "1Y", "ALL"] as const;
type Range = (typeof RANGES)[number];

const RANGE_START: Record<Range, (now: Date) => Date> = {
  "1M": (now) => subMonths(now, 1),
  "3M": (now) => subMonths(now, 3),
  "6M": (now) => subMonths(now, 6),
  "1Y": (now) => subYears(now, 1),
  ALL: (now) => subYears(now, 10),
};

// Theme tokens from globals.css, so the chart follows light/dark mode.
const SERIES_COLOR = "var(--chart-1)";
const AXIS_COLOR = "var(--muted-foreground)";
const GRID_COLOR = "var(--border)";

const formatMonth = (val: string) => new Date(val).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" });
const formatCompact = (val: number) =>
  new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", notation: "compact", maximumFractionDigits: 1 }).format(val / 100);

interface NetWorthTooltipProps {
  active?: boolean;
  payload?: { value?: number | string }[];
  label?: string | number;
}

function NetWorthTooltip({ active, payload, label }: NetWorthTooltipProps) {
  if (!active || !payload?.length) return null;
  return (
    <div className="bg-white dark:bg-slate-800 p-3 rounded-xl shadow-lg border border-slate-200 dark:border-slate-700/60">
      <p className="text-slate-500 dark:text-slate-400 text-xs mb-1 font-medium">{formatMonth(String(label))}</p>
      <p className="font-bold text-slate-900 dark:text-white font-heading">{formatCurrency(Number(payload[0].value))}</p>
    </div>
  );
}

export function NetWorthCard() {
  const token = getStoredToken();
  const [range, setRange] = useState<Range>("6M");

  const params = useMemo(() => {
    const now = new Date();
    return new URLSearchParams({
      start_date: format(RANGE_START[range](now), "yyyy-MM-dd"),
      end_date: format(now, "yyyy-MM-dd"),
    }).toString();
  }, [range]);

  const { data: points, isLoading } = useQuery<NetWorthDataPoint[]>({
    queryKey: ["reports", "net-worth", range],
    queryFn: () => apiFetch<NetWorthDataPoint[]>(`/reports/net-worth?${params}`, {}, token),
  });

  const { data: accounts } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => apiFetch<Account[]>("/accounts", {}, token),
  });

  const caption = useMemo(() => {
    const netWorthAccounts = (accounts ?? []).filter((a) => a.type === "asset" || a.type === "liability");
    const base = `Based on ${netWorthAccounts.length} account${netWorthAccounts.length === 1 ? "" : "s"}`;
    const asOfDates = netWorthAccounts.map((a) => a.balance_as_of).filter((d): d is string => !!d);
    if (asOfDates.length === 0) return base;
    const oldest = asOfDates.reduce((min, d) => (d < min ? d : min));
    return `${base} · balances as of ${formatDate(oldest)}`;
  }, [accounts]);

  const stats = useMemo(() => {
    if (!points?.length) return null;
    const current = points[points.length - 1].net_worth;
    if (points.length < 2) return { current, delta: null, deltaPercent: null };
    const start = points[0].net_worth;
    const delta = current - start;
    return { current, delta, deltaPercent: start !== 0 ? (delta / Math.abs(start)) * 100 : null };
  }, [points]);

  // Center the y-axis on the starting value so small moves are still visible.
  const yDomain = useMemo<[number, number] | ["auto", "auto"]>(() => {
    if (!points?.length) return ["auto", "auto"];
    const first = points[0].net_worth;
    const values = points.map((p) => p.net_worth);
    const maxDiff = Math.max(
      Math.abs(Math.max(...values) - first),
      Math.abs(Math.min(...values) - first),
      Math.abs(first * 0.1),
      10000
    );
    return [first - maxDiff * 1.1, first + maxDiff * 1.1];
  }, [points]);

  const rangeToggle = (
    <div className="flex bg-slate-100 dark:bg-slate-800/80 p-1 rounded-lg">
      {RANGES.map((r) => (
        <button
          key={r}
          onClick={() => setRange(r)}
          aria-pressed={range === r}
          className={`px-2.5 sm:px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
            range === r
              ? "bg-white dark:bg-slate-700 text-indigo-600 dark:text-indigo-400 shadow-sm"
              : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
          }`}
        >
          {r}
        </button>
      ))}
    </div>
  );

  return (
    <StatCard title="Net Worth" icon={TrendingUp} action={rangeToggle} wrapHeader>
      <div className="mb-6">
        {isLoading ? (
          <Skeleton className="h-9 w-48" />
        ) : stats ? (
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <span className="text-3xl font-bold font-heading text-slate-900 dark:text-white">{formatCurrency(stats.current)}</span>
            {stats.delta !== null && (
              <span className={`text-sm font-medium ${stats.delta >= 0 ? "text-emerald-600 dark:text-emerald-500" : "text-rose-600 dark:text-rose-500"}`}>
                {stats.delta >= 0 ? "+" : ""}
                {formatCurrency(stats.delta)}
                {stats.deltaPercent !== null && ` (${stats.deltaPercent >= 0 ? "+" : ""}${stats.deltaPercent.toFixed(1)}%)`}
              </span>
            )}
          </div>
        ) : null}
        <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">{caption}</p>
      </div>

      <div className="h-72 w-full" role="img" aria-label="Net worth over time">
        {isLoading ? (
          <Skeleton className="h-full w-full" />
        ) : points?.length ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={points} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="netWorthFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" style={{ stopColor: SERIES_COLOR, stopOpacity: 0.3 }} />
                  <stop offset="95%" style={{ stopColor: SERIES_COLOR, stopOpacity: 0 }} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" vertical={false} stroke={GRID_COLOR} />
              <XAxis
                dataKey="month"
                tickFormatter={formatMonth}
                stroke={GRID_COLOR}
                tickLine={false}
                dy={10}
                tick={{ fill: AXIS_COLOR, fontSize: 12 }}
                minTickGap={30}
                padding={{ left: 10, right: 10 }}
              />
              <YAxis
                domain={yDomain}
                tickFormatter={formatCompact}
                stroke={GRID_COLOR}
                tickLine={false}
                width={65}
                tick={{ fill: AXIS_COLOR, fontSize: 12 }}
              />
              <Tooltip content={<NetWorthTooltip />} cursor={{ stroke: AXIS_COLOR, strokeWidth: 1, strokeDasharray: "4 4" }} />
              <Area
                type="monotone"
                dataKey="net_worth"
                stroke={SERIES_COLOR}
                strokeWidth={3}
                fill="url(#netWorthFill)"
                dot={false}
                activeDot={{ r: 6, fill: SERIES_COLOR, stroke: "var(--card)", strokeWidth: 2 }}
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div className="flex flex-col h-full items-center justify-center text-slate-500 gap-3">
            <Activity className="w-12 h-12 text-slate-300 dark:text-slate-700" />
            <p className="text-sm font-medium">No net worth data available</p>
          </div>
        )}
      </div>
    </StatCard>
  );
}
