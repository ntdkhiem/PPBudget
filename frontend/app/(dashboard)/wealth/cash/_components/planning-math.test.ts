import { afterEach, describe, expect, it } from "vitest";
import type { PlanningMonth } from "@/lib/api";
import {
  activeMonths,
  completeMonths,
  cushionCents,
  emergencyFundMonths,
  formatMonthYear,
  formatMonths,
  type Baseline,
} from "./planning-math";

// The formatting, month and cushion helpers. Goals are scheduled on the server.

const month = (m: number, active: boolean): PlanningMonth => ({
  month: `2026-${String(m).padStart(2, "0")}-01T00:00:00Z`,
  income: active ? 600_000 : 0,
  outflow: active ? 450_000 : 0,
  needs: 0,
  wants: 0,
  savings: 0,
  unbucketed: 0,
  net_worth: 0,
});
const names = (months: PlanningMonth[]) => months.map((m) => m.month.slice(5, 7));

describe("formatMonthYear", () => {
  const originalTZ = process.env.TZ;
  afterEach(() => {
    if (originalTZ === undefined) delete process.env.TZ;
    else process.env.TZ = originalTZ;
  });

  it("names the month the server dated", () => {
    expect(formatMonthYear("2027-04-01T00:00:00Z")).toBe("April 2027");
    expect(formatMonthYear("2026-12-01T00:00:00Z")).toBe("December 2026");
  });

  it("does not slip to the month before west of Greenwich", () => {
    process.env.TZ = "America/Los_Angeles";
    const iso = "2027-04-01T00:00:00Z";
    // Guard: the zone really applied, and a local reading would be wrong.
    expect(new Date(iso).getTimezoneOffset()).toBeGreaterThan(0);
    expect(new Date(iso).toLocaleDateString("en-US", { month: "long", year: "numeric" })).toBe(
      "March 2027",
    );
    expect(formatMonthYear(iso)).toBe("April 2027");
  });

  it("is a dash for no date", () => {
    expect(formatMonthYear(null)).toBe("—");
  });
});

describe("formatMonths", () => {
  it("reads as months, then years", () => {
    expect(formatMonths(null)).toBe("—");
    expect(formatMonths(0)).toBe("now");
    expect(formatMonths(-2)).toBe("now");
    expect(formatMonths(7)).toBe("7 mo");
    expect(formatMonths(12)).toBe("1 yr");
    expect(formatMonths(14)).toBe("1 yr 2 mo");
  });
});

describe("activeMonths", () => {
  it("drops months with no activity at all", () => {
    expect(names(activeMonths([month(6, false), month(7, true), month(8, true)]))).toEqual([
      "07",
      "08",
    ]);
  });
});

describe("completeMonths", () => {
  const now = new Date("2026-09-24T18:00:00Z");

  it("drops the month under way", () => {
    expect(names(completeMonths([month(7, true), month(8, true), month(9, true)], now))).toEqual([
      "07",
      "08",
    ]);
  });

  it("keeps a finished month when nothing has posted yet this month", () => {
    expect(names(completeMonths([month(7, true), month(8, true), month(9, false)], now))).toEqual([
      "07",
      "08",
    ]);
  });

  it("keeps the last finished months after syncing stopped", () => {
    const months = [month(5, true), month(6, true), month(7, false), month(8, false), month(9, false)];
    expect(names(completeMonths(months, now))).toEqual(["05", "06"]);
  });

  it("keeps this month when it is all there is", () => {
    expect(names(completeMonths([month(8, false), month(9, true)], now))).toEqual(["09"]);
  });
});

describe("cushionCents", () => {
  const baseline = (liquid: number, earmarked: number) =>
    ({ liquidAssets: liquid, earmarkedCents: earmarked, essentialMonthly: 400_000 }) as Baseline;

  it("counts liquid cash less what planned expenses hold back", () => {
    expect(cushionCents(baseline(2_000_000, 500_000))).toBe(1_500_000);
    expect(emergencyFundMonths(baseline(2_000_000, 500_000))).toBe(3.75);
  });

  it("never goes below zero when more is earmarked than held", () => {
    expect(cushionCents(baseline(300_000, 500_000))).toBe(0);
  });
});
