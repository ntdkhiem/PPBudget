import type { BudgetSummary, Category } from "@/lib/api";

export type Bucket = NonNullable<BudgetSummary["bucket"]>;

/** One budget as the page shows it. Money is in cents, straight from the server. */
export interface BudgetCardData {
  catId: string;
  catName: string;
  budgetId: string;
  limit: number;
  spent: number;
  /** limit - spent; negative when over. */
  left: number;
  spentPerDay: number;
  leftPerDay: number;
  bucket: Bucket;
  periodType: string;
}

export const BUCKETS: { key: Bucket; label: string; goalPercent: number }[] = [
  { key: "needs", label: "Needs", goalPercent: 50 },
  { key: "wants", label: "Wants", goalPercent: 30 },
  { key: "savings", label: "Savings", goalPercent: 20 },
];

/** Joins each budget to its category's name; budgets whose category is gone are dropped. */
export function toBudgetCards(budgets: BudgetSummary[], categories: Category[]): BudgetCardData[] {
  const names = new Map(categories.map((c) => [c.id, c.name]));
  return budgets.flatMap((b) => {
    const catName = names.get(b.category_id);
    if (catName === undefined) return [];
    return [{
      catId: b.category_id,
      catName,
      budgetId: b.id,
      limit: b.amount_cents,
      spent: b.spent_total,
      left: b.left_total,
      spentPerDay: b.spent_per_day,
      leftPerDay: b.left_per_day,
      bucket: b.bucket ?? "needs",
      periodType: b.period_type,
    }];
  });
}

/** How much of its limit a budget has used; spending against a zero limit sorts first. */
function usage(card: BudgetCardData): number {
  if (card.limit > 0) return card.spent / card.limit;
  return card.spent > 0 ? Infinity : 0;
}

/** Closest to (or furthest past) the limit first; ties by name. */
export function sortByUsage(cards: BudgetCardData[]): BudgetCardData[] {
  return [...cards].sort((a, b) => usage(b) - usage(a) || a.catName.localeCompare(b.catName));
}
