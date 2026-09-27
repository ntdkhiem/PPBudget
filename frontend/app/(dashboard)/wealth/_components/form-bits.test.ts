import { describe, expect, it } from "vitest";
import { toCents, toFraction, toInputValue } from "./form-bits";

describe("toCents", () => {
  it("reads dollars as typed, separators and signs included", () => {
    expect(toCents("1,234.56")).toBe(123456);
    expect(toCents("$50")).toBe(5000);
    expect(toCents("-12.5")).toBe(-1250);
  });

  it("rounds to the cent rather than truncating a float error", () => {
    // 0.29 * 100 is 28.999999999999996 in floating point.
    expect(toCents("0.29")).toBe(29);
  });

  it("is null for nothing to read", () => {
    expect(toCents("")).toBeNull();
    expect(toCents("abc")).toBeNull();
  });
});

describe("toFraction", () => {
  it("reads percent as a fraction", () => {
    expect(toFraction("6")).toBeCloseTo(0.06, 12);
    expect(toFraction("22.49%")).toBeCloseTo(0.2249, 12);
  });

  it("is null for nothing to read", () => {
    expect(toFraction("")).toBeNull();
  });
});

describe("toInputValue", () => {
  it("is empty for an unanswered field", () => {
    expect(toInputValue("money", undefined)).toBe("");
    expect(toInputValue("text", null)).toBe("");
  });

  it("puts each kind back in its input's units", () => {
    expect(toInputValue("boolean", true)).toBe("yes");
    expect(toInputValue("boolean", false)).toBe("no");
    expect(toInputValue("date", "1990-06-15T00:00:00Z")).toBe("1990-06-15");
    expect(toInputValue("money", 16_500_000)).toBe("165000");
    expect(toInputValue("money", 12_345)).toBe("123.45");
    expect(toInputValue("integer", 6)).toBe("6");
    expect(toInputValue("choice", "single")).toBe("single");
    expect(toInputValue("text", "CA")).toBe("CA");
  });

  it("shows percent without the float noise a fraction carries", () => {
    // 0.07 * 100 is 7.000000000000001.
    expect(toInputValue("percent", 0.07)).toBe("7");
    expect(toInputValue("percent", 0.02)).toBe("2");
    expect(toInputValue("percent", 0.2249)).toBe("22.49");
  });

  it("round-trips with the conversions it reverses", () => {
    for (const cents of [0, 1, 99, 123_456, 16_500_000]) {
      expect(toCents(toInputValue("money", cents))).toBe(cents);
    }
    for (const fraction of [0.02, 0.06, 0.5, 0.2249]) {
      expect(toFraction(toInputValue("percent", fraction))).toBeCloseTo(fraction, 12);
    }
  });
});
