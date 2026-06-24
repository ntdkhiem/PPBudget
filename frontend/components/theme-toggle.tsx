"use client";

import * as React from "react";
import { Moon, Sun, Monitor } from "lucide-react";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  // Avoid hydration mismatch by waiting for mount
  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return (
      <div className="flex bg-slate-100 dark:bg-slate-800 rounded-lg p-1 w-full justify-between items-center opacity-0">
        <div className="h-8 w-8" />
      </div>
    );
  }

  return (
    <div className="flex bg-slate-200/50 dark:bg-slate-800/50 rounded-lg p-1 w-full justify-between items-center shadow-inner">
      <button
        onClick={() => setTheme("light")}
        className={`flex-1 flex justify-center py-2 rounded-md transition-all duration-200 ${
          theme === "light"
            ? "bg-white dark:bg-slate-700 text-indigo-600 shadow-sm"
            : "text-slate-500 hover:text-slate-700 dark:text-slate-300 dark:hover:text-slate-300"
        }`}
        aria-label="Light mode"
      >
        <Sun className="h-4 w-4" />
      </button>
      <button
        onClick={() => setTheme("system")}
        className={`flex-1 flex justify-center py-2 rounded-md transition-all duration-200 ${
          theme === "system"
            ? "bg-white dark:bg-slate-700 text-indigo-600 shadow-sm"
            : "text-slate-500 hover:text-slate-700 dark:text-slate-300 dark:hover:text-slate-300"
        }`}
        aria-label="System default mode"
      >
        <Monitor className="h-4 w-4" />
      </button>
      <button
        onClick={() => setTheme("dark")}
        className={`flex-1 flex justify-center py-2 rounded-md transition-all duration-200 ${
          theme === "dark"
            ? "bg-white dark:bg-slate-700 text-indigo-600 shadow-sm"
            : "text-slate-500 hover:text-slate-700 dark:text-slate-300 dark:hover:text-slate-300"
        }`}
        aria-label="Dark mode"
      >
        <Moon className="h-4 w-4" />
      </button>
    </div>
  );
}
