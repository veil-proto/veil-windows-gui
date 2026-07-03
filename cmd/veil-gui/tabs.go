//go:build windows

package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (u *ui) buildConfigTab() fyne.CanvasObject {
	u.configEditor = widget.NewMultiLineEntry()
	u.configEditor.SetPlaceHolder("Select a config from the Connection tab to edit...")

	saveBtn := widget.NewButton("Save Changes", u.saveConfigEditor)
	saveBtn.Importance = widget.HighImportance

	return container.NewBorder(
		newSectionLabel("Edit Config"),
		container.NewPadded(saveBtn),
		nil,
		nil,
		u.configEditor,
	)
}

func (u *ui) buildLogTab() fyne.CanvasObject {
	u.logViewer = widget.NewMultiLineEntry()
	// Read-only, but Fyne's MultiLineEntry allows selection.
	// We'll just update it dynamically.
	u.logViewer.Disable()
	u.logViewer.SetText("Waiting for veil-service logs...")

	return container.NewBorder(
		newSectionLabel("Service Logs"),
		nil, nil, nil,
		u.logViewer,
	)
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
