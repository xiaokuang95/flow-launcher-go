package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The launcher's look: the desktop's own colours, and a softer shape than
// the reference's for everything else. The windows are rounded, the rows are
// rounded and spaced, the pictures are rounded, the text is a little smaller
// and lighter, and a group of settings is a card rather than a list of bare
// fields. The reference draws with the sharp corners of Windows 11; this
// draws the shape a desktop of its own would, which is what was asked for.
//
// Every window uses these, so that the launcher, its settings, a plugin's
// settings and a plugin's own window are the same thing to look at.

const (
	// windowRadius is how round a window is.
	windowRadius float32 = 14
	// cardRadius rounds the groups of settings.
	cardRadius float32 = 12
	// rowRadius rounds a result, and a row of settings.
	rowRadius float32 = 12
	// iconRadius rounds the picture of a result.
	iconRadius float32 = 7

	// windowPadding is the room between a window's edge and what is in it.
	windowPadding float32 = 12
	// cardPadding is the room inside a group of settings.
	cardPadding float32 = 14
	// rowPadding is the room inside a row.
	rowPadding float32 = 10

	// lookIconSize is how big a result's picture is.
	lookIconSize float32 = 30

	// settingsRowHeight is how tall one setting is, which is a row a hand
	// can hit, and settingsLabelWidth how much of it the label takes, so
	// that the controls line up down the window.
	settingsRowHeight  float32 = 38
	settingsLabelWidth float32 = 220

	// macFont is the face the launcher asks for: Segoe UI Variable is
	// Windows' own, and is a system font on Windows 11.
	macFont = "Segoe UI Variable Text"
)

// macLook paints the launcher's shape onto the theme the windows draw with:
// rounder, roomier, and lighter in the text.
func macLook(theme *ui.Theme) {
	theme.Radius = rowRadius
	theme.Spacing = 5
	theme.Font = macFont
	theme.FontSize = 14
}

// selectionTint is the colour of the row the user is on: the accent mixed
// into the background rather than painted over it, which is how a desktop of
// its own says where the user is. A list of mygo's paints the row it is on
// with the accent itself, so a row that is on paints this over it, and takes
// its words back to the theme's own colour.
func selectionTint(t *ui.Theme) ui.Color {
	return blend(t.Background, t.Accent, 0.22)
}

// blend mixes b into a by amount, and keeps the colour opaque, which is what
// a tint that has to hide what is under it needs.
func blend(a, b ui.Color, amount float32) ui.Color {
	mix := func(x, y uint8) uint8 {
		return uint8(float32(x)*(1-amount) + float32(y)*amount + 0.5)
	}
	return ui.RGB(mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B))
}

// applyMacLook is the theme a window draws with, painted from the desktop's
// own and then shaped.
func applyMacLook(c *ui.Context) {
	theme := *c.Theme()
	macLook(&theme)
	c.SetTheme(&theme)
}

// windowOptions are what the launcher's own windows are made with: no
// frame, round corners and the desktop's material behind them.
func windowOptions(title string, width, height int, stateKey string, view func(*ui.Context)) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:       title,
		Width:       width,
		Height:      height,
		Transparent: true,
		Vibrancy:    mygo.VibrancyWindow,
		Content:     ui.View(view),
		StateKey:    stateKey,
	}
}

// paneOptions are what a settings window is made with: the frame the user
// moves and closes it with, which is what a window that is not the launcher
// itself keeps.
func paneOptions(title string, width, height int, stateKey string, view func(*ui.Context)) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:    title,
		Width:    width,
		Height:   height,
		Content:  ui.View(view),
		StateKey: stateKey,
	}
}

// windowView is the root of a window of the launcher's own: the rounded card
// the whole of it is drawn on, with the desktop's material showing through
// where the window is transparent.
func windowView(c *ui.Context) ui.Element {
	t := c.Theme()

	root := ui.Column(c).Fill().Radius(windowRadius).Clip()
	if !c.Vibrancy() {
		root.Background(t.Background)
	} else {
		// The material is behind the window; a little of the background
		// keeps the text readable over whatever is under it.
		root.Background(t.Background.Alpha(0.72))
	}
	return root
}

// paneView is the root of a window that keeps the desktop's frame: the
// settings panes are not rounded, since the frame around them is not.
func paneView(c *ui.Context) ui.Element {
	return ui.Column(c).Fill().Background(c.Theme().Background)
}

// card is a group of settings: a rounded face, a hairline edge, and the room
// inside it.
func card(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).
		Radius(cardRadius).
		Padding(cardPadding).
		Gap(0).
		Background(t.Surface).
		Border(1, t.Border.Alpha(0.45))
}

// resultRow paints one result: rounded, spaced, and tinted when it is the one
// the user is on.
func resultRow(c *ui.Context, selected bool) ui.Element {
	t := c.Theme()

	row := ui.Row(c).
		Height(resultHeight).
		Padding(0, rowPadding).
		Gap(12).
		Radius(rowRadius).
		AlignItems(ui.Center)

	if selected {
		row.Background(selectionTint(t)).TextColor(t.Text)
	}
	return row
}

// iconFrame is the rounded frame a result's picture is drawn in, which is
// what keeps a square picture from looking like a sticker.
func iconFrame(c *ui.Context, bitmap *ui.Bitmap) {
	if bitmap == nil {
		return
	}
	ui.Image(c, bitmap).Size(lookIconSize, lookIconSize).Radius(iconRadius)
}

// sectionTitle is what a group of settings is called.
func sectionTitle(c *ui.Context, text string) {
	ui.Text(c, text).
		FontSize(15).
		Bold().
		TextColor(c.Theme().TextMuted).
		Padding(4, 0, 0, 2)
}

// settingsRow is one setting: what it is called on the left, and what changes
// it at the far edge of the row, which is where a hand looks for it.
func settingsRow(c *ui.Context, label string, build func()) {
	ui.Row(c).Height(settingsRowHeight).Padding(14, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		// A long name wraps rather than losing its end, which is where the
		// word that tells it from another one usually is.
		ui.Text(c, label).Width(settingsLabelWidth)
		ui.Box(c).Grow(1)
		build()
	})
}
