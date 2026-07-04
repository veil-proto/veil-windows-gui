// Ported from cmd/veil-gui/disallowed_test.go — same cases, same fixture
// values, against the TS port of the same algorithm.
import { describe, expect, it } from "vitest";
import { parseCIDR } from "./cidr";
import { disallowedSidecarPath, subtractCIDRs } from "./disallowed";

function cidrSizeOf(cidr: string): bigint {
  const c = parseCIDR(cidr);
  if (!c) throw new Error(`unparseable: ${cidr}`);
  return 1n << BigInt(c.bits - c.ones);
}

describe("subtractCIDRs", () => {
  it("leaves non-overlapping allowed untouched", () => {
    expect(subtractCIDRs(["10.0.0.0/24"], ["192.168.0.0/24"])).toEqual(["10.0.0.0/24"]);
  });

  it("removes an allowed CIDR fully contained by disallowed", () => {
    expect(subtractCIDRs(["10.0.0.0/24"], ["10.0.0.0/16"])).toEqual([]);
  });

  it("removes an exact match", () => {
    expect(subtractCIDRs(["10.0.0.0/24"], ["10.0.0.0/24"])).toEqual([]);
  });

  it("splits on a partial overlap", () => {
    // Disallow the second half of a /24: 10.0.0.128/25. Expect the first
    // half (10.0.0.0/25) to remain.
    expect(subtractCIDRs(["10.0.0.0/24"], ["10.0.0.128/25"])).toEqual(["10.0.0.0/25"]);
  });

  it("carves a single host out of a small range", () => {
    const got = subtractCIDRs(["10.0.0.0/30"], ["10.0.0.1/32"]).sort();
    expect(got).toEqual(["10.0.0.0/32", "10.0.0.2/31"].sort());

    const total = got.reduce((sum, c) => sum + cidrSizeOf(c), 0n);
    expect(total).toBe(3n);
  });

  it("carves a LAN out of a default route (classic split tunnel)", () => {
    const got = subtractCIDRs(["0.0.0.0/0"], ["192.168.1.0/24"]);
    expect(got.length).toBeGreaterThan(0);
    expect(got).not.toContain("192.168.1.0/24");
  });

  it("is a no-op with no disallowed entries", () => {
    const input = ["10.0.0.0/24", "192.168.0.0/16"];
    expect(subtractCIDRs(input, [])).toEqual(input);
  });

  it("passes through an unparseable allowed entry", () => {
    expect(subtractCIDRs(["not-a-cidr"], ["10.0.0.0/24"])).toEqual(["not-a-cidr"]);
  });

  it("never lets an IPv6 disallow affect an IPv4 allowed entry", () => {
    expect(subtractCIDRs(["10.0.0.0/24"], ["::/0"])).toEqual(["10.0.0.0/24"]);
  });
});

describe("disallowedSidecarPath", () => {
  it("derives <name>.disallowed.json next to the .conf file", () => {
    expect(disallowedSidecarPath("C:\\configs\\Home.conf")).toBe(
      "C:\\configs\\Home.disallowed.json",
    );
  });

  it("works with forward slashes too", () => {
    expect(disallowedSidecarPath("/home/user/Office.conf")).toBe(
      "/home/user/Office.disallowed.json",
    );
  });
});
