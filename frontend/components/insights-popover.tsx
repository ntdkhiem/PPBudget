"use client";

import * as React from "react"
import { Bell, AlertTriangle, PlusCircle, Inbox, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Insight } from "@/lib/api"
import { motion, AnimatePresence } from "framer-motion"
import Link from "next/link"

interface InsightsPopoverProps {
  insights: Insight[];
  onDismiss: (id: string) => void;
}

const formatDescription = (text: string) => {
  return text.split(' ').map((word, i) => {
    if (word.startsWith('$') || word.includes('%') || !isNaN(Number(word.replace(/[^0-9.-]+/g,"")))) {
      return <span key={i} className="font-semibold text-slate-900 dark:text-white">{word} </span>;
    }
    return word + ' ';
  });
};

export function InsightsPopover({ insights, onDismiss }: InsightsPopoverProps) {
  const hasInsights = insights.length > 0;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button 
          variant="outline"
          size="icon" 
          className="relative rounded-xl border-slate-200 dark:border-slate-800/60 bg-white/60 dark:bg-slate-900/60 backdrop-blur-xl shadow-sm hover:shadow-md transition-all h-10 w-10" 
          aria-label="Insights & Alerts"
        >
          <Bell className="h-5 w-5 text-slate-700 dark:text-slate-300" />
          
          {/* Notification Badge with Pulse Animation */}
          {hasInsights && (
            <span className="absolute -top-1 -right-1 flex h-3.5 w-3.5">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-rose-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-3.5 w-3.5 bg-rose-500 border-2 border-white dark:border-slate-900"></span>
            </span>
          )}
        </Button>
      </PopoverTrigger>
      
      <PopoverContent className="w-80 md:w-96 p-0 rounded-2xl overflow-hidden border border-slate-200 dark:border-slate-700/60 shadow-xl" align="end">
        <div className="bg-slate-50 dark:bg-slate-800/50 p-4 border-b border-slate-100 dark:border-slate-700/60 flex items-center justify-between">
          <div>
            <h4 className="font-bold text-slate-900 dark:text-white font-heading">Insights & Alerts</h4>
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              {hasInsights ? `You have ${insights.length} active updates` : "You're all caught up!"}
            </p>
          </div>
        </div>
        
        <div className="max-h-[350px] overflow-y-auto p-3 space-y-2 bg-white dark:bg-slate-900">
          <AnimatePresence>
            {hasInsights ? (
              insights.map((insight) => {
                const isHigh = insight.severity === 'high';
                const isMed = insight.severity === 'medium';

                const borderColor = isHigh ? 'border-l-rose-500' : isMed ? 'border-l-amber-500' : 'border-l-blue-500';
                const Icon = insight.type === 'anomaly' ? AlertTriangle : insight.type === 'subscription' ? PlusCircle : Inbox;
                const iconColor = isHigh ? 'text-rose-500' : isMed ? 'text-amber-500' : 'text-blue-500';
                const iconBg = isHigh ? 'bg-rose-100 dark:bg-rose-500/20' : isMed ? 'bg-amber-100 dark:bg-amber-500/20' : 'bg-blue-100 dark:bg-blue-500/20';

                return (
                  <motion.div
                    key={insight.id}
                    layout
                    initial={{ opacity: 0, scale: 0.95 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, height: 0, marginBottom: 0, scale: 0.9 }}
                    className={`group relative flex flex-col p-3 bg-slate-50/50 dark:bg-slate-800/30 rounded-xl border-l-4 ${borderColor} border-y border-r border-slate-100 dark:border-y-slate-700 dark:border-r-slate-700`}
                  >
                    {insight.dismissable && (
                      <button
                        onClick={() => onDismiss(insight.id)}
                        className="absolute top-2 right-2 p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 opacity-0 group-hover:opacity-100 transition-opacity rounded-full hover:bg-slate-200 dark:hover:bg-slate-700"
                        title="Dismiss"
                      >
                        <X className="w-4 h-4" />
                      </button>
                    )}

                    <div className="flex items-start gap-3">
                      <div className={`mt-0.5 p-2 rounded-full flex-shrink-0 ${iconBg} ${iconColor}`}>
                        <Icon className="w-4 h-4" />
                      </div>
                      <div className="flex-1 pr-6">
                        <h4 className="text-sm font-bold text-slate-900 dark:text-white mb-1">{insight.title}</h4>
                        <p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed mb-2">
                          {formatDescription(insight.description)}
                        </p>
                        {insight.action_url && (
                          <Link href={insight.action_url} className="mt-1 inline-flex items-center px-3 py-1.5 text-xs font-semibold bg-slate-900 text-white dark:bg-white dark:text-slate-900 rounded-lg hover:opacity-90 transition-opacity">
                            Resolve Action
                          </Link>
                        )}
                      </div>
                    </div>
                  </motion.div>
                );
              })
            ) : (
              <div className="flex flex-col items-center justify-center py-8 text-center px-4">
                <div className="w-12 h-12 rounded-full bg-emerald-50 dark:bg-emerald-500/10 flex items-center justify-center mb-3">
                  <Inbox className="w-6 h-6 text-emerald-500" />
                </div>
                <p className="text-sm font-medium text-slate-900 dark:text-white">Nothing to review!</p>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">Your finances are looking perfectly healthy.</p>
              </div>
            )}
          </AnimatePresence>
        </div>
      </PopoverContent>
    </Popover>
  )
}
