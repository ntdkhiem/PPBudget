import * as React from "react"
import { cn } from "@/lib/utils"
import { Slot } from "@radix-ui/react-slot"

export interface DashboardCardProps extends React.HTMLAttributes<HTMLDivElement> {
  asChild?: boolean;
}

export function DashboardCard({ children, className, asChild = false, ...props }: DashboardCardProps) {
  const Comp = asChild ? Slot : "div";
  return (
    <Comp 
      className={cn(
        "bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60",
        className
      )}
      {...props}
    >
      {children}
    </Comp>
  )
}
