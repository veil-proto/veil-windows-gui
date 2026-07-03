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
// to breathe.
const (
	windowW = 480
	windowH = 680
)

type ui struct {
	win fyne.Window

	// status block
	logo      *canvas.Image
	status    *canvas.Text
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

// build lays out the main window. The Connection tab is divided into three
// card-style sections separated by thin rules (theme.ColorNameSeparator):
// status, connection control, and import. Each section is padded so content
// never touches the window edges, and the whole tab is wrapped in a
// VScroll so a resize below the natural content height scrolls instead of
// clipping. Split Tunnel, Advanced, and Logs are separate tabs alongside it.
func (u *ui) build() fyne.CanvasObject {
	u.logo = canvas.NewImageFromResource(appIcon)
	u.logo.FillMode = canvas.ImageFillContain
	u.logo.SetMinSize(fyne.NewSize(64, 64))

	u.status = canvas.NewText("Starting...", slate)
	u.status.TextSize = 20
	u.status.TextStyle = fyne.TextStyle{Bold: true}
	u.status.Alignment = fyne.TextAlignCenter

	u.detail = newMuted("")
	u.detail.Wrapping = fyne.TextTruncate

	u.traffic = newMuted("")
	u.handshake = newMuted("")

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
	u.configHint.Importance = widget.LowImportance

	u.connectBtn = widget.NewButton("Connect", u.connectSelected)
	u.connectBtn.Importance = widget.HighImportance
	u.discBtn = widget.NewButton("Disconnect", u.disconnect)
	u.deleteBtn = widget.NewButton("Delete selected", u.deleteSelected)
	u.deleteBtn.Importance = widget.DangerImportance

	u.linkEntry = widget.NewEntry()
	u.linkEntry.SetPlaceHolder("veil://...")
	u.importBtn = widget.NewButton("Import link", func() {
		u.handleImportLink(u.linkEntry.Text)
		u.linkEntry.SetText("")
	})
	u.pasteBtn = widget.NewButton("Paste", func() {
		u.handleImportLink(u.win.Clipboard().Content())
	})
	u.fileBtn = widget.NewButton("Import .conf file...", u.openConfDialog)

	statusBlock := container.NewVBox(
		container.NewCenter(u.logo),
		container.NewPadded(container.NewCenter(u.status)),
		u.detail,
	)

	statsBlock := container.NewGridWithColumns(2,
		container.NewVBox(
			newSectionLabel("Traffic"),
			u.traffic,
		),
		container.NewVBox(
			newSectionLabel("Handshake"),
			u.handshake,
		),
	)

	configBlock := container.NewVBox(
		newSectionLabel("Configuration"),
		u.configSel,
		u.configHint,
		container.NewGridWithColumns(2, u.connectBtn, u.discBtn),
		u.deleteBtn,
	)

	importBlock := container.NewVBox(
		newSectionLabel("Import a veil:// link"),
		u.linkEntry,
		container.NewGridWithColumns(2, u.importBtn, u.pasteBtn),
		widget.NewLabel("- or -"),
		u.fileBtn,
	)

	// The window is no longer squeezed to a fixed 440x640, so sections get a
	// visible separator plus their own padding instead of everything being
	// crammed edge-to-edge. NewVScroll means a taller-than-usual status/peer
	// list (or a small window on a cramped display) never clips content —
	// it scrolls instead of overflowing.
	mainScreen := container.NewVScroll(container.NewPadded(
		container.NewVBox(
			statusBlock,
			widget.NewSeparator(),
			container.NewPadded(statsBlock),
			widget.NewSeparator(),
			container.NewPadded(configBlock),
			widget.NewSeparator(),
			container.NewPadded(importBlock),
		),
	))

	return container.NewAppTabs(
		container.NewTabItem("Connection", mainScreen),
		container.NewTabItem("Split Tunnel", container.NewPadded(u.buildSplitTunnelTab())),
		container.NewTabItem("Advanced", container.NewPadded(u.buildConfigTab())),
		container.NewTabItem("Logs", container.NewPadded(u.buildLogTab())),
	)
}

func newMuted(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Alignment = fyne.TextAlignCenter
	return l
}

// newSectionLabel renders a small, uppercased heading used to delimit the
// three cards in the main window. Uppercase + slate colour signals "section
// label" without competing with the status text above.
func newSectionLabel(text string) fyne.CanvasObject {
	l := widget.NewLabel(strings.ToUpper(text))
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
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
		u.setStatus("Service unavailable", rgb(0xFF, 0x5D, 0x73))
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

func (u *ui) setStatus(text string, c color.Color) {
	u.status.Text = text
	u.status.Color = c
	u.status.Refresh()
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
		u.setStatus("No config selected", rgb(0xFF, 0xD1, 0x66))
		return
	}
	text, err := os.ReadFile(entry.Path)
	if err != nil {
		u.setStatus("Config read failed", rgb(0xFF, 0x5D, 0x73))
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
				u.setStatus("Delete failed", rgb(0xFF, 0x5D, 0x73))
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
		u.setStatus("Import failed", rgb(0xFF, 0x5D, 0x73))
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
			u.setStatus("Import failed", rgb(0xFF, 0x5D, 0x73))
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
		u.setStatus("Import failed", rgb(0xFF, 0x5D, 0x73))
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
	u.setStatus("Not a .conf file", rgb(0xFF, 0xD1, 0x66))
}

func fileExt(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return ""
	}
	return name[i:]
}
