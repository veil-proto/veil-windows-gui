//go:build windows

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/veil-proto/veil/config"
)

// splitTunnelTab is the structured Allowed/Disallowed subnet editor. Unlike
// the Advanced tab (raw .conf text), this tab understands the config's peer
// structure and edits AllowedIPs (a real config.PeerConfig field) plus
// Disallowed (a GUI-only concept, see disallowed.go) through form controls
// with inline CIDR validation instead of free text.
//
// The tab operates on whatever config is selected in the Connection tab
// (u.selectedConfig()); it has no independent config picker of its own to
// avoid two out-of-sync "current config" notions in the same window.
type splitTunnelTab struct {
	u *ui

	root fyne.CanvasObject

	entryLabel *widget.Label
	peerList   *fyne.Container // one card per peer, rebuilt on refresh

	cfg        *config.Config
	confPath   string
	disallowed disallowedDoc
}

// buildSplitTunnelTab constructs the tab and wires a refresh whenever the
// user switches configs in the Connection tab's selector.
func (u *ui) buildSplitTunnelTab() fyne.CanvasObject {
	st := &splitTunnelTab{u: u}
	u.splitTunnel = st

	st.entryLabel = newMuted("Select a config from the Connection tab.")
	st.peerList = container.NewVBox()

	refreshBtn := widget.NewButton("Reload from config", func() { st.refresh() })

	content := container.NewVScroll(container.NewVBox(
		newSectionLabel("Split Tunnel — Allowed / Disallowed Subnets"),
		st.entryLabel,
		widget.NewSeparator(),
		st.peerList,
	))

	st.root = container.NewBorder(nil, container.NewPadded(refreshBtn), nil, nil, content)

	// Hook the existing config selector so switching configs on the
	// Connection tab also refreshes this tab, without this tab needing its
	// own picker.
	if u.configSel != nil {
		prevOnChanged := u.configSel.OnChanged
		u.configSel.OnChanged = func(s string) {
			if prevOnChanged != nil {
				prevOnChanged(s)
			}
			st.refresh()
		}
	}

	return st.root
}

// refresh reloads the currently-selected config and its Disallowed sidecar,
// then rebuilds the per-peer cards.
func (st *splitTunnelTab) refresh() {
	u := st.u
	entry, ok := u.selectedConfig()
	if !ok {
		st.cfg, st.confPath = nil, ""
		st.entryLabel.SetText("Select a config from the Connection tab.")
		st.peerList.Objects = nil
		st.peerList.Refresh()
		return
	}

	text, err := os.ReadFile(entry.Path)
	if err != nil {
		st.entryLabel.SetText("Could not read config: " + err.Error())
		return
	}
	cfg, err := config.LoadConfigString(string(text))
	if err != nil {
		st.entryLabel.SetText("Config is invalid, fix it in the Advanced tab first: " + err.Error())
		st.cfg, st.confPath = nil, ""
		st.peerList.Objects = nil
		st.peerList.Refresh()
		return
	}
	doc, err := loadDisallowed(entry.Path)
	if err != nil {
		st.entryLabel.SetText("Could not read disallowed sidecar: " + err.Error())
		return
	}

	st.cfg, st.confPath, st.disallowed = cfg, entry.Path, doc
	st.entryLabel.SetText(fmt.Sprintf("%s  ·  %d peer(s)", entry.Name, len(cfg.Peers)))
	st.rebuildPeerCards()
}

// rebuildPeerCards regenerates one card per peer from st.cfg/st.disallowed.
func (st *splitTunnelTab) rebuildPeerCards() {
	st.peerList.Objects = nil
	for i := range st.cfg.Peers {
		st.peerList.Add(st.buildPeerCard(i))
	}
	if len(st.cfg.Peers) == 0 {
		st.peerList.Add(newMuted("This config has no peers."))
	}
	st.peerList.Refresh()
}

// peerKeyHex returns the hex-encoded public key used both for display and as
// the Disallowed sidecar's per-peer map key.
func peerKeyHex(p config.PeerConfig) string {
	return hex.EncodeToString(p.PublicKey)
}

