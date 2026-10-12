import { describe, expect, it } from "vitest";
import { parseDollarsToCents } from "./utils";

describe("parseDollarsToCents", () => {
  it("converts dollars to whole cents", () => {
    expect(parseDollarsToCents("500")).toBe(50000);
    expect(parseDollarsToCents("12.5")).toBe(1250);
    expect(parseDollarsToCents(" 0.1 ")).toBe(10);
    expect(parseDollarsToCents("0")).toBe(0);
  });

  it("rounds sub-cent amounts instead of storing fractions", () => {
    expect(parseDollarsToCents("19.999")).toBe(2000);
    expect(parseDollarsToCents("0.29")).toBe(29);
  });

  it("rejects empty, non-numeric and negative input", () => {
    expect(parseDollarsToCents("")).toBeNull();
    expect(parseDollarsToCents("   ")).toBeNull();
    expect(parseDollarsToCents("abc")).toBeNull();
    expect(parseDollarsToCents("-5")).toBeNull();
    expect(parseDollarsToCents("Infinity")).toBeNull();
  });
});
