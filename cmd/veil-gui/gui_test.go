//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/veil-proto/veil/link"
)

func TestHuman(t *testing.T) {
	tests := map[uint64]string{
		0:          "0 B",
		42:         "42 B",
		1024:       "1.0 KiB",
		1024 * 512: "512.0 KiB",
	}
	for n, want := range tests {
		if got := human(n); got != want {
			t.Fatalf("human(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	if got := sanitizeName(`..\bad:name|`); got != "-bad-name-" {
		t.Fatalf("sanitizeName = %q", got)
	}
	if got := sanitizeName("   "); got != "veil" {
		t.Fatalf("empty sanitizeName = %q", got)
	}
}

func TestImportLinkInto(t *testing.T) {
	dir := t.TempDir()
	cfg := "[Interface]\nPrivateKey = 00\n"
	linkStr := link.Encode(cfg, "Home:Office")
	ent, err := importLinkInto(dir, linkStr)
	if err != nil {
		t.Fatal(err)
	}
	if ent.Name != "Home-Office" {
		t.Fatalf("Name = %q", ent.Name)
	}
	got, err := os.ReadFile(filepath.Join(dir, "Home-Office.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != cfg {
		t.Fatalf("config content mismatch")
	}
}
