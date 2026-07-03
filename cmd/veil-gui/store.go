//go:build windows

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/veil-proto/veil/link"
)

// The GUI keeps imported configs as plain VEIL .conf files under the user's
// app-data directory. The service is told which one to connect (by sending its
// text); the GUI just manages the list the user picks from.

type configEntry struct {
	Name string // display name (file base, no extension)
	Path string
}

func storeDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "VEIL", "configs")
}

// sanitizeName reduces a display name to a safe file base (no separators or
// other characters that would escape the store directory).
func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "veil"
	}
	return name
}

func listConfigsIn(dir string) ([]configEntry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []configEntry
	for _, e := range ents {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".conf") {
			continue
		}
		out = append(out, configEntry{
			Name: strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
			Path: filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func saveConfigIn(dir, name, text string) (configEntry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return configEntry{}, err
	}
	base := sanitizeName(name)
	path := filepath.Join(dir, base+".conf")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return configEntry{}, err
	}
	return configEntry{Name: base, Path: path}, nil
}

// importLinkInto decodes a veil:// link and saves it as a config in dir.
func importLinkInto(dir, linkStr string) (configEntry, error) {
	text, name, err := link.Decode(linkStr)
	if err != nil {
		return configEntry{}, err
	}
	if name == "" {
		name = "veil"
	}
	return saveConfigIn(dir, name, text)
}

func listConfigs() ([]configEntry, error)            { return listConfigsIn(storeDir()) }
func importLink(linkStr string) (configEntry, error) { return importLinkInto(storeDir(), linkStr) }
