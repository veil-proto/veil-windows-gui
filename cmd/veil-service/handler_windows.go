//go:build windows

package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/veil-proto/veil-windows/control"
	"github.com/veil-proto/veil-windows/wintunnel"
)

// persistingHandler wraps the tunnel controller and remembers the active config
// on disk, so the service reconnects the last tunnel after a crash-restart or a
// reboot without the tray needing to be running.
type persistingHandler struct {
	*wintunnel.Tunnel
}

func newHandler() *persistingHandler {
	return &persistingHandler{Tunnel: &wintunnel.Tunnel{}}
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

// Disconnect tears the tunnel down and forgets the active config, so the service
// does not reconnect it on the next start.
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

// autoReconnect restores the previously-active tunnel on service start.
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
