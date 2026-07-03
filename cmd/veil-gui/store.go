//go:build windows

package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"github.com/veil-proto/veil/link"
)

// The GUI keeps imported configs as plain VEIL .conf files under the user's
// app-data directory. The service is told which one to connect (by sending its
// text); the GUI just manages the list the user picks from.
//
// Configs reach the store via three entry points:
//   - importLink:        decode a veil:// link (legacy, from the original GUI)
//   - importFromFile:    copy a .conf file the user picked via the file dialog
//   - importFromURI:     same, but driven by a drag-and-drop URI

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
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".conf") {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		// Skip dotfiles like ".conf" that have no base name; they
		// would otherwise appear as configs with an empty name.
		if strings.TrimSpace(base) == "" {
			continue
		}
		out = append(out, configEntry{
			Name: base,
			Path: filepath.Join(dir, name),
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

// deleteConfigIn removes a config by display name. Returns true if a file was
// deleted, false if the config was not found.
func deleteConfigIn(dir, name string) (bool, error) {
	base := sanitizeName(name)
	path := filepath.Join(dir, base+".conf")
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

// importFromFileInto copies a local .conf file into the store, keeping the
// original filename (sanitized). The caller is expected to have already
// validated the extension.
func importFromFileInto(dir, srcPath string) (configEntry, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return configEntry{}, err
	}
	name := filepath.Base(srcPath)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return saveConfigIn(dir, name, string(data))
}

// importFromURIInto handles a fyne.URI (file dialog or drag-and-drop) pointing
// at a .conf file. Returns an error if the URI is not a file URI or does not
// have a .conf extension.
//
// For file:// URIs we read the path directly with os.ReadFile. This avoids
// depending on Fyne's storage repository registration (which is only wired
// up once the app is initialised) and keeps this function usable from tests
// and from drag-and-drop handlers that may fire before the app loop is ready.
func importFromURIInto(dir string, uri fyne.URI) (configEntry, error) {
	if uri == nil {
		return configEntry{}, errInvalidURI
	}
	name := uri.Name()
	if !strings.EqualFold(filepath.Ext(name), ".conf") {
		return configEntry{}, errNotConf
	}

	var data []byte
	var err error
	if uri.Scheme() == "file" {
		data, err = os.ReadFile(uri.Path())
	} else {
		reader, rerr := storage.Reader(uri)
		if rerr != nil {
			return configEntry{}, rerr
		}
		data, err = io.ReadAll(reader)
		reader.Close()
	}
	if err != nil {
		return configEntry{}, err
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return saveConfigIn(dir, base, string(data))
}

func listConfigs() ([]configEntry, error)             { return listConfigsIn(storeDir()) }
func importLink(linkStr string) (configEntry, error)  { return importLinkInto(storeDir(), linkStr) }
func importFromFile(path string) (configEntry, error) { return importFromFileInto(storeDir(), path) }
func importFromURI(uri fyne.URI) (configEntry, error) { return importFromURIInto(storeDir(), uri) }
func deleteConfig(name string) (bool, error)          { return deleteConfigIn(storeDir(), name) }
