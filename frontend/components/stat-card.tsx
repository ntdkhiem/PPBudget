import * as React from "react"
import { LucideIcon } from "lucide-react"
import { cn } from "@/lib/utils"
import { DashboardCard } from "@/components/dashboard-card"

const TONES = {
  indigo: "bg-indigo-50 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400",
  emerald: "bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400",
  amber: "bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400",
  rose: "bg-rose-50 text-rose-600 dark:bg-rose-500/10 dark:text-rose-400",
  purple: "bg-purple-50 text-purple-600 dark:bg-purple-500/10 dark:text-purple-400",
} as const;

export interface StatCardProps {
  title: string;
  icon: LucideIcon;
  tone?: keyof typeof TONES;
  /** Right side of the header: a caption or a link. */
  action?: React.ReactNode;
  /** Let a wide action drop below the title on narrow screens instead of truncating the title. */
  wrapHeader?: boolean;
  children: React.ReactNode;
  className?: string;
}

export function StatCard({ title, icon: Icon, tone = "indigo", action, wrapHeader = false, children, className }: StatCardProps) {
  return (
    <DashboardCard className={cn("min-w-0 flex flex-col", className)}>
      <div className={cn("flex items-center justify-between gap-3 mb-4", wrapHeader && "flex-wrap")}>
        <div className="flex items-center gap-3 min-w-0">
          <div className={cn("p-2 rounded-xl shrink-0", TONES[tone])}>
            <Icon className="w-5 h-5" />
          </div>
          <h2 className="text-base font-semibold font-heading text-slate-900 dark:text-white truncate">{title}</h2>
        </div>
        {action && <div className="shrink-0 text-xs font-medium text-slate-500 dark:text-slate-400">{action}</div>}
      </div>
      <div className="flex flex-col flex-1">{children}</div>
    </DashboardCard>
  )
}
