//go:build windows

// Command veil-gui is the VEIL Windows GUI: a resizable window plus a tray
// icon that drive the veil-service tunnel over its control pipe. Double-click
// to launch; closing the window hides it to the tray (the tunnel keeps
// running in the service). Deliberately low-noise per the brandbook: one
// connection state, a config picker, traffic, link import, and .conf file
// import (file dialog or drag-and-drop), plus a structured split-tunnel
// editor and live service logs in their own tabs.
package main

import (
	_ "embed"
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/veil-proto/veil-windows/control"
)

//go:embed icon.png
var iconPNG []byte

var appIcon = fyne.NewStaticResource("veil.png", iconPNG)

const pollInterval = 1500 * time.Millisecond

// windowW/windowH are the starting size of the main window. The window is
// resizable (see main below) so the structured split-tunnel table has room
// to breathe. Bumped slightly wider/taller than before (480x680) to give the
// new card-based layout — which has more generous internal padding per the
// spaceLG spacing scale — room to sit comfortably without immediately
// engaging the VScroll on a typical desktop display.
const (
	windowW = 520
	windowH = 720
)

type ui struct {
	win fyne.Window

	// status block
	logo      *canvas.Image
	status    *statusBadge
	detail    *widget.Label
	traffic   *widget.Label
	handshake *widget.Label

	// config block
	configSel  *widget.Select
	configHint *widget.Label
	connectBtn *widget.Button
	discBtn    *widget.Button
	deleteBtn  *widget.Button

	// import block
	linkEntry *widget.Entry
	importBtn *widget.Button
	pasteBtn  *widget.Button
	fileBtn   *widget.Button

	// extra tabs
	configEditor *widget.Entry
	logViewer    *widget.Entry
	logsSince    uint64 // cursor for the last log line already appended to logViewer

	splitTunnel *splitTunnelTab

	configs []configEntry
}

func main() {
	a := app.NewWithID("proto.veil.gui")
	a.SetIcon(appIcon)
	a.Settings().SetTheme(veilTheme{})

	w := a.NewWindow("VEIL")
	w.SetIcon(appIcon)
	w.Resize(fyne.NewSize(windowW, windowH))
	// The window is resizable — the split-tunnel AllowedIPs/Disallowed table
	// needs room to grow, and the old fixed 440x640 window left no space for
	// it. Fyne has no direct "min size" window API; the effective floor
	// comes from the content's own MinSize (a padded VBox of labels/buttons/
	// a table with a sensible min row count), and every growing section is
	// wrapped in container.NewVScroll so content beyond that floor scrolls
	// instead of forcing the window to keep growing.

	u := &ui{win: w}
	w.SetContent(u.build())

	// Drag-and-drop a .conf file anywhere on the window to import it.
	w.SetOnDropped(u.onDropped)

	if desk, ok := a.(desktop.App); ok {
		menu := fyne.NewMenu("VEIL",
			fyne.NewMenuItem("Show", func() { w.Show(); w.RequestFocus() }),
			fyne.NewMenuItem("Connect", func() { u.connectSelected() }),
			fyne.NewMenuItem("Disconnect", func() { u.disconnect() }),
		)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(appIcon)
	}
	// Closing the window hides to tray; the service keeps the tunnel up.
	w.SetCloseIntercept(func() { w.Hide() })

	go u.pollLoop()
	w.ShowAndRun()
}

