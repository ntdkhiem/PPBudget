"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { motion } from "framer-motion";
import { Compass, ListChecks, PiggyBank, UserCog } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * The Wealth Strategy shell.
 *
 * Four surfaces that used to be two disconnected pages plus nothing. Cash Plan
 * and Retirement keep their own detail views but now sit under one plan rather
 * than each maintaining a separate idea of the user's income and surplus.
 *
 * The active-tab treatment deliberately mirrors the sidebar in
 * app/(dashboard)/layout.tsx -- same shared-layout animation, same indigo
 * accent -- so moving between the two levels of navigation feels like one
 * system. The layoutId differs because two independent groups animating under
 * the same id would fight over the indicator.
 */

type Tab = {
  href: string;
  label: string;
  icon: typeof Compass;
  /** Index tabs match exactly; the others own their whole subtree. */
  exact?: boolean;
};

const TABS: Tab[] = [
  {
    href: "/wealth",
    label: "Overview",
    icon: ListChecks,
    exact: true,
  },
  { href: "/wealth/cash", label: "Cash", icon: Compass },
  { href: "/wealth/retirement", label: "Retirement", icon: PiggyBank },
  { href: "/wealth/profile", label: "Profile", icon: UserCog },
];

export default function WealthLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="w-full">
      <nav
        aria-label="Wealth strategy sections"
        className="mb-8 flex gap-1 overflow-x-auto rounded-2xl border border-slate-200 bg-white/70 p-1.5 backdrop-blur dark:border-slate-800 dark:bg-slate-900/60"
      >
        {TABS.map((tab) => {
          const isActive = tab.exact
            ? pathname === tab.href
            : pathname === tab.href || pathname.startsWith(`${tab.href}/`);
          const Icon = tab.icon;

          return (
            <Link
              key={tab.href}
              href={tab.href}
              aria-current={isActive ? "page" : undefined}
              className="relative flex shrink-0 items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-medium transition-colors"
            >
              {isActive && (
                <motion.div
                  layoutId="activeWealthTab"
                  className="absolute inset-0 rounded-xl bg-indigo-50 dark:bg-indigo-500/10"
                  transition={{ type: "spring", stiffness: 380, damping: 30 }}
                />
              )}
              <Icon
                className={cn(
                  "relative z-10 h-4 w-4",
                  isActive
                    ? "text-indigo-600 dark:text-indigo-400"
                    : "text-slate-400 group-hover:text-indigo-500",
                )}
              />
              <span
                className={cn(
                  "relative z-10",
                  isActive
                    ? "font-semibold text-indigo-700 dark:text-indigo-300"
                    : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200",
                )}
              >
                {tab.label}
              </span>
            </Link>
          );
        })}
      </nav>

      {children}
    </div>
  );
}
