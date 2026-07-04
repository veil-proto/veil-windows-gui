// Minimal IPv4/IPv6 CIDR parsing and prefix arithmetic — just enough to
// support disallowed.ts's CIDR subtraction, mirroring what Go's net package
// gave the original disallowed.go for free.

export interface ParsedCIDR {
  ip: Uint8Array; // network address, already masked (4 bytes for IPv4, 16 for IPv6)
  bits: number; // 32 or 128
  ones: number; // prefix length
}

export function parseCIDR(input: string): ParsedCIDR | null {
  const trimmed = input.trim();
  const slash = trimmed.lastIndexOf("/");
  if (slash < 0) return null;
  const addrPart = trimmed.slice(0, slash);
  const prefixPart = trimmed.slice(slash + 1);
  const ones = Number(prefixPart);
  if (!Number.isInteger(ones) || ones < 0) return null;

  const v4 = parseIPv4(addrPart);
  if (v4) {
    if (ones > 32) return null;
    return { ip: maskBytes(v4, ones, 32), bits: 32, ones };
  }
  const v6 = parseIPv6(addrPart);
  if (v6) {
    if (ones > 128) return null;
    return { ip: maskBytes(v6, ones, 128), bits: 128, ones };
  }
  return null;
}

export function cidrToString(c: ParsedCIDR): string {
  const addr = c.bits === 32 ? formatIPv4(c.ip) : formatIPv6(c.ip);
  return `${addr}/${c.ones}`;
}

/** Number of addresses covered by a CIDR (as a bigint, since /0 overflows a number). */
export function cidrSize(c: ParsedCIDR): bigint {
  return 1n << BigInt(c.bits - c.ones);
}

function parseIPv4(input: string): Uint8Array | null {
  const parts = input.split(".");
  if (parts.length !== 4) return null;
  const bytes = new Uint8Array(4);
  for (let i = 0; i < 4; i++) {
    if (!/^\d{1,3}$/.test(parts[i])) return null;
    const n = Number(parts[i]);
    if (n > 255) return null;
    bytes[i] = n;
  }
  return bytes;
}

function parseIPv6(input: string): Uint8Array | null {
  if (!input.includes(":")) return null;
  const doubleColonCount = (input.match(/::/g) ?? []).length;
  if (doubleColonCount > 1) return null;

  let head: string[];
  let tail: string[];
  if (input.includes("::")) {
    const [h, t] = input.split("::");
    head = h === "" ? [] : h.split(":");
    tail = t === "" ? [] : t.split(":");
  } else {
    head = input.split(":");
    tail = [];
  }

  const groups = [...head, ...tail];
  if (groups.length > 8) return null;
  const missing = 8 - head.length - tail.length;
  if (!input.includes("::") && missing !== 0) return null;
  if (input.includes("::") && missing < 0) return null;

  const bytes = new Uint8Array(16);
  const allGroups = input.includes("::")
    ? [...head, ...Array(missing).fill("0"), ...tail]
    : groups;
  if (allGroups.length !== 8) return null;

  for (let i = 0; i < 8; i++) {
    if (!/^[0-9a-fA-F]{1,4}$/.test(allGroups[i])) return null;
    const value = parseInt(allGroups[i], 16);
    bytes[i * 2] = (value >> 8) & 0xff;
    bytes[i * 2 + 1] = value & 0xff;
  }
  return bytes;
}

function formatIPv4(bytes: Uint8Array): string {
  return Array.from(bytes).join(".");
}

function formatIPv6(bytes: Uint8Array): string {
  const groups: number[] = [];
  for (let i = 0; i < 16; i += 2) {
    groups.push((bytes[i] << 8) | bytes[i + 1]);
  }
  // Simple, correct-but-not-maximally-compressed formatting is enough here:
  // this project's own configs are never displayed in Go's canonical
  // shortest form anywhere else either, and round-tripping through
  // parseCIDR only cares about the address value, not its textual style.
  return groups.map((g) => g.toString(16)).join(":");
}

function maskBytes(ip: Uint8Array, ones: number, bits: number): Uint8Array {
  const out = new Uint8Array(bits / 8);
  for (let i = 0; i < out.length; i++) {
    const bitOffset = i * 8;
    if (bitOffset + 8 <= ones) {
      out[i] = ip[i];
    } else if (bitOffset >= ones) {
      out[i] = 0;
    } else {
      const keepBits = ones - bitOffset;
      const mask = 0xff << (8 - keepBits);
      out[i] = ip[i] & mask;
    }
  }
  return out;
}

/** Reports whether ip lies inside CIDR c (same family, already masked). */
export function contains(c: ParsedCIDR, ip: Uint8Array): boolean {
  const masked = maskBytes(ip, c.ones, c.bits);
  return bytesEqual(masked, c.ip);
}

export function cidrsOverlap(a: ParsedCIDR, b: ParsedCIDR): boolean {
  if (a.bits !== b.bits) return false;
  return contains(a, b.ip) || contains(b, a.ip);
}

/** Splits CIDR n into its two next-longer-prefix children. */
export function splitInHalf(n: ParsedCIDR): [ParsedCIDR, ParsedCIDR] {
  const newOnes = n.ones + 1;
  const byteIdx = Math.floor((newOnes - 1) / 8);
  const bitIdx = 7 - ((newOnes - 1) % 8);

  const loIP = new Uint8Array(n.ip);
  loIP[byteIdx] &= ~(1 << bitIdx) & 0xff;
  const hiIP = new Uint8Array(n.ip);
  hiIP[byteIdx] |= 1 << bitIdx;

  return [
    { ip: maskBytes(loIP, newOnes, n.bits), bits: n.bits, ones: newOnes },
    { ip: maskBytes(hiIP, newOnes, n.bits), bits: n.bits, ones: newOnes },
  ];
}

function bytesEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}
