//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
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

func TestImportFromFileInto(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "ya.conf")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "[Interface]\nPrivateKey = 01\nAddress = 10.0.0.2/24\n"
	if err := os.WriteFile(src, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}

	store := t.TempDir()
	ent, err := importFromFileInto(store, src)
	if err != nil {
		t.Fatal(err)
	}
	if ent.Name != "ya" {
		t.Fatalf("Name = %q, want %q", ent.Name, "ya")
	}
	if ent.Path != filepath.Join(store, "ya.conf") {
		t.Fatalf("Path = %q", ent.Path)
	}
	got, err := os.ReadFile(ent.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("content mismatch: got %q", got)
	}

	// original file must survive the copy
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source file removed: %v", err)
	}
}

func TestImportFromURIInto(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "drop-in.conf")
	want := "[Interface]\nPrivateKey = ab\n[Peer]\nPublicKey = cd\n"
	if err := os.WriteFile(src, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}

	uri := storage.NewFileURI(src)
	store := t.TempDir()
	ent, err := importFromURIInto(store, uri)
	if err != nil {
		t.Fatal(err)
	}
	if ent.Name != "drop-in" {
		t.Fatalf("Name = %q", ent.Name)
	}
	got, err := os.ReadFile(ent.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("content mismatch: got %q", got)
	}
}

func TestImportFromURIIntoRejectsNonConf(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "not-a-config.txt")
	if err := os.WriteFile(src, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}

	uri := storage.NewFileURI(src)
	if _, err := importFromURIInto(t.TempDir(), uri); err != errNotConf {
		t.Fatalf("err = %v, want %v", err, errNotConf)
	}
}

func TestImportFromURIIntoNil(t *testing.T) {
	if _, err := importFromURIInto(t.TempDir(), (fyne.URI)(nil)); err != errInvalidURI {
		t.Fatalf("err = %v, want %v", err, errInvalidURI)
	}
}

func TestDeleteConfigIn(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "ya.conf")
	if err := os.WriteFile(cfg, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := deleteConfigIn(dir, "ya")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("removed = false, want true")
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Fatalf("file still exists after delete: %v", err)
	}

	// deleting again is a no-op, not an error
	removed, err = deleteConfigIn(dir, "ya")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Fatal("second delete should report removed=false")
	}
}

func TestListConfigsInFiltersConfOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.conf", "b.CONF", "c.txt", "d", ".conf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := listConfigsIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b"} // ".conf" dotfile is skipped, see listConfigsIn
	if len(got) != len(want) {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	for i, e := range got {
		if e.Name != want[i] {
			t.Fatalf("got[%d].Name = %q, want %q", i, e.Name, want[i])
		}
	}
}
