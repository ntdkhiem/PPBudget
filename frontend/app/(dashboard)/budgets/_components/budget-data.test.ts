import { describe, expect, it } from "vitest";
import type { BudgetSummary, Category } from "@/lib/api";
import { sortByUsage, toBudgetCards, type BudgetCardData } from "./budget-data";

function card(catName: string, limit: number, spent: number): BudgetCardData {
  return {
    catId: catName, catName, budgetId: catName, limit, spent, left: limit - spent,
    spentPerDay: 0, leftPerDay: 0, bucket: "needs", periodType: "monthly",
  };
}

describe("sortByUsage", () => {
  it("puts the budgets closest to their limit first, over-limit ahead of all", () => {
    const sorted = sortByUsage([
      card("Dining", 25_000, 5_000), // 20%
      card("Rent", 180_000, 180_000), // 100%
      card("Groceries", 50_000, 62_000), // 124%
      card("Gas", 10_000, 9_000), // 90%
    ]);
    expect(sorted.map((c) => c.catName)).toEqual(["Groceries", "Rent", "Gas", "Dining"]);
  });

  it("ranks spending against a zero limit first and an unused zero limit last", () => {
    const sorted = sortByUsage([card("Unused", 0, 0), card("Half", 100, 50), card("Zero limit", 0, 10)]);
    expect(sorted.map((c) => c.catName)).toEqual(["Zero limit", "Half", "Unused"]);
  });

  it("breaks ties by name and leaves its input alone", () => {
    const input = [card("b", 100, 50), card("a", 200, 100)];
    expect(sortByUsage(input).map((c) => c.catName)).toEqual(["a", "b"]);
    expect(input.map((c) => c.catName)).toEqual(["b", "a"]);
  });
});

describe("toBudgetCards", () => {
  it("takes the server's figures and drops budgets whose category is gone", () => {
    const budgets = [
      { id: "b1", name: "x", category_id: "c1", amount_cents: 50_000, period_type: "one_time", start_date: "", end_date: "",
        bucket: "wants", spent_total: 21_000, spent_per_day: 1_750, left_total: 29_000, left_per_day: 1_450 },
      { id: "b2", name: "y", category_id: "gone", amount_cents: 1, period_type: "monthly", start_date: "", end_date: "",
        spent_total: 0, spent_per_day: 0, left_total: 1, left_per_day: 0 },
    ] as BudgetSummary[];
    const categories = [{ id: "c1", name: "Groceries" }] as Category[];

    expect(toBudgetCards(budgets, categories)).toEqual([{
      catId: "c1", catName: "Groceries", budgetId: "b1", limit: 50_000, spent: 21_000, left: 29_000,
      spentPerDay: 1_750, leftPerDay: 1_450, bucket: "wants", periodType: "one_time",
    }]);
  });
});