// truncateKey shortens a hex public key for display: first 8 + last 8 hex
// chars, matching how most WireGuard-style UIs show keys without eating the
// whole card width.
func truncateKey(hexKey string) string {
	if len(hexKey) <= 20 {
		return hexKey
	}
	return hexKey[:8] + "…" + hexKey[len(hexKey)-8:]
}

// buildPeerCard renders one peer's public key plus its AllowedIPs and
// Disallowed CIDR lists as editable rows.
func (st *splitTunnelTab) buildPeerCard(peerIdx int) fyne.CanvasObject {
	p := &st.cfg.Peers[peerIdx]
	keyHex := peerKeyHex(*p)

	keyLabel := widget.NewLabel(truncateKey(keyHex))
	keyLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	copyBtn := widget.NewButton("Copy key", func() {
		st.u.win.Clipboard().SetContent(keyHex)
	})

	allowedList := container.NewVBox()
	disallowedList := container.NewVBox()

	var rebuildAllowed, rebuildDisallowed func()

	rebuildAllowed = func() {
		allowedList.Objects = nil
		for i, cidr := range p.AllowedIPs {
			allowedList.Add(st.cidrRow(cidr, func() {
				p.AllowedIPs = append(p.AllowedIPs[:i], p.AllowedIPs[i+1:]...)
				st.persistConfig()
				rebuildAllowed()
			}))
		}
		allowedList.Refresh()
	}
	rebuildDisallowed = func() {
		disallowedList.Objects = nil
		cur := st.disallowed.PerPeer[keyHex]
		for i, cidr := range cur {
			disallowedList.Add(st.cidrRow(cidr, func() {
				next := append(cur[:i:i], cur[i+1:]...)
				if len(next) == 0 {
					delete(st.disallowed.PerPeer, keyHex)
				} else {
					st.disallowed.PerPeer[keyHex] = next
				}
				st.persistDisallowed()
				rebuildDisallowed()
			}))
		}
		disallowedList.Refresh()
	}
	rebuildAllowed()
	rebuildDisallowed()

	allowedAddBtn := widget.NewButton("+ Add CIDR", func() {
		st.showAddDialog("Add to AllowedIPs", func(cidrs []string) {
			p.AllowedIPs = append(p.AllowedIPs, cidrs...)
			st.persistConfig()
			rebuildAllowed()
		})
	})
	disallowedAddBtn := widget.NewButton("+ Add CIDR", func() {
		st.showAddDialog("Add to Disallowed", func(cidrs []string) {
			cur := st.disallowed.PerPeer[keyHex]
			st.disallowed.PerPeer[keyHex] = append(cur, cidrs...)
			st.persistDisallowed()
			rebuildDisallowed()
		})
	})

	header := container.NewBorder(nil, nil, keyLabel, copyBtn)

	allowedSection := container.NewVBox(
		newSectionLabel("Allowed IPs"),
		allowedList,
		allowedAddBtn,
	)
	disallowedSection := container.NewVBox(
		newSectionLabel("Disallowed (GUI-only; carved out at connect time)"),
		disallowedList,
		disallowedAddBtn,
	)

	card := container.NewVBox(
		header,
		container.NewGridWithColumns(2, allowedSection, disallowedSection),
		widget.NewSeparator(),
	)
	return card
}

// cidrRow renders one CIDR entry with a remove button.
func (st *splitTunnelTab) cidrRow(cidr string, onRemove func()) fyne.CanvasObject {
	label := widget.NewLabel(cidr)
	label.TextStyle = fyne.TextStyle{Monospace: true}
	removeBtn := widget.NewButton("×", onRemove)
	removeBtn.Importance = widget.LowImportance
	return container.NewBorder(nil, nil, nil, removeBtn, label)
}

