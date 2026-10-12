import { addDays, addMonths, addWeeks, addYears, format } from "date-fns";
import type { Subscription } from "@/lib/api";

export type BillStatus = "overdue" | "due" | "paid";

export interface BillDue {
  sub: Subscription;
  /** yyyy-MM-dd */
  due: string;
  status: BillStatus;
}

const CYCLE_DAYS: Record<Subscription["billing_cycle"], number> = { weekly: 7, monthly: 30, yearly: 365 };

/** A calendar date ("2026-10-05" or "2026-10-05T00:00:00Z") as local midnight, so it never shifts a day. */
function parseDay(iso: string): Date {
  const [y, m, d] = iso.slice(0, 10).split("-").map(Number);
  return new Date(y, m - 1, d);
}

/** The nth billing date after the stored one, stepped from it in one go so month ends don't drift. */
function occurrence(anchor: Date, cycle: Subscription["billing_cycle"], n: number): Date {
  if (cycle === "weekly") return addWeeks(anchor, n);
  if (cycle === "yearly") return addYears(anchor, n);
  return addMonths(anchor, n);
}

/**
 * Whether the latest payment covers the bill due on `due`: a payment counts
 * toward the due date it is nearer to, so anything from half a cycle before
 * it on. Autopay that posts a day or two early still counts.
 */
function isPaid(sub: Subscription, due: Date): boolean {
  if (!sub.last_paid_date) return false;
  return parseDay(sub.last_paid_date) >= addDays(due, -Math.floor(CYCLE_DAYS[sub.billing_cycle] / 2));
}

/**
 * Bills to show on the dashboard: every occurrence from today through `days`
 * ahead, plus the current bill when its date has passed unpaid. Overdue first,
 * then by date.
 *
 * The server keeps a subscription's date on its current cycle (this month's
 * bill all month, this week's all week), so the next occurrence often isn't
 * stored yet and has to be projected from it here.
 */
export function billsDue(subs: Subscription[], today: Date, days = 7): BillDue[] {
  const start = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const end = addDays(start, days);
  const bills: BillDue[] = [];

  for (const sub of subs) {
    const anchor = parseDay(sub.next_billing_date);
    if (anchor < start && !isPaid(sub, anchor)) {
      bills.push({ sub, due: format(anchor, "yyyy-MM-dd"), status: "overdue" });
    }
    // Bounded: a stored date is at most one cycle old, but never loop forever on bad data.
    for (let n = 0; n < 1000; n++) {
      const due = occurrence(anchor, sub.billing_cycle, n);
      if (due > end) break;
      if (due >= start) {
        bills.push({ sub, due: format(due, "yyyy-MM-dd"), status: isPaid(sub, due) ? "paid" : "due" });
      }
    }
  }

  const rank = (b: BillDue) => (b.status === "overdue" ? 0 : 1);
  return bills.sort((a, b) => rank(a) - rank(b) || a.due.localeCompare(b.due) || a.sub.name.localeCompare(b.sub.name));
}
