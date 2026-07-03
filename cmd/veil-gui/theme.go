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
// The look is a shadcn/ui-inspired dark aesthetic — cards as the primary
// content unit, clear typography hierarchy, badges/pills for status — built
// entirely from Fyne's own canvas/container/widget primitives and rendered in
// the VEIL navy/indigo/violet/cyan palette below.
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

	// cardBg is the card surface fill — one step lighter than navy900, giving
	// every card a subtle "raised" look against the page background without
	// departing from the brand palette (shadcn's card-vs-background contrast,
	// rendered in navy).
	cardBg = rgb(0x0C, 0x17, 0x30)
	// cardBorder is a low-contrast 1px border, a touch brighter than the plain
	// separator color so card edges read clearly against cardBg.
	cardBorder = rgb(0x22, 0x2E, 0x54)
	// mutedFg approximates shadcn's ~70%-opacity muted foreground by blending
	// fgLight toward slate — used for secondary/description text inside cards.
	mutedFg = rgb(0x9A, 0xA5, 0xC3)
	// badge dot / pill danger tone (disconnected / error state).
	dangerRed = rgb(0xFF, 0x5D, 0x73)
	warnAmber = rgb(0xFF, 0xD1, 0x66)
)

// Spacing scale (logical px), applied consistently instead of ad hoc
// per-file padding. Roughly a 4px rhythm: xs/sm/md/lg/xl.
const (
	spaceXS = float32(4)
	spaceSM = float32(8)
	spaceMD = float32(12)
	spaceLG = float32(16)
	spaceXL = float32(24)

	cardRadius = float32(10)
	pillRadius = float32(999) // fully rounded pill
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

// Size tunes a handful of metrics for the VEIL brand look, following the
// spaceXS..spaceXL rhythm defined above (roughly a 4px scale) instead of
// arbitrary per-widget values:
//   - Padding: spaceSM (8) — the generic gap Fyne inserts around/between
//     widgets; kept modest so cards (which add their own internal padding)
//     don't end up double-padded.
//   - InnerPadding: spaceMD (12) — nested containers (e.g. inside a card)
//     get slightly more room to breathe than bare widget padding.
//   - Text: bumped very slightly so labels read at small sizes.
//   - InputBorder: thin to keep the input bar visually quiet.
func (veilTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return spaceSM
	case theme.SizeNameInnerPadding:
		return spaceMD
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	}
	return theme.DefaultTheme().Size(n)
}
