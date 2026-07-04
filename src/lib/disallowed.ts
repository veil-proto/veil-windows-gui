// TypeScript port of the old Fyne GUI's disallowed.go.
//
// "Disallowed" subnets are a pure client-side concept: they do not exist in
// the wire protocol, the engine, or config.PeerConfig. A user marks certain
// CIDRs within a peer's AllowedIPs as "don't actually route these" — e.g. to
// carve a local subnet out of a 0.0.0.0/0 AllowedIPs — without the sidecar
// or protocol ever knowing "disallowed" is a concept. This is implemented as
// a client-side transform: at Connect time, AllowedIPs minus Disallowed
// (via CIDR subtraction, subtractCIDRs below) is what actually gets
// serialized and sent to the sidecar; the user's original AllowedIPs and
// their Disallowed input are both preserved locally so the editor keeps
// showing them exactly as entered.

import { readTextFile, writeTextFile, exists } from "@tauri-apps/plugin-fs";
import { cidrsOverlap, cidrToString, parseCIDR, splitInHalf, type ParsedCIDR } from "./cidr";
import type { ParsedConfig } from "./control-types";

export interface DisallowedDoc {
  per_peer: Record<string, string[]>;
}

export function disallowedSidecarPath(confPath: string): string {
  const lastSlash = Math.max(confPath.lastIndexOf("/"), confPath.lastIndexOf("\\"));
  const dir = lastSlash >= 0 ? confPath.slice(0, lastSlash) : "";
  const fileName = lastSlash >= 0 ? confPath.slice(lastSlash + 1) : confPath;
  const dot = fileName.lastIndexOf(".");
  const base = dot >= 0 ? fileName.slice(0, dot) : fileName;
  const sep = confPath.includes("\\") && !confPath.includes("/") ? "\\" : "/";
  return dir ? `${dir}${sep}${base}.disallowed.json` : `${base}.disallowed.json`;
}

export async function loadDisallowed(confPath: string): Promise<DisallowedDoc> {
  const path = disallowedSidecarPath(confPath);
  if (!(await exists(path))) {
    return { per_peer: {} };
  }
  const text = await readTextFile(path);
  const doc = JSON.parse(text) as DisallowedDoc;
  if (!doc.per_peer) doc.per_peer = {};
  return doc;
}

export async function saveDisallowed(confPath: string, doc: DisallowedDoc): Promise<void> {
  await writeTextFile(disallowedSidecarPath(confPath), JSON.stringify(doc, null, 2));
}

/**
 * Mirrors the old Fyne GUI's effectiveConfigText: returns a copy of pc with
 * each peer's allowedIPs reduced by that peer's Disallowed CIDRs (keyed by
 * publicKey). The on-disk config and the Disallowed doc are both left
 * untouched — this is only ever used to build the text actually sent to the
 * sidecar at Connect time.
 */
export function applyDisallowed(pc: ParsedConfig, doc: DisallowedDoc): ParsedConfig {
  if (Object.keys(doc.per_peer).length === 0) {
    return pc;
  }
  return {
    ...pc,
    peers: pc.peers.map((peer) => {
      const disallowed = doc.per_peer[peer.publicKey];
      if (!disallowed || disallowed.length === 0) {
        return peer;
      }
      return { ...peer, allowedIPs: subtractCIDRs(peer.allowedIPs ?? [], disallowed) };
    }),
  };
}

/**
 * Computes allowed minus disallowed as a set of CIDRs: every address covered
 * by an entry in allowed but not covered by any entry in disallowed. Each
 * allowed CIDR is subtracted against every disallowed CIDR in turn, so
 * overlaps with more than one disallowed entry are all removed. IPv4 and
 * IPv6 entries never interact.
 *
 * Invalid CIDR strings are passed through unchanged rather than dropped —
 * validation is the editor's job (via the sidecar's Validate, over
 * parseConfig/serializeConfig); this function's contract is purely
 * "subtract what parses, leave the rest".
 */
export function subtractCIDRs(allowed: string[], disallowed: string[]): string[] {
  const disallowedNets: ParsedCIDR[] = [];
  for (const d of disallowed) {
    const parsed = parseCIDR(d);
    if (parsed) disallowedNets.push(parsed);
  }
  if (disallowedNets.length === 0) {
    return [...allowed];
  }

  const result: string[] = [];
  for (const a of allowed) {
    const aNet = parseCIDR(a);
    if (!aNet) {
      // Not a parseable CIDR — leave it exactly as entered.
      result.push(a);
      continue;
    }
    let remaining: ParsedCIDR[] = [aNet];
    for (const d of disallowedNets) {
      const next: ParsedCIDR[] = [];
      for (const r of remaining) {
        next.push(...subtractOne(r, d));
      }
      remaining = next;
    }
    for (const r of remaining) {
      result.push(cidrToString(r));
    }
  }
  return result;
}

/**
 * Removes d from a, returning the CIDR-aligned pieces of a that remain. If a
 * and d don't overlap, a is returned unchanged; if d fully covers a, nothing
 * is returned; otherwise a is split into its two next-longer-prefix halves
 * and each half is recursively subtracted.
 */
function subtractOne(a: ParsedCIDR, d: ParsedCIDR): ParsedCIDR[] {
  if (a.bits !== d.bits) {
    return [a]; // different address families, no overlap
  }
  if (!cidrsOverlap(a, d)) {
    return [a];
  }
  if (d.ones <= a.ones) {
    // d is equal-or-broader and (per the overlap check above) contains a.
    return [];
  }
  if (a.ones >= a.bits) {
    // a is a single host address that overlaps d; fully removed.
    return [];
  }
  const [left, right] = splitInHalf(a);
  return [...subtractOne(left, d), ...subtractOne(right, d)];
}
