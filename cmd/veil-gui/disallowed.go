//go:build windows

package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// "Disallowed" subnets are a pure GUI-side, client-only concept: they do not
// exist in the wire protocol, the engine, or config.PeerConfig. A user marks
// certain CIDRs within a peer's AllowedIPs as "don't actually route these" —
// e.g. to carve a local subnet out of a 0.0.0.0/0 AllowedIPs — without the
// service or protocol ever knowing "disallowed" is a concept. We implement
// this as a client-side transform: at Connect time (see effectiveConfigText
// below) we compute AllowedIPs minus Disallowed via CIDR subtraction and
// serialize *that* config text to send over the wire. The user's original
// AllowedIPs (the on-disk source of truth) and their Disallowed input are
// both preserved locally so the structured editor can keep showing them
// as entered, unmodified by the subtraction.

// disallowedSidecarPath returns the path of the sidecar JSON file that stores
// a config's Disallowed CIDRs, keyed off the config's own store path (e.g.
// "Home.conf" -> "Home.disallowed.json", right next to it). Keeping it
// alongside the .conf file (rather than in some central index) means
// deleting a config's file and its sidecar are two independent, obvious
// operations, and importing/exporting a single config never has to remember
// a second location.
func disallowedSidecarPath(confPath string) string {
	dir := filepath.Dir(confPath)
	base := strings.TrimSuffix(filepath.Base(confPath), filepath.Ext(confPath))
	return filepath.Join(dir, base+".disallowed.json")
}

// disallowedDoc is the on-disk shape of the sidecar file. PerPeer maps a
// peer's public key (hex-encoded, matching config.PeerConfig.PublicKey once
// hex-decoded) to the list of CIDRs the user has marked disallowed for that
// peer specifically — different peers can have different AllowedIPs, so the
// exclusion list is naturally per-peer too.
type disallowedDoc struct {
	PerPeer map[string][]string `json:"per_peer"`
}

func loadDisallowed(confPath string) (disallowedDoc, error) {
	b, err := os.ReadFile(disallowedSidecarPath(confPath))
	if err != nil {
		if os.IsNotExist(err) {
			return disallowedDoc{PerPeer: map[string][]string{}}, nil
		}
		return disallowedDoc{}, err
	}
	var doc disallowedDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return disallowedDoc{}, err
	}
	if doc.PerPeer == nil {
		doc.PerPeer = map[string][]string{}
	}
	return doc, nil
}

func saveDisallowed(confPath string, doc disallowedDoc) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(disallowedSidecarPath(confPath), b, 0o600)
}

// subtractCIDRs computes allowed minus disallowed as a set of CIDRs: every
// address covered by an entry in allowed but not covered by any entry in
// disallowed. Each allowed CIDR is subtracted against every disallowed CIDR
// in turn (subtractOne), so overlaps with more than one disallowed entry are
// all removed. IPv4 and IPv6 entries never interact (an IPv4 CIDR is never
// split by an IPv6 disallowed entry or vice versa).
//
// Invalid CIDR strings are passed through unchanged rather than dropped —
// validation is the editor's job (via config.PeerConfig.Validate()); this
// function's contract is purely "subtract what parses, leave the rest".
func subtractCIDRs(allowed, disallowed []string) []string {
	var disallowedNets []*net.IPNet
	for _, d := range disallowed {
		if _, n, err := net.ParseCIDR(strings.TrimSpace(d)); err == nil {
			disallowedNets = append(disallowedNets, n)
		}
	}
	if len(disallowedNets) == 0 {
		out := make([]string, len(allowed))
		copy(out, allowed)
		return out
	}

	var result []string
	for _, a := range allowed {
		aTrim := strings.TrimSpace(a)
		_, aNet, err := net.ParseCIDR(aTrim)
		if err != nil {
			// Not a parseable CIDR — leave it exactly as entered.
			result = append(result, a)
			continue
		}
		remaining := []*net.IPNet{aNet}
		for _, d := range disallowedNets {
			var next []*net.IPNet
			for _, r := range remaining {
				next = append(next, subtractOne(r, d)...)
			}
			remaining = next
		}
		for _, r := range remaining {
			result = append(result, r.String())
		}
	}
	return result
}

// subtractOne removes d from a, returning the CIDR-aligned pieces of a that
// remain. This is the standard "subtract a prefix from a prefix" algorithm:
// if a and d don't overlap, a is returned unchanged; if d fully covers a,
// nothing is returned; otherwise a is split into its two next-longer-prefix
// halves and each half is recursively subtracted (a half fully inside d is
// dropped, a half disjoint from d is kept whole, a half straddling d's
// boundary is split further). IPv4/IPv6 families that don't match are always
// disjoint (returned unchanged).
func subtractOne(a, d *net.IPNet) []*net.IPNet {
	aOnes, aBits := a.Mask.Size()
	dOnes, dBits := d.Mask.Size()
	if aBits != dBits {
		return []*net.IPNet{a} // different address families, no overlap
	}

	if !cidrsOverlap(a, d) {
		return []*net.IPNet{a}
	}
	if dOnes <= aOnes && d.Contains(a.IP) {
		// d fully contains a (d is equal or broader).
		return nil
	}

	// a is broader than d and overlaps it: split a into two halves at the
	// next prefix length and recurse into each.
	if aOnes >= aBits {
		// a is a single host address (/32 or /128) and didn't fully match
		// the containment check above only due to a mask-size edge case;
		// treat as fully removed since we already know it overlaps.
		return nil
	}
	left, right := splitInHalf(a)
	var out []*net.IPNet
	out = append(out, subtractOne(left, d)...)
	out = append(out, subtractOne(right, d)...)
	return out
}

// cidrsOverlap reports whether a and d share any address.
func cidrsOverlap(a, d *net.IPNet) bool {
	return a.Contains(d.IP) || d.Contains(a.IP)
}

// splitInHalf splits CIDR n (prefix length p) into its two children at
// prefix length p+1: the "0" half and the "1" half of the newly-significant
// bit.
func splitInHalf(n *net.IPNet) (lo, hi *net.IPNet) {
	ones, bits := n.Mask.Size()
	newOnes := ones + 1
	newMask := net.CIDRMask(newOnes, bits)

	ip := n.IP.To16()
	if bits == 32 {
		ip = n.IP.To4()
	}
	byteIdx := (newOnes - 1) / 8
	bitIdx := 7 - (newOnes-1)%8

	loIP := make(net.IP, len(ip))
	copy(loIP, ip)
	loIP[byteIdx] &^= 1 << bitIdx // ensure the new bit is 0

	hiIP := make(net.IP, len(ip))
	copy(hiIP, ip)
	hiIP[byteIdx] |= 1 << bitIdx // set the new bit to 1

	lo = &net.IPNet{IP: loIP.Mask(newMask), Mask: newMask}
	hi = &net.IPNet{IP: hiIP.Mask(newMask), Mask: newMask}
	return lo, hi
}