// build lays out the main window. The Connection tab is now three cards —
// status, configuration, and import — each a rounded/bordered surface built
// by newCard, stacked with spaceLG gaps instead of the old flat VBox +
// widget.Separator rules. The status card uses the shadcn "icon/indicator
// left, title+description+content right" horizontal layout: the app logo on
// the left, the status badge/detail/stats stacked on the right. Each card is
// padded so content never touches the window edges, and the whole tab is
// wrapped in a VScroll so a resize below the natural content height scrolls
// instead of clipping. Split Tunnel, Advanced, and Logs are separate tabs.
func (u *ui) build() fyne.CanvasObject {
	u.logo = canvas.NewImageFromResource(appIcon)
	u.logo.FillMode = canvas.ImageFillContain
	u.logo.SetMinSize(fyne.NewSize(56, 56))

	u.status = newStatusBadge()

	u.detail = newMuted("")
	u.detail.Alignment = fyne.TextAlignLeading
	u.detail.Wrapping = fyne.TextTruncate

	u.traffic = newMuted("—")
	u.traffic.Alignment = fyne.TextAlignLeading
	u.handshake = newMuted("—")
	u.handshake.Alignment = fyne.TextAlignLeading

	u.configSel = widget.NewSelect(nil, func(selected string) {
		if u.configEditor != nil {
			entry, ok := u.selectedConfig()
			if ok {
				text, _ := os.ReadFile(entry.Path)
				u.configEditor.SetText(string(text))
			} else {
				u.configEditor.SetText("")
			}
		}
	})
	u.configSel.PlaceHolder = "Select a config"
	u.refreshConfigs()

	u.configHint = newMuted("No configs yet — import one below.")
	u.configHint.Alignment = fyne.TextAlignLeading

	// Button hierarchy: Connect is the primary action (filled brand accent).
	// Disconnect is a secondary/lower-emphasis action. Delete is destructive.
	u.connectBtn = widget.NewButton("Connect", u.connectSelected)
	u.connectBtn.Importance = widget.HighImportance
	u.discBtn = widget.NewButton("Disconnect", u.disconnect)
	u.discBtn.Importance = widget.MediumImportance
	u.deleteBtn = widget.NewButton("Delete selected", u.deleteSelected)
	u.deleteBtn.Importance = widget.DangerImportance

	u.linkEntry = widget.NewEntry()
	u.linkEntry.SetPlaceHolder("veil://...")
	// Import link is the primary action of the import card; Paste and the
	// file picker are secondary/low-emphasis alternates to the same goal.
	u.importBtn = widget.NewButton("Import link", func() {
		u.handleImportLink(u.linkEntry.Text)
		u.linkEntry.SetText("")
	})
	u.importBtn.Importance = widget.HighImportance
	u.pasteBtn = widget.NewButton("Paste", func() {
		u.handleImportLink(u.win.Clipboard().Content())
	})
	u.pasteBtn.Importance = widget.LowImportance
	u.fileBtn = widget.NewButton("Import .conf file...", u.openConfDialog)
	u.fileBtn.Importance = widget.MediumImportance

	statsRow := container.NewGridWithColumns(2,
		container.NewVBox(newSectionLabel("Traffic"), u.traffic),
		container.NewVBox(newSectionLabel("Handshake"), u.handshake),
	)

	// Status card: logo on the left, badge + name/endpoint + stats stacked on
	// the right — the shadcn "avatar left, title/description right" pattern.
	statusRight := container.NewVBox(
		u.status,
		u.detail,
		vspace(spaceSM),
		statsRow,
	)
	statusCard := newCard(container.NewBorder(nil, nil, container.NewPadded(u.logo), nil, statusRight))

	configCard := newCard(container.NewVBox(
		cardTitle("Configuration"),
		cardDescription("Choose which tunnel config to connect with."),
		vspace(spaceSM),
		u.configSel,
		u.configHint,
		vspace(spaceXS),
		container.NewGridWithColumns(2, u.connectBtn, u.discBtn),
		u.deleteBtn,
	))

	importCard := newCard(container.NewVBox(
		cardTitle("Import a veil:// link"),
		cardDescription("Paste a link, or import a .conf file instead."),
		vspace(spaceSM),
		u.linkEntry,
		container.NewGridWithColumns(2, u.importBtn, u.pasteBtn),
		u.fileBtn,
	))

	// The window is no longer squeezed to a fixed 440x640, so cards get
	// spaceLG gaps and their own internal padding instead of everything
	// being crammed edge-to-edge behind bare separators. NewVScroll means a
	// taller-than-usual status/peer list (or a small window on a cramped
	// display) never clips content — it scrolls instead of overflowing.
	mainScreen := container.NewVScroll(container.New(
		&insetLayout{top: spaceLG, bottom: spaceLG, left: spaceLG, right: spaceLG},
		container.New(layout.NewCustomPaddedVBoxLayout(spaceLG), statusCard, configCard, importCard),
	))

	return container.NewAppTabs(
		container.NewTabItem("Connection", mainScreen),
		container.NewTabItem("Split Tunnel", u.buildSplitTunnelTab()),
		container.NewTabItem("Advanced", u.buildConfigTab()),
		container.NewTabItem("Logs", u.buildLogTab()),
	)
}

