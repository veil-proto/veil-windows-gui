import { describe, expect, it } from "vitest";
import { sanitizeName } from "./config-store";

describe("sanitizeName", () => {
  it("passes through an already-safe name", () => {
    expect(sanitizeName("Home Server")).toBe("Home Server");
  });

  it("replaces path-escaping characters", () => {
    expect(sanitizeName("a/b\\c:d*e?f\"g<h>i|j")).toBe("a-b-c-d-e-f-g-h-i-j");
  });

  it("trims leading/trailing dots and whitespace", () => {
    expect(sanitizeName("  ..foo..  ")).toBe("foo");
  });

  it("falls back to 'veil' when nothing survives", () => {
    expect(sanitizeName("   ")).toBe("veil");
    expect(sanitizeName("...")).toBe("veil");
  });
});
