"use client";

import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";
import type { FieldKind } from "./wealth-data";

/**
 * Small form pieces shared by the intake and the grant editor, so the two
 * look and convert the same way.
 */

/** Dollars in the input, cents on the wire. */
export function toCents(v: string): number | null {
  const n = Number.parseFloat(v.replace(/[^0-9.\-]/g, ""));
  return Number.isFinite(n) ? Math.round(n * 100) : null;
}

/** Percent in the input, fraction on the wire. */
export function toFraction(v: string): number | null {
  const n = Number.parseFloat(v.replace(/[^0-9.\-]/g, ""));
  return Number.isFinite(n) ? n / 100 : null;
}

/**
 * Turns a stored answer back into what its input expects.
 *
 * The reverse of the conversions above: cents to dollars, fractions to
 * percent, timestamps to a date input's yyyy-mm-dd, booleans to yes/no.
 * Without this an edit starts from an empty box, which reads as "this was
 * never answered" and invites the user to retype something that was already
 * right.
 */
export function toInputValue(kind: FieldKind, value: unknown): string {
  if (value === undefined || value === null) return "";
  switch (kind) {
    case "boolean":
      return value ? "yes" : "no";
    case "date":
      return typeof value === "string" ? value.slice(0, 10) : "";
    case "percent":
      return typeof value === "number" ? String(Number((value * 100).toFixed(4))) : "";
    case "money":
      return typeof value === "number" ? String(value / 100) : "";
    default:
      return String(value);
  }
}

export function Choice<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T | undefined;
  options: Array<{ value: T; label: string }>;
  onChange: (v: T) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          onClick={() => onChange(opt.value)}
          aria-pressed={value === opt.value}
          className={cn(
            "rounded-xl border px-4 py-2 text-sm font-medium transition-colors",
            value === opt.value
              ? "border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-500/15 dark:text-indigo-300"
              : "border-slate-200 text-slate-600 hover:border-slate-300 dark:border-slate-700 dark:text-slate-300 dark:hover:border-slate-600",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
      {hint && <p className="text-xs text-slate-500 dark:text-slate-400">{hint}</p>}
    </div>
  );
}