// pollLoop refreshes status every pollInterval; UI writes go through fyne.Do
// so they run on the main goroutine. It also polls for new log lines on the
// same cadence — logs use the same request/response named-pipe protocol as
// status, so a second long-lived streaming connection isn't worth the
// complexity; piggybacking on the existing poll tick keeps one cadence to
// reason about and one goroutine driving all periodic UI updates.
func (u *ui) pollLoop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		resp, err := status()
		fyne.Do(func() { u.apply(resp, err) })

		logs, lerr := fetchLogs(u.logsSince)
		if lerr == nil && len(logs) > 0 {
			u.logsSince = logs[len(logs)-1].Seq
			fyne.Do(func() { u.appendLogs(logs) })
		}

		<-ticker.C
	}
}

func (u *ui) apply(resp control.Response, err error) {
	if err != nil || !resp.OK || resp.Status == nil {
		u.setStatus("Service unavailable", dangerRed)
		u.detail.SetText("Install and start veil-service first.")
		u.traffic.SetText("—")
		u.handshake.SetText("—")
		return
	}
	st := resp.Status
	switch st.State {
	case control.StateConnected:
		u.setStatus("Connected", teal)
		var rx, tx uint64
		var hs int64
		ep := ""
		for _, p := range st.Peers {
			rx += p.RxBytes
			tx += p.TxBytes
			if p.LastHandshake > hs {
				hs = p.LastHandshake
			}
			if ep == "" {
				ep = p.Endpoint
			}
		}
		name := st.Name
		if name == "" {
			name = "tunnel"
		}
		if ep != "" {
			name += "  ·  " + ep
		}
		u.detail.SetText(name)
		u.traffic.SetText(fmt.Sprintf("↓ %s  ↑ %s", human(rx), human(tx)))
		u.handshake.SetText(ago(hs))
	case control.StateConnecting:
		u.setStatus("Connecting...", purple)
		u.detail.SetText(st.Name)
		u.traffic.SetText("—")
		u.handshake.SetText("—")
	default:
		u.setStatus("Disconnected", slate)
		u.detail.SetText("")
		u.traffic.SetText("—")
		u.handshake.SetText("—")
	}
}

// setStatus updates the status badge. It keeps its historic (text, color)
// signature — every call site throughout the file passes one of the brand
// colors below to mean a specific semantic state — and maps that color onto
// the nearest badge tone, so the badge/pill visual (dot + tinted pill) is
// driven by the same call sites as before without rewriting every caller to
// pass a statusTone directly.
func (u *ui) setStatus(text string, c color.Color) {
	u.status.SetState(text, toneFromColor(c))
}

// toneFromColor maps the legacy brand colors historically passed to
// setStatus onto the new badge's semantic tones.
func toneFromColor(c color.Color) statusTone {
	switch c {
	case teal, cyan:
		return tonePositive
	case purple, violet, indigo:
		return toneProgress
	case dangerRed:
		return toneDanger
	case warnAmber:
		return toneWarning
	default:
		return toneNeutral
	}
}

