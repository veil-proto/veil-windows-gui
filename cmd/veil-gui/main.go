//go:build windows

// Command veil-gui is the VEIL Windows GUI: a small window plus a tray icon that
// drive the veil-service tunnel over its control pipe. Double-click to launch;
// closing the window hides it to the tray (the tunnel keeps running in the
// service). Deliberately low-noise per the brandbook: one connection state, a
// config picker, traffic, and link import.
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
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/veil-proto/veil-windows/control"
)

//go:embed icon.png
var iconPNG []byte

var appIcon = fyne.NewStaticResource("veil.png", iconPNG)

const pollInterval = 1500 * time.Millisecond

type ui struct {
	win       fyne.Window
	logo      *canvas.Image
	status    *canvas.Text
	detail    *widget.Label
	traffic   *widget.Label
	handshake *widget.Label
	configSel *widget.Select
	linkEntry *widget.Entry
	configs   []configEntry
}

func main() {
	a := app.NewWithID("proto.veil.gui")
	a.SetIcon(appIcon)
	a.Settings().SetTheme(veilTheme{})

	w := a.NewWindow("VEIL")
	w.SetIcon(appIcon)
	w.Resize(fyne.NewSize(420, 500))

	u := &ui{win: w}
	w.SetContent(u.build())

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

func (u *ui) build() fyne.CanvasObject {
	u.logo = canvas.NewImageFromResource(appIcon)
	u.logo.FillMode = canvas.ImageFillContain
	u.logo.SetMinSize(fyne.NewSize(76, 76))

	u.status = canvas.NewText("Starting...", slate)
	u.status.TextSize = 18
	u.status.TextStyle = fyne.TextStyle{Bold: true}
	u.status.Alignment = fyne.TextAlignCenter

	u.detail = newMuted("")
	u.traffic = newMuted("")
	u.handshake = newMuted("")

	u.configSel = widget.NewSelect(nil, nil)
	u.configSel.PlaceHolder = "Select a config"
	u.refreshConfigs()

	connectBtn := widget.NewButton("Connect", u.connectSelected)
	connectBtn.Importance = widget.HighImportance
	disconnectBtn := widget.NewButton("Disconnect", u.disconnect)

	u.linkEntry = widget.NewEntry()
	u.linkEntry.SetPlaceHolder("veil://...")
	importBtn := widget.NewButton("Import link", func() {
		u.handleImport(u.linkEntry.Text)
		u.linkEntry.SetText("")
	})
	pasteBtn := widget.NewButton("Paste", func() {
		u.handleImport(u.win.Clipboard().Content())
	})

	return container.NewVBox(
		container.NewCenter(u.logo),
		container.NewCenter(u.status),
		container.NewCenter(u.detail),
		widget.NewSeparator(),
		container.NewCenter(u.traffic),
		container.NewCenter(u.handshake),
		widget.NewSeparator(),
		u.configSel,
		container.NewGridWithColumns(2, connectBtn, disconnectBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Import a veil:// link", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		u.linkEntry,
		container.NewGridWithColumns(2, importBtn, pasteBtn),
	)
}

func newMuted(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Alignment = fyne.TextAlignCenter
	return l
}

// pollLoop refreshes status every pollInterval; UI writes go through fyne.Do so
// they run on the main goroutine.
func (u *ui) pollLoop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		resp, err := status()
		fyne.Do(func() { u.apply(resp, err) })
		<-ticker.C
	}
}

func (u *ui) apply(resp control.Response, err error) {
	if err != nil || !resp.OK || resp.Status == nil {
		u.setStatus("Service unavailable", rgb(0xFF, 0x5D, 0x73))
		u.detail.SetText("Install and start veil-service first.")
		u.traffic.SetText("")
		u.handshake.SetText("")
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
			name += "  -  " + ep
		}
		u.detail.SetText(name)
		u.traffic.SetText(fmt.Sprintf("down %s     up %s", human(rx), human(tx)))
		u.handshake.SetText("handshake " + ago(hs))
	case control.StateConnecting:
		u.setStatus("Connecting...", purple)
		u.detail.SetText(st.Name)
		u.traffic.SetText("")
		u.handshake.SetText("")
	default:
		u.setStatus("Disconnected", slate)
		u.detail.SetText("")
		u.traffic.SetText("")
		u.handshake.SetText("")
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
		return
	}
	u.configs = entries
	opts := make([]string, 0, len(entries))
	for _, e := range entries {
		opts = append(opts, e.Name)
	}
	u.configSel.Options = opts
	if u.configSel.Selected == "" && len(opts) > 0 {
		u.configSel.SetSelected(opts[0])
	} else {
		u.configSel.Refresh()
	}
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
	go func() {
		resp, err := connect(string(text), entry.Name)
		fyne.Do(func() { u.apply(resp, err) })
	}()
}

func (u *ui) disconnect() {
	go func() {
		resp, err := disconnect()
		fyne.Do(func() { u.apply(resp, err) })
	}()
}

func (u *ui) handleImport(raw string) {
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
