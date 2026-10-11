package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// TestEveryThemeIsReadable checks the themes the launcher can wear: the
// words have to stand out against the background they are drawn on. A theme
// that names a text colour and no background — Blur White, Circle System —
// used to be painted white on white, or black on black.
func TestEveryThemeIsReadable(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, theme := range core.AvailableThemes() {
			reference, ok := core.FindTheme(theme.FileNameWithoutExtension)
			if !ok {
				continue
			}
			// The launcher starts from the theme the desktop gave it.
			painted := *ui.LightTheme()
			if dark {
				painted = *ui.DarkTheme()
			}
			core.PaintTheme(&painted, reference, dark)

			text := contrast(painted.Text, painted.Background)
			muted := contrast(painted.TextMuted, painted.Background)
			// A contrast of 3 is where large text stops being readable;
			// the reference's own themes are all above it but for Gray,
			// which is 2.7 in the reference too.
			if text < 2.5 || muted < 1.8 {
				t.Errorf("%s (dark=%v): the text is drawn at a contrast of %.2f on its background (muted %.2f): background %v, text %v",
					reference.Name, dark, text, muted, painted.Background, painted.Text)
			}
		}
	}
}

// contrast is how far apart two colours are, from 1 for the same colour to
// 21 for black on white.
func contrast(a, b ui.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(c ui.Color) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}