func (u *ui) refreshConfigs() {
	entries, err := listConfigs()
	if err != nil {
		u.configHint.SetText("Could not read config directory: " + err.Error())
		return
	}
	u.configs = entries
	opts := make([]string, 0, len(entries))
	for _, e := range entries {
		opts = append(opts, e.Name)
	}
	u.configSel.Options = opts

	wasSelected := u.configSel.Selected
	if (wasSelected == "" || !contains(opts, wasSelected)) && len(opts) > 0 {
		u.configSel.SetSelected(opts[0])
	} else if len(opts) == 0 {
		u.configSel.ClearSelected()
		u.configHint.SetText("No configs yet — import one below.")
	} else {
		u.configSel.Refresh()
		u.configHint.SetText(fmt.Sprintf("%d config(s) stored.", len(opts)))
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func (u *ui) selectedConfig() (configEntry, bool) {
	for _, e := range u.configs {
		if e.Name == u.configSel.Selected {
			return e, true
		}
	}
	return configEntry{}, false
}

func (u *ui) connectSelected() {
	entry, ok := u.selectedConfig()
	if !ok {
		u.setStatus("No config selected", warnAmber)
		return
	}
	text, err := os.ReadFile(entry.Path)
	if err != nil {
		u.setStatus("Config read failed", dangerRed)
		u.detail.SetText(err.Error())
		return
	}
	// Apply any GUI-only Disallowed-subnet carve-outs (Split Tunnel tab)
	// before the config text ever leaves the process. The on-disk config
	// and the sidecar are untouched by this — only the text sent to the
	// service is reduced.
	sendText := effectiveConfigText(entry.Path, string(text))
	go func() {
		resp, err := connect(sendText, entry.Name)
		fyne.Do(func() { u.apply(resp, err) })
	}()
}

func (u *ui) disconnect() {
	go func() {
		resp, err := disconnect()
		fyne.Do(func() { u.apply(resp, err) })
	}()
}

// deleteSelected asks for confirmation, then removes the config file from the
// store. The active tunnel (if any) is left untouched; the service keeps
// running with whatever config it already has until the user connects a
// different one or disconnects.
func (u *ui) deleteSelected() {
	entry, ok := u.selectedConfig()
	if !ok {
		return
	}
	dialog.ShowConfirm(
		"Delete config",
		fmt.Sprintf("Remove %q from the local store?\nThis does not disconnect the active tunnel.", entry.Name),
		func(ok bool) {
			if !ok {
				return
			}
			if _, err := deleteConfig(entry.Name); err != nil {
				u.setStatus("Delete failed", dangerRed)
				u.detail.SetText(err.Error())
				return
			}
			u.refreshConfigs()
			u.setStatus("Config deleted", cyan)
			u.detail.SetText("")
		},
		u.win,
	)
}

func (u *ui) handleImportLink(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	entry, err := importLink(raw)
	if err != nil {
		u.setStatus("Import failed", dangerRed)
		u.detail.SetText(err.Error())
		return
	}
	u.refreshConfigs()
	u.configSel.SetSelected(entry.Name)
	u.setStatus("Config imported", cyan)
	u.detail.SetText(entry.Name)
}

// openConfDialog opens a native file picker restricted to .conf files. After
// the user picks a file, it is copied into the store and selected.
func (u *ui) openConfDialog() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			u.setStatus("Import failed", dangerRed)
			u.detail.SetText(err.Error())
			return
		}
		if reader == nil {
			return // user cancelled
		}
		reader.Close()
		u.importURI(reader.URI())
	}, u.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".conf"}))
	fd.SetTitleText("Select a VEIL .conf file")
	fd.Show()
}

// importURI is the shared sink for both file-dialog and drag-and-drop imports.
func (u *ui) importURI(uri fyne.URI) {
	entry, err := importFromURI(uri)
	if err != nil {
		u.setStatus("Import failed", dangerRed)
		u.detail.SetText(err.Error())
		return
	}
	u.refreshConfigs()
	u.configSel.SetSelected(entry.Name)
	u.setStatus("Config imported", cyan)
	u.detail.SetText(entry.Name)
}

// onDropped handles files dropped onto the main window. Only .conf files are
// imported; everything else is ignored. The first successful import wins and
// gets selected.
func (u *ui) onDropped(_ fyne.Position, uris []fyne.URI) {
	for _, uri := range uris {
		if !strings.EqualFold(fileExt(uri.Name()), ".conf") {
			continue
		}
		u.importURI(uri)
		return
	}
	u.setStatus("Not a .conf file", warnAmber)
}

func fileExt(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return ""
	}
	return name[i:]
}
