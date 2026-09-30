/**
 * The month a transaction counts toward, when it is not the month it posted:
 * rent paid on the 28th for next month. The server stores it as the first of
 * that month (`budget_month`) and leaves the posted date as the bank has it.
 *
 * Dates here are calendar days, read in UTC from their yyyy-mm-dd prefix. A
 * local reading of "2026-10-01T00:00:00Z" west of Greenwich is 30 September,
 * which would put every label a month early.
 */

/** Year and 0-based month of an ISO date or yyyy-mm-dd string. */
function yearMonth(iso: string): { y: number; m: number } {
  const [y, m] = iso.slice(0, 7).split("-").map(Number);
  return { y, m: m - 1 };
}

/** "2026-10" for the month `offset` months on from the posted date's. */
export function shiftedMonth(postedISO: string, offset: number): string {
  const { y, m } = yearMonth(postedISO);
  const d = new Date(Date.UTC(y, m + offset, 1));
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

/** Months from the posted month to the budget month; 0 when there is none. */
export function budgetMonthOffset(postedISO: string, budgetMonth?: string | null): number {
  if (!budgetMonth) return 0;
  const p = yearMonth(postedISO);
  const b = yearMonth(budgetMonth);
  return (b.y - p.y) * 12 + (b.m - p.m);
}

/** "Oct", or "Jan 2027" when the year differs from the posted date's. */
export function monthLabel(postedISO: string, offset: number): string {
  const { y, m } = yearMonth(postedISO);
  const d = new Date(Date.UTC(y, m + offset, 1));
  return d.toLocaleDateString("en-US", {
    month: "short",
    ...(d.getUTCFullYear() !== y ? { year: "numeric" } : {}),
    timeZone: "UTC",
  });
}

/** The badge text for a moved transaction, or null when it counts where it posted. */
export function countsTowardLabel(postedISO: string, budgetMonth?: string | null): string | null {
  const offset = budgetMonthOffset(postedISO, budgetMonth);
  return offset === 0 ? null : `Counts toward ${monthLabel(postedISO, offset)}`;
}
