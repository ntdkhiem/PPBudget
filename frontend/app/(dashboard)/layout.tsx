"use client";

import { useRouter, usePathname } from "next/navigation";
import { useEffect, useState, ReactNode } from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { LayoutDashboard, ReceiptText, ListChecks, PieChart, Settings, SlidersHorizontal, LogOut, Wallet, Database } from "lucide-react";
import { DatePickerWithRange } from "@/components/ui/date-range-picker";
import { useDateRange } from "@/app/contexts/DateRangeContext";
import { GlobalSearch } from "@/components/global-search";
export default function DashboardLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [isAuthed, setIsAuthed] = useState(false);
  const { date, setDate } = useDateRange();

  useEffect(() => {
    const token = localStorage.getItem("ppbudget_token");
    if (!token) {
      router.push("/login");
    } else {
      setIsAuthed(true);
    }
  }, [router]);

  if (!isAuthed) return null;

  const handleLogout = () => {
    localStorage.removeItem("ppbudget_token");
    router.push("/login");
  };

  const navItems = [
    { href: "/", label: "Dashboard", icon: LayoutDashboard },
    { href: "/accounts", label: "Accounts", icon: Wallet },
    { href: "/transactions", label: "Transactions", icon: ReceiptText },
    { href: "/budgets", label: "Budgets", icon: PieChart },
    { href: "/settings/categories", label: "Categories", icon: Settings },
    { href: "/settings/rules", label: "Rules", icon: SlidersHorizontal },
    { href: "/settings/importer", label: "Data Importer", icon: Database },
  ];

  return (
    <div className="flex min-h-screen bg-slate-50 dark:bg-slate-950 font-sans">
      <aside className="w-64 bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl border-r border-slate-200/60 dark:border-slate-800/60 p-4 flex flex-col relative z-10 shadow-sm">
        <div className="flex items-center gap-3 mb-10 pl-2">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center text-white font-bold text-lg shadow-lg shadow-indigo-500/30">
            F
          </div>
          <h2 className="text-2xl font-bold font-heading bg-clip-text text-transparent bg-gradient-to-r from-slate-800 to-slate-600 dark:from-slate-100 dark:to-slate-300">PPBudget</h2>
        </div>
        
        <div className="mb-6">
          <GlobalSearch />
          <DatePickerWithRange date={date} setDate={setDate} className="w-full" />
        </div>

        <nav className="flex flex-col gap-2 flex-1">
          {navItems.map((item) => {
            const isActive = pathname === item.href;
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                className="relative px-4 py-3 rounded-xl transition-all duration-200 group flex items-center gap-3 text-sm font-medium"
              >
                {isActive && (
                  <motion.div
                    layoutId="activeNav"
                    className="absolute inset-0 bg-indigo-50 dark:bg-indigo-500/10 rounded-xl"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  />
                )}
                {isActive && (
                  <motion.div 
                    layoutId="activeNavBorder"
                    className="absolute left-0 top-1/4 bottom-1/4 w-1 bg-indigo-600 rounded-r-full"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.2 }}
                  />
                )}
                <Icon className={`relative z-10 h-5 w-5 ${isActive ? "text-indigo-600" : "text-slate-500 group-hover:text-indigo-500 transition-colors"}`} />
                <span className={`relative z-10 ${isActive ? "text-indigo-700 dark:text-indigo-300 font-semibold" : "text-slate-600 dark:text-slate-400 group-hover:text-slate-900 dark:group-hover:text-slate-200"}`}>
                  {item.label}
                </span>
              </Link>
            );
          })}
        </nav>
        
        <div className="mt-auto pt-8 border-t border-slate-200/60 dark:border-slate-800/60">
          <button 
            onClick={handleLogout}
            className="w-full flex items-center gap-3 px-4 py-3 text-sm font-medium text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-500/10 rounded-xl transition-colors"
          >
            <LogOut className="h-5 w-5" />
            Sign out
          </button>
        </div>
      </aside>
      
      <main className="flex-1 p-6 lg:p-8 overflow-y-auto bg-[radial-gradient(ellipse_at_top_right,_var(--tw-gradient-stops))] from-indigo-50/40 via-transparent to-transparent">
        <div className="max-w-7xl mx-auto">
          {children}
        </div>
      </main>
    </div>
  );
}