// showAddDialog offers both a single-CIDR quick-add and a bulk paste box
// (newline- or comma-separated), validating every entry as a CIDR before
// calling onAdd. Invalid entries are reported and none are added, so a typo
// in a batch of ten doesn't silently drop just that one.
func (st *splitTunnelTab) showAddDialog(title string, onAdd func(cidrs []string)) {
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder("10.0.0.0/24\n192.168.1.0/24, 172.16.0.0/12\n...")
	entry.Wrapping = fyne.TextWrapWord

	hint := newMuted("One CIDR per line, or comma-separated.")
	hint.Importance = widget.LowImportance

	content := container.NewVBox(hint, entry)

	d := dialog.NewCustomConfirm(title, "Add", "Cancel", content, func(confirmed bool) {
		if !confirmed {
			return
		}
		cidrs, bad := parseCIDRList(entry.Text)
		if len(bad) > 0 {
			dialog.ShowError(fmt.Errorf("invalid CIDR(s), nothing was added: %s", strings.Join(bad, ", ")), st.u.win)
			return
		}
		if len(cidrs) == 0 {
			return
		}
		onAdd(cidrs)
	}, st.u.win)
	d.Resize(fyne.NewSize(420, 300))
	d.Show()
}

// parseCIDRList splits raw on newlines and commas, trims whitespace, and
// validates each resulting entry as a CIDR (reusing config's own validation
// logic via a throwaway PeerConfig, so "what counts as a valid CIDR" has
// exactly one definition in the whole codebase). Returns the valid CIDRs and
// a separate list of the invalid raw tokens.
func parseCIDRList(raw string) (valid []string, invalid []string) {
	raw = strings.ReplaceAll(raw, ",", "\n")
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		probe := config.PeerConfig{
			PublicKey:  make([]byte, 32), // dummy, just to isolate the AllowedIPs check
			AllowedIPs: []string{line},
		}
		if err := probe.Validate(); err != nil {
			invalid = append(invalid, line)
			continue
		}
		valid = append(valid, line)
	}
	return valid, invalid
}

// persistConfig re-serializes st.cfg back to its .conf file. This tab edits
// AllowedIPs directly on the loaded config.Config, so any change needs to be
// written back through config.Serialize() to become the new on-disk source
// of truth (the same file the Advanced tab and Connect both read).
func (st *splitTunnelTab) persistConfig() {
	if st.cfg == nil || st.confPath == "" {
		return
	}
	if err := os.WriteFile(st.confPath, []byte(st.cfg.Serialize()), 0o600); err != nil {
		dialog.ShowError(fmt.Errorf("save config: %w", err), st.u.win)
		return
	}
	// Keep the Advanced tab's editor in sync if it's showing this config.
	if st.u.configEditor != nil {
		if entry, ok := st.u.selectedConfig(); ok && entry.Path == st.confPath {
			st.u.configEditor.SetText(st.cfg.Serialize())
		}
	}
}

func (st *splitTunnelTab) persistDisallowed() {
	if st.confPath == "" {
		return
	}
	if err := saveDisallowed(st.confPath, st.disallowed); err != nil {
		dialog.ShowError(fmt.Errorf("save disallowed subnets: %w", err), st.u.win)
	}
}

// effectiveConfigText returns the config text that should actually be sent
// to the service on Connect: the on-disk config with each peer's AllowedIPs
// reduced by that peer's Disallowed CIDRs (see disallowed.go). If loading or
// parsing fails, or there's no Disallowed data for this config, the original
// text is returned unchanged — the Disallowed feature only ever narrows
// AllowedIPs, so any failure here should be "fall back to the raw config",
// never a hard error, since the Advanced/Connection tabs already validate
// the base config independently.
func effectiveConfigText(confPath, rawText string) string {
	doc, err := loadDisallowed(confPath)
	if err != nil || len(doc.PerPeer) == 0 {
		return rawText
	}
	cfg, err := config.LoadConfigString(rawText)
	if err != nil {
		return rawText
	}
	changed := false
	for i := range cfg.Peers {
		key := peerKeyHex(cfg.Peers[i])
		disallowed, ok := doc.PerPeer[key]
		if !ok || len(disallowed) == 0 {
			continue
		}
		cfg.Peers[i].AllowedIPs = subtractCIDRs(cfg.Peers[i].AllowedIPs, disallowed)
		changed = true
	}
	if !changed {
		return rawText
	}
	return cfg.Serialize()
}
