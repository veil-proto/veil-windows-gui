//go:build windows

package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// veilTheme is the VEIL brand theme: dark navy surfaces, indigo/violet controls,
// per the brandbook. Always dark, regardless of the OS setting.
//
// Sizes are overridden to produce a denser layout that fits the fixed 440x640
// window without internal scrolling.
type veilTheme struct{}

func rgb(r, g, b uint8) color.Color { return color.NRGBA{R: r, G: g, B: b, A: 255} }

// Brand palette (brandbook section 3).
var (
	navy900   = rgb(0x07, 0x11, 0x26)
	navy950   = rgb(0x05, 0x08, 0x14)
	indigo    = rgb(0x34, 0x3B, 0x8F)
	violet    = rgb(0x6D, 0x43, 0xE6)
	purple    = rgb(0x87, 0x58, 0xFF)
	cyan      = rgb(0x25, 0xD7, 0xE8)
	teal      = rgb(0x46, 0xF0, 0xE5)
	slate     = rgb(0x53, 0x62, 0x7F)
	fgLight   = rgb(0xE6, 0xEA, 0xF5)
	inputBg   = rgb(0x0A, 0x18, 0x33)
	separator = rgb(0x1A, 0x24, 0x44)
)

func (veilTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return navy900
	case theme.ColorNameForeground:
		return fgLight
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return violet
	case theme.ColorNameHover:
		return purple
	case theme.ColorNameButton:
		return indigo
	case theme.ColorNameInputBackground:
		return inputBg
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return slate
	case theme.ColorNameSeparator:
		return separator
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return navy950
	case theme.ColorNameShadow:
		return color.NRGBA{A: 130}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x6D, G: 0x43, B: 0xE6, A: 90}
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (veilTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (veilTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

// Size tunes a handful of metrics for the VEIL brand look. The window used to
// be fixed at 440x640, which forced very tight padding to avoid clipping;
// now that it's resizable (with scrollable sections absorbing overflow), the
// padding can breathe a little more without risking clipped content:
//   - Padding: denser than Fyne's default, but no longer squeezed to the bone
//   - InnerPadding: same for nested containers
//   - Text: bumped very slightly so labels read at small sizes
//   - InputBorder: thin to keep the input bar visually quiet
func (veilTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInputBorder:
		return 1
	}
	return theme.DefaultTheme().Size(n)
}
