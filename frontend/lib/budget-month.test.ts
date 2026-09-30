import { afterEach, describe, expect, it } from "vitest";
import { budgetMonthOffset, countsTowardLabel, monthLabel, shiftedMonth } from "./budget-month";

describe("budget month helpers", () => {
  const originalTZ = process.env.TZ;
  afterEach(() => {
    if (originalTZ === undefined) delete process.env.TZ;
    else process.env.TZ = originalTZ;
  });

  it("shifts months across a year", () => {
    expect(shiftedMonth("2026-09-28", 1)).toBe("2026-10");
    expect(shiftedMonth("2026-12-29", 1)).toBe("2027-01");
    expect(shiftedMonth("2026-01-03", -1)).toBe("2025-12");
  });

  it("measures the offset from the posted month", () => {
    expect(budgetMonthOffset("2026-09-28", "2026-10-01T00:00:00Z")).toBe(1);
    expect(budgetMonthOffset("2026-12-29T00:00:00Z", "2027-01-01T00:00:00Z")).toBe(1);
    expect(budgetMonthOffset("2026-09-28", null)).toBe(0);
  });

  it("labels the month without slipping a day west of Greenwich", () => {
    process.env.TZ = "America/Los_Angeles";
    expect(countsTowardLabel("2026-09-28T00:00:00Z", "2026-10-01T00:00:00Z")).toBe("Counts toward Oct");
    expect(monthLabel("2026-12-29", 1)).toBe("Jan 2027");
    expect(countsTowardLabel("2026-09-28", undefined)).toBeNull();
  });
});
