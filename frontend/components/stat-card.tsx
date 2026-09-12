"use client";

import * as React from "react"
import { motion } from "framer-motion"
import { LucideIcon } from "lucide-react"
import { cn } from "@/lib/utils"

export interface StatCardProps {
  title: string;
  icon: LucideIcon;
  iconColorClass?: string;
  iconBgClass?: string;
  delay?: number;
  children: React.ReactNode;
  className?: string;
}

export function StatCard({ 
  title, 
  icon: Icon, 
  iconColorClass = "text-indigo-600 dark:text-indigo-400", 
  iconBgClass = "bg-indigo-50 dark:bg-indigo-500/10",
  delay = 0,
  children,
  className
}: StatCardProps) {
  return (
    <motion.div 
      initial={{ opacity: 0, y: 10 }} 
      animate={{ opacity: 1, y: 0 }} 
      transition={{ delay }} 
      className={cn(
        "bg-white dark:bg-slate-900/80 backdrop-blur-xl p-6 rounded-3xl shadow-sm border border-slate-200 dark:border-slate-700/60 dark:border-slate-800/60 relative overflow-hidden group min-w-0 flex flex-col",
        className
      )}
    >
      <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity pointer-events-none">
        <Icon className={cn("w-16 h-16", iconColorClass)} />
      </div>
      <div className="flex items-center gap-3 mb-4 relative z-10 flex-shrink-0">
        <div className={cn("p-2 rounded-xl", iconBgClass, iconColorClass)}>
          <Icon className="w-5 h-5" />
        </div>
        <h3 className="text-sm font-medium text-slate-500 dark:text-slate-400 font-heading uppercase">
          {title}
        </h3>
      </div>
      <div className="relative z-10 flex flex-col flex-1">
        {children}
      </div>
    </motion.div>
  )
}
