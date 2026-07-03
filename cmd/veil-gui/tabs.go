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
func (u *ui) buildConfigTab() fyne.CanvasObject {
	u.configEditor = widget.NewMultiLineEntry()
	u.configEditor.SetPlaceHolder("Select a config from the Connection tab to edit...")

	saveBtn := widget.NewButton("Save Changes", u.saveConfigEditor)
	saveBtn.Importance = widget.HighImportance

	return container.NewBorder(
		newSectionLabel("Edit Config (Advanced)"),
		container.NewPadded(saveBtn),
		nil,
		nil,
		u.configEditor,
	)
}

// buildLogTab shows live veil-service logs, polled and appended to alongside
// the status poll in pollLoop (see main.go). The log buffer is process-
// lifetime on the service side, so switching tabs or reopening the window
// never loses history — only a service restart resets the cursor.
func (u *ui) buildLogTab() fyne.CanvasObject {
	u.logViewer = widget.NewMultiLineEntry()
	u.logViewer.Disable()
	u.logViewer.SetText("Waiting for veil-service logs...")
	u.logViewer.Wrapping = fyne.TextWrapWord

	clearBtn := widget.NewButton("Clear view", func() {
		u.logViewer.SetText("")
	})

	scroll := container.NewVScroll(u.logViewer)

	return container.NewBorder(
		newSectionLabel("Service Logs"),
		container.NewPadded(clearBtn),
		nil,
		nil,
		scroll,
	)
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
		ts := time.Unix(l.Time, 0).Format("15:04:05")
		if l.Level != "" {
			fmt.Fprintf(&b, "[%s] %s: %s", ts, l.Level, l.Msg)
		} else {
			fmt.Fprintf(&b, "[%s] %s", ts, l.Msg)
		}
	}
	u.logViewer.SetText(b.String())
	u.logViewer.CursorRow = strings.Count(b.String(), "\n")
	u.logViewer.Refresh()
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
