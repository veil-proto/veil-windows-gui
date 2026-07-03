//go:build windows

package main

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// This file holds the shadcn/ui-inspired design primitives shared by every
// tab: cards, section/muted typography, and a status badge/pill. Fyne has no
// built-in bordered-rounded-card widget, so a "card" here is the standard
// Fyne pattern for that look: a canvas.Rectangle (fill + 1px stroke + corner
// radius) stacked behind the real content via container.NewStack, with the
// content itself inset by a fixed padding layout so it never touches the
// rounded edge.

// newCard wraps content in a rounded, bordered surface: cardBg fill,
// cardBorder 1px stroke, cardRadius corners, spaceLG padding on all sides.
// This is the primary grouping unit across all four tabs — connection
// status, each split-tunnel peer, and the editor/log toolbars all sit in one
// of these instead of being separated by bare widget.Separator rules.
func newCard(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(cardBg)
	bg.StrokeColor = cardBorder
	bg.StrokeWidth = 1
	bg.CornerRadius = cardRadius

	padded := container.New(
		&insetLayout{top: spaceLG, bottom: spaceLG, left: spaceLG, right: spaceLG},
		content,
	)
	return container.NewStack(bg, padded)
}

// newCardTight is newCard with smaller (spaceMD) padding, used for cards
// whose content is already dense (e.g. a peer card that stacks a header plus
// a two-column CIDR grid) so the outer inset doesn't eat too much width.
func newCardTight(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(cardBg)
	bg.StrokeColor = cardBorder
	bg.StrokeWidth = 1
	bg.CornerRadius = cardRadius

	padded := container.New(
		&insetLayout{top: spaceMD, bottom: spaceMD, left: spaceMD, right: spaceMD},
		content,
	)
	return container.NewStack(bg, padded)
}

// insetLayout insets a single child by fixed logical-pixel margins on each
// side. container.NewPadded uses the theme's padding metric (spaceSM, shared
// with general widget spacing); cards want a larger and constant inset
// regardless of that shared metric, so they get their own tiny layout.
type insetLayout struct {
	top, bottom, left, right float32
}

func (l *insetLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	child := objects[0]
	child.Move(fyne.NewPos(l.left, l.top))
	child.Resize(fyne.NewSize(size.Width-l.left-l.right, size.Height-l.top-l.bottom))
}

func (l *insetLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	min := objects[0].MinSize()
	return fyne.NewSize(min.Width+l.left+l.right, min.Height+l.top+l.bottom)
}

// cardTitle renders a card's title: semibold, slightly larger than body
// text, fgLight. This is the top of the typography hierarchy inside a card.
func cardTitle(text string) *widget.Label {
	l := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return l
}

// cardDescription renders secondary/muted text under a card title — smaller
// and dimmer than body copy, approximating shadcn's muted-foreground tone.
func cardDescription(text string) *canvas.Text {
	t := canvas.NewText(text, mutedFg)
	t.TextSize = 12
	return t
}

// newMuted keeps its previous call sites working (secondary inline text used
// throughout the tabs) but now draws from the same mutedFg tone the cards
// use, instead of the plain default foreground color a bare widget.Label
// would use.
func newMuted(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Alignment = fyne.TextAlignCenter
	l.Importance = widget.LowImportance
	return l
}

// newSectionLabel renders a small, uppercased heading used inside a card to
// delimit a sub-section (e.g. "Allowed IPs" inside a peer card). Uppercase +
// muted tone signals "section label" without competing with the card title.
func newSectionLabel(text string) fyne.CanvasObject {
	// Plain uppercase; Fyne has no letter-spacing knob, so this is as close
	// to shadcn's tracked-out uppercase eyebrow text as the toolkit allows.
	t := canvas.NewText(strings.ToUpper(text), mutedFg)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 12
	return t
}

// statusTone is the semantic color family for a badge/pill.
type statusTone int

const (
	toneNeutral  statusTone = iota // disconnected / idle — slate
	toneProgress                   // connecting — violet/purple
	tonePositive                   // connected — cyan/teal
	toneDanger                     // error / service unavailable — red
	toneWarning                    // warning — amber
)

func toneColor(t statusTone) color.Color {
	switch t {
	case tonePositive:
		return teal
	case toneProgress:
		return purple
	case toneDanger:
		return dangerRed
	case toneWarning:
		return warnAmber
	default:
		return slate
	}
}

// statusBadge is a small rounded pill with a colored dot plus label — the
// shadcn "badge" treatment applied to connection state, replacing the old
// plain recolored canvas.Text. Dot and text share the tone color; the pill
// background is a translucent wash of the same tone over cardBg so the badge
// reads as "tinted", not just "colored text in a box".
type statusBadge struct {
	widget.BaseWidget

	dot  *canvas.Circle
	text *canvas.Text
	bg   *canvas.Rectangle
}

func newStatusBadge() *statusBadge {
	b := &statusBadge{
		dot:  canvas.NewCircle(slate),
		text: canvas.NewText("Starting…", fgLight),
		bg:   canvas.NewRectangle(washColor(slate)),
	}
	b.text.TextStyle = fyne.TextStyle{Bold: true}
	b.text.TextSize = 13
	b.bg.CornerRadius = pillRadius
	b.bg.StrokeColor = washColor(slate)
	b.bg.StrokeWidth = 1
	b.ExtendBaseWidget(b)
	return b
}

// SetState updates the badge's label and tone in one call.
func (b *statusBadge) SetState(text string, tone statusTone) {
	c := toneColor(tone)
	b.text.Text = text
	b.dot.FillColor = c
	b.bg.FillColor = washColor(c)
	b.bg.StrokeColor = c
	b.text.Refresh()
	b.dot.Refresh()
	b.bg.Refresh()
}

func (b *statusBadge) CreateRenderer() fyne.WidgetRenderer {
	inner := container.NewHBox(
		container.NewPadded(fixedSize(b.dot, fyne.NewSize(10, 10))),
		b.text,
	)
	padded := container.New(&insetLayout{top: spaceXS, bottom: spaceXS, left: spaceMD, right: spaceMD}, inner)
	stack := container.NewStack(b.bg, padded)
	return widget.NewSimpleRenderer(stack)
}

// washColor returns a low-alpha version of c for pill backgrounds/borders —
// a "tinted glass" look rather than a solid fill, so the badge stays legible
// against cardBg while clearly carrying the status color.
func washColor(c color.Color) color.Color {
	r, g, bl, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: 40}
}

// fixedSize wraps obj in a container that reports a fixed MinSize, used to
// give the badge's dot (a canvas.Circle, which has no intrinsic size) a
// concrete footprint inside an HBox.
func fixedSize(obj fyne.CanvasObject, size fyne.Size) fyne.CanvasObject {
	return container.New(&fixedSizeLayout{size: size}, obj)
}

type fixedSizeLayout struct{ size fyne.Size }

func (l *fixedSizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(l.size)
		o.Move(fyne.NewPos((size.Width-l.size.Width)/2, (size.Height-l.size.Height)/2))
	}
}

func (l *fixedSizeLayout) MinSize([]fyne.CanvasObject) fyne.Size { return l.size }

// spacer returns a fixed-height/width blank spacer for manual vertical rhythm
// where a VBox's default spacing isn't the right amount (e.g. the gap
// between a card's header row and its body).
func vspace(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}
