package main

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// TestThemesAreDrawnWithTheirOwnBackground draws the launcher wearing a few
// themes and compares the window with what the theme says: the background
// the theme names is what is drawn, and the words on it are a different
// colour. A theme that names white text and no background was drawn white
// on white, which is what this catches.
func TestThemesAreDrawnWithTheirOwnBackground(t *testing.T) {
	a := listApp(t)
	withResults(a, 5)
	// No searching: the window is what is looked at, not the plugins.
	a.engine = nil
	a.searched = a.query

	for _, scheme := range []string{"Light", "Dark"} {
		for _, name := range []string{"BlurWhite", "Circle System", "Dracula", "Darker", "SlimLight", "Nord Darker"} {
			reference, ok := core.FindTheme(name)
			if !ok {
				t.Fatalf("no theme called %q", name)
			}
			a.settings.Theme = reference.FileName
			a.settings.ColorScheme = scheme

			// What the theme says the window is drawn with.
			wanted := *ui.LightTheme()
			if a.darkTheme(scheme == "Dark") {
				wanted = *ui.DarkTheme()
			}
			core.PaintTheme(&wanted, reference, a.darkTheme(scheme == "Dark"))

			a.list = ui.ListState{}
			picture := ui.NewTester(a.view, 580, 200).Image()

			// The window's own background, away from the rows. A theme's
			// background may be translucent — it is meant to be seen
			// through a blurred window — so what is compared is how light
			// it is rather than the colour itself.
			drawn := picture.RGBAAt(575, 195)
			if difference := luminanceOf(drawn) - luminanceOfColor(wanted.Background); difference > 0.2 || difference < -0.2 {
				t.Errorf("%s (%s): the window is drawn on %v, want the theme's own %v",
					reference.Name, scheme, drawn, wanted.Background)
			}

			// And the words on it are drawn in the theme's text colour,
			// which has to be a different one.
			letters := 0
			for y := 0; y < 200; y++ {
				for x := 0; x < 580; x++ {
					pixel := picture.RGBAAt(x, y)
					if pixel == drawn {
						continue
					}
					if sameColour(pixel, wanted.Text) {
						letters++
					}
				}
			}
			if letters < 50 {
				t.Errorf("%s (%s): %d pixels are drawn in the theme's text colour %v on %v",
					reference.Name, scheme, letters, wanted.Text, drawn)
			}
		}
	}
}

// sameColour reports whether two colours look the same.
func sameColour(drawn color.RGBA, wanted ui.Color) bool {
	near := func(a, b uint8) bool {
		difference := int(a) - int(b)
		return difference > -8 && difference < 8
	}
	return near(drawn.R, wanted.R) && near(drawn.G, wanted.G) && near(drawn.B, wanted.B)
}

// luminanceOf is how light a drawn pixel is, and luminanceOfColor the same
// for a colour the launcher paints with.
func luminanceOf(c color.RGBA) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

func luminanceOfColor(c ui.Color) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}
