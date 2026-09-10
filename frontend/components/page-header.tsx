import * as React from "react"
import { cn } from "@/lib/utils"

interface PageHeaderProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'title'> {
  title: string | React.ReactNode;
  description?: string | React.ReactNode;
  children?: React.ReactNode;
}

export function PageHeader({ title, description, children, className, ...props }: PageHeaderProps) {
  return (
    <div className={cn("flex flex-col md:flex-row md:items-end justify-between gap-4 mb-8", className)} {...props}>
      <div className="flex-1">
        <h1 className="text-4xl font-bold font-heading text-slate-900 dark:text-white tracking-tight">
          {title}
        </h1>
        {description && <p className="text-lg text-slate-500 mt-2">{description}</p>}
      </div>
      {children && <div className="flex shrink-0 gap-3">{children}</div>}
    </div>
  )
}
