import { afterEach, describe, expect, it } from "vitest";
import type { Subscription } from "@/lib/api";
import { billsDue } from "./upcoming-bills";

function sub(name: string, cycle: Subscription["billing_cycle"], next: string, lastPaid: string | null = null): Subscription {
  return { id: name, name, amount: 1000, billing_cycle: cycle, next_billing_date: `${next}T00:00:00Z`, last_paid_date: lastPaid ? `${lastPaid}T00:00:00Z` : null };
}

const due = (subs: Subscription[], today: Date) =>
  billsDue(subs, today).map((b) => `${b.sub.name} ${b.due} ${b.status}`);

describe("billsDue", () => {
  const oct28 = new Date(2026, 9, 28, 15, 30);

  it("projects next month's bill the server hasn't rolled to yet", () => {
    // Rent stays stored as Oct 1 all October; Nov 1 is inside the week ahead.
    expect(due([sub("Rent", "monthly", "2026-10-01", "2026-09-30")], oct28)).toEqual(["Rent 2026-11-01 due"]);
  });

  it("keeps a past-due unpaid bill as overdue, ahead of upcoming ones", () => {
    expect(due([sub("Rent", "monthly", "2026-10-01", "2026-09-30"), sub("Phone", "monthly", "2026-10-05")], oct28))
      .toEqual(["Phone 2026-10-05 overdue", "Rent 2026-11-01 due"]);
  });

  it("drops a past-due bill once a payment for it is linked", () => {
    expect(due([sub("Phone", "monthly", "2026-10-05", "2026-10-04")], oct28)).toEqual([]);
  });

  it("does not count last cycle's payment toward this one", () => {
    expect(due([sub("Phone", "monthly", "2026-10-05", "2026-09-05")], oct28)).toEqual(["Phone 2026-10-05 overdue"]);
  });

  it("handles weekly bills: this week's overdue and next week's due", () => {
    expect(due([sub("Gym", "weekly", "2026-10-23")], oct28)).toEqual(["Gym 2026-10-23 overdue", "Gym 2026-10-30 due"]);
    expect(due([sub("Gym", "weekly", "2026-10-23", "2026-10-23")], oct28)).toEqual(["Gym 2026-10-30 due"]);
  });

  it("marks a bill paid early as paid", () => {
    expect(due([sub("Insurance", "monthly", "2026-11-03", "2026-10-27")], oct28)).toEqual(["Insurance 2026-11-03 paid"]);
  });

  it("treats a bill due today as due, not overdue", () => {
    expect(due([sub("Water", "monthly", "2026-10-28")], oct28)).toEqual(["Water 2026-10-28 due"]);
  });

  it("includes yearly bills and leaves out anything past the window", () => {
    expect(due([sub("Domain", "yearly", "2026-11-02"), sub("Later", "monthly", "2026-11-05")], oct28))
      .toEqual(["Domain 2026-11-02 due"]);
  });

  it("steps month-end dates from the stored date without drifting", () => {
    const feb25 = new Date(2027, 1, 25);
    expect(due([sub("Loan", "monthly", "2027-01-31", "2027-01-31")], feb25)).toEqual(["Loan 2027-02-28 due"]);
  });

  describe("west of UTC", () => {
    const originalTZ = process.env.TZ;
    afterEach(() => {
      if (originalTZ === undefined) delete process.env.TZ;
      else process.env.TZ = originalTZ;
    });

    it("reads stored dates as calendar days, not UTC midnight", () => {
      process.env.TZ = "America/Los_Angeles";
      // Read as an instant, 2026-10-29T00:00Z is 28 Oct in California, which
      // would make tomorrow's bill look due today.
      expect(due([sub("Water", "monthly", "2026-10-29")], new Date(2026, 9, 28, 20))).toEqual(["Water 2026-10-29 due"]);
    });
  });
});
