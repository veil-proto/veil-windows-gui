//go:build windows

package main

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestSubtractCIDRs_NoOverlap(t *testing.T) {
	got := subtractCIDRs([]string{"10.0.0.0/24"}, []string{"192.168.0.0/24"})
	want := []string{"10.0.0.0/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSubtractCIDRs_FullyContained(t *testing.T) {
	got := subtractCIDRs([]string{"10.0.0.0/24"}, []string{"10.0.0.0/16"})
	if len(got) != 0 {
		t.Fatalf("got %v, want empty (fully disallowed)", got)
	}
}

func TestSubtractCIDRs_ExactMatch(t *testing.T) {
	got := subtractCIDRs([]string{"10.0.0.0/24"}, []string{"10.0.0.0/24"})
	if len(got) != 0 {
		t.Fatalf("got %v, want empty (exact match removed)", got)
	}
}

func TestSubtractCIDRs_PartialOverlapSplits(t *testing.T) {
	// Disallow the second half of a /24: 10.0.0.128/25. Expect the first
	// half (10.0.0.0/25) to remain.
	got := subtractCIDRs([]string{"10.0.0.0/24"}, []string{"10.0.0.128/25"})
	want := []string{"10.0.0.0/25"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSubtractCIDRs_SingleHostCarveOut(t *testing.T) {
	// A very common real-world case: exclude one /32 host from a broad range.
	got := subtractCIDRs([]string{"10.0.0.0/30"}, []string{"10.0.0.1/32"})
	sort.Strings(got)
	want := []string{"10.0.0.0/32", "10.0.0.2/31"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Sanity: total addresses covered should be 3 (4 - 1).
	total := 0
	for _, c := range got {
		total += cidrSize(t, c)
	}
	if total != 3 {
		t.Fatalf("covered %d addresses, want 3", total)
	}
}

func TestSubtractCIDRs_DefaultRouteMinusLAN(t *testing.T) {
	// 0.0.0.0/0 minus a local LAN is the classic split-tunnel use case.
	got := subtractCIDRs([]string{"0.0.0.0/0"}, []string{"192.168.1.0/24"})
	if len(got) == 0 {
		t.Fatal("expected remaining coverage after carving out a /24 from /0")
	}
	// The disallowed block itself must not appear in any remaining piece.
	for _, c := range got {
		if c == "192.168.1.0/24" {
			t.Fatalf("disallowed CIDR leaked into result: %v", got)
		}
	}
}

func TestSubtractCIDRs_NoDisallowedIsNoOp(t *testing.T) {
	in := []string{"10.0.0.0/24", "192.168.0.0/16"}
	got := subtractCIDRs(in, nil)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("got %v, want unchanged %v", got, in)
	}
}

func TestSubtractCIDRs_InvalidAllowedPassedThrough(t *testing.T) {
	got := subtractCIDRs([]string{"not-a-cidr"}, []string{"10.0.0.0/24"})
	want := []string{"not-a-cidr"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSubtractCIDRs_DifferentFamiliesUnaffected(t *testing.T) {
	got := subtractCIDRs([]string{"10.0.0.0/24"}, []string{"::/0"})
	want := []string{"10.0.0.0/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v (IPv6 disallow must not affect IPv4 allowed)", got, want)
	}
}

// cidrSize returns the number of addresses covered by a CIDR string.
func cidrSize(t *testing.T, cidr string) int {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse %q: %v", cidr, err)
	}
	ones, bits := n.Mask.Size()
	return 1 << (bits - ones)
}

func TestDisallowedSidecarPath(t *testing.T) {
	got := disallowedSidecarPath(filepath.Join("C:", "configs", "Home.conf"))
	want := filepath.Join("C:", "configs", "Home.disallowed.json")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadSaveDisallowedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "Office.conf")
	if err := os.WriteFile(confPath, []byte("[Interface]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// No sidecar yet: should load an empty-but-usable doc.
	doc, err := loadDisallowed(confPath)
	if err != nil {
		t.Fatalf("loadDisallowed (missing file): %v", err)
	}
	if doc.PerPeer == nil || len(doc.PerPeer) != 0 {
		t.Fatalf("expected empty PerPeer map, got %+v", doc)
	}

	doc.PerPeer["aabbcc"] = []string{"192.168.1.0/24", "10.0.0.5/32"}
	if err := saveDisallowed(confPath, doc); err != nil {
		t.Fatalf("saveDisallowed: %v", err)
	}

	got, err := loadDisallowed(confPath)
	if err != nil {
		t.Fatalf("loadDisallowed (after save): %v", err)
	}
	if !reflect.DeepEqual(got.PerPeer["aabbcc"], doc.PerPeer["aabbcc"]) {
		t.Fatalf("got %+v, want %+v", got.PerPeer, doc.PerPeer)
	}

	if _, err := os.Stat(disallowedSidecarPath(confPath)); err != nil {
		t.Fatalf("sidecar file not created: %v", err)
	}
}
