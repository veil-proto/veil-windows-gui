//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/veil-proto/veil-windows/control"
)

// buildConfigTab is the raw .conf text editor — the on-disk source of truth,
// unchanged in behavior from before. It's labeled "Advanced" in the tab bar
// now that the Split Tunnel tab offers a structured alternative for the one
// thing most users actually need to edit (AllowedIPs), but this tab still
// edits the whole file verbatim with zero validation, same as always.
//
// Laid out as a single card with a header row (title + description) on top,
// the editor filling the middle, and a toolbar row along the bottom holding
// the Save Changes button — the same "card with header/toolbar" treatment
// used for the Logs tab, so the two simplest tabs read as a matched pair.
func (u *ui) buildConfigTab() fyne.CanvasObject {
	u.configEditor = widget.NewMultiLineEntry()
	u.configEditor.SetPlaceHolder("Select a config from the Connection tab to edit...")

	saveBtn := widget.NewButton("Save Changes", u.saveConfigEditor)
	saveBtn.Importance = widget.HighImportance

	header := container.NewVBox(
		cardTitle("Edit Config"),
		cardDescription("Raw .conf text — the on-disk source of truth. No validation."),
	)
	toolbar := container.NewBorder(nil, nil, nil, saveBtn)

	body := container.NewBorder(
		container.NewVBox(header, vspace(spaceSM)),
		container.NewVBox(vspace(spaceSM), toolbar),
		nil, nil,
		u.configEditor,
	)

	return container.New(&insetLayout{top: spaceLG, bottom: spaceLG, left: spaceLG, right: spaceLG}, newCard(body))
}

// buildLogTab shows live veil-service logs, polled and appended to alongside
// the status poll in pollLoop (see main.go). The log buffer is process-
// lifetime on the service side, so switching tabs or reopening the window
// never loses history — only a service restart resets the cursor.
//
// Same card-with-header/toolbar shape as the Advanced tab: title+description
// on top, the scrolling log view filling the middle, "Clear view" (a
// low-emphasis action — it only clears the local view, not the service-side
// buffer) along the bottom.
func (u *ui) buildLogTab() fyne.CanvasObject {
	u.logViewer = widget.NewMultiLineEntry()
	u.logViewer.Disable()
	u.logViewer.SetText("Waiting for veil-service logs...")
	u.logViewer.Wrapping = fyne.TextWrapWord

	clearBtn := widget.NewButton("Clear view", func() {
		u.logViewer.SetText("")
	})
	clearBtn.Importance = widget.LowImportance

	header := container.NewVBox(
		cardTitle("Service Logs"),
		cardDescription("Live output from veil-service, polled every few seconds."),
	)
	toolbar := container.NewBorder(nil, nil, nil, clearBtn)

	scroll := container.NewVScroll(u.logViewer)

	body := container.NewBorder(
		container.NewVBox(header, vspace(spaceSM)),
		container.NewVBox(vspace(spaceSM), toolbar),
		nil, nil,
		scroll,
	)

	return container.New(&insetLayout{top: spaceLG, bottom: spaceLG, left: spaceLG, right: spaceLG}, newCard(body))
}

// appendLogs formats and appends newly-polled log lines to the log viewer.
// Called on the main goroutine from pollLoop via fyne.Do.
func (u *ui) appendLogs(lines []control.LogLine) {
	if u.logViewer == nil || len(lines) == 0 {
		return
	}
	var b strings.Builder
	// First real batch of logs replaces the "Waiting for..." placeholder.
	if u.logViewer.Text != "" && !strings.HasPrefix(u.logViewer.Text, "Waiting for") {
		b.WriteString(u.logViewer.Text)
	}
	for _, l := range lines {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(formatLogLine(l))
	}
	u.logViewer.SetText(b.String())
	u.logViewer.CursorRow = strings.Count(b.String(), "\n")
	u.logViewer.Refresh()
}

func formatLogLine(l control.LogLine) string {
	ts := time.Unix(l.Time, 0).Format("15:04:05")
	if l.Level != "" {
		return fmt.Sprintf("[%s] %s: %s", ts, l.Level, l.Msg)
	}
	return fmt.Sprintf("[%s] %s", ts, l.Msg)
}

func (u *ui) saveConfigEditor() {
	entry, ok := u.selectedConfig()
	if !ok {
		u.setStatus("No config selected to save", rgb(0xFF, 0xD1, 0x66))
		return
	}
	text := u.configEditor.Text
	err := os.WriteFile(entry.Path, []byte(text), 0600)
	if err != nil {
		u.setStatus("Failed to save config", rgb(0xFF, 0x5D, 0x73))
		u.detail.SetText(err.Error())
		return
	}
	u.setStatus("Config saved", cyan)
	u.detail.SetText(entry.Name)
}
