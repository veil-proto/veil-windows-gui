//go:build windows

package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/veil-proto/veil-windows/control"
	"github.com/veil-proto/veil-windows/wintunnel"
	"github.com/veil-proto/veil/config"
)

// persistingHandler wraps the tunnel controller and remembers the active config
// on disk, so the sidecar reconnects the last tunnel after a crash-restart
// (the Tauri shell respawning it) without the frontend needing to reissue
// Connect itself.
type persistingHandler struct {
	*wintunnel.Tunnel
}

func newHandler() *persistingHandler {
	t := &wintunnel.Tunnel{}
	// Route the process's default log output through the tunnel's ring
	// buffer (in addition to stderr) so every existing log.Printf call site
	// across the engine/tunnel code is captured for the Logs control
	// command, with no call-site changes required. This must happen before
	// anything logs (in particular before autoReconnect below), and the
	// buffer is process-lifetime so it survives Connect/Disconnect cycles.
	// Critically, this must never write to os.Stdout: stdout is reserved
	// entirely for the control-protocol's JSON response lines (ServeIO), so
	// mixing log output into it would corrupt the wire format the Tauri
	// shell parses.
	log.SetOutput(io.MultiWriter(os.Stderr, t.LogBuffer()))
	return &persistingHandler{Tunnel: t}
}

type activeConfig struct {
	Name   string `json:"name"`
	Config string `json:"config"`
}

func activeConfigPath() string {
	dir := os.Getenv("ProgramData")
	if dir == "" {
		dir = `C:\ProgramData`
	}
	return filepath.Join(dir, "VEIL", "active.json")
}

// Connect brings the tunnel up and records it as the active config.
func (h *persistingHandler) Connect(cfg, name string) error {
	if err := h.Tunnel.Connect(cfg, name); err != nil {
		return err
	}
	saveActive(activeConfig{Name: name, Config: cfg})
	return nil
}

// Disconnect tears the tunnel down and forgets the active config, so the
// sidecar does not reconnect it if respawned.
func (h *persistingHandler) Disconnect() error {
	if err := os.Remove(activeConfigPath()); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: clear active config: %v", err)
	}
	return h.Tunnel.Disconnect()
}

func saveActive(a activeConfig) {
	p := activeConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		log.Printf("warning: create state dir: %v", err)
		return
	}
	b, err := json.Marshal(a)
	if err != nil {
		return
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		log.Printf("warning: persist active config: %v", err)
	}
}

func loadActive() (activeConfig, bool) {
	b, err := os.ReadFile(activeConfigPath())
	if err != nil {
		return activeConfig{}, false
	}
	var a activeConfig
	if json.Unmarshal(b, &a) != nil || a.Config == "" {
		return activeConfig{}, false
	}
	return a, true
}

// ParseConfig parses .conf text into the structured shape the split-tunnel
// editor works with. Delegates entirely to github.com/veil-proto/veil/config
// (via configconv.go's toParsedConfig) rather than duplicating INI parsing
// in the frontend.
func (h *persistingHandler) ParseConfig(configText string) (ParsedConfig, error) {
	cfg, err := config.LoadConfigString(configText)
	if err != nil {
		return ParsedConfig{}, err
	}
	return toParsedConfig(cfg), nil
}

// SerializeConfig renders a structured config back into .conf text,
// validating it first so the frontend finds out about a bad edit (e.g. a
// malformed CIDR) before it ever reaches Connect.
func (h *persistingHandler) SerializeConfig(pc ParsedConfig) (string, error) {
	cfg, err := fromParsedConfig(pc)
	if err != nil {
		return "", err
	}
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	return cfg.Serialize(), nil
}

// autoReconnect restores the previously-active tunnel on sidecar start.
func autoReconnect(h control.Handler) {
	a, ok := loadActive()
	if !ok {
		return
	}
	log.Printf("restoring active tunnel %q", a.Name)
	if err := h.Connect(a.Config, a.Name); err != nil {
		log.Printf("auto-reconnect failed: %v", err)
	}
}
