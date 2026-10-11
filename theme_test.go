package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// TestAThemeChangesWhatIsDrawn draws the launcher with the desktop's own
// theme and with one of the reference's, and compares the pictures: a theme
// that draws the same picture is a theme that does nothing.
func TestAThemeChangesWhatIsDrawn(t *testing.T) {
	a := listApp(t)
	withResults(a, 5)

	a.settings.Theme = core.DefaultThemeName
	a.list = ui.ListState{}
	before := ui.NewTester(a.view, 580, 200).Image()

	a.settings.Theme = "Dracula"
	a.list = ui.ListState{}
	after := ui.NewTester(a.view, 580, 200).Image()

	changed := differingPixels(before, after)
	if changed == 0 {
		t.Fatalf("the launcher was drawn the same with Dracula as with the desktop's own theme")
	}
	t.Logf("Dracula changed %d of %d pixels", changed, 580*200)
}

// TestChoosingAThemeChangesTheSettingsWindow is what the user does: pick a
// theme on the theme page. The window they are looking at has to wear it,
// or they cannot tell that anything happened: the theme's own background
// is what says so, rather than the words of the list they chose from.
func TestChoosingAThemeChangesTheSettingsWindow(t *testing.T) {
	a := listApp(t)
	a.settings = core.DefaultSettings()

	window := &settingsWindow{app: a}
	window.draft = *a.settings
	window.page = pageOf(t, "theme")
	window.pageList = ui.ListState{}
	before := ui.NewTester(window.view, 900, 620).Image()

	// What the page does when a theme is chosen from the list.
	window.draft.Theme = "Dracula"
	window.apply()

	window.pageList = ui.ListState{}
	after := ui.NewTester(window.view, 900, 620).Image()

	if a.settings.Theme != "Dracula" {
		t.Errorf("the launcher's theme is %q, want what was chosen", a.settings.Theme)
	}
	// The window wears the theme, so most of it is drawn differently: a
	// change of a few hundred pixels is the list saying which one is
	// chosen, not the window wearing it.
	changed := differingPixels(before, after)
	if changed < len(pixelsOf(before))/2 {
		t.Errorf("choosing a theme changed %d of %d pixels, want the window to wear it",
			changed, len(pixelsOf(before)))
	}
}

// pixelsOf is every pixel of a picture, so that a count of them can be
// taken.
func pixelsOf(picture *image.RGBA) []color.RGBA {
	bounds := picture.Bounds()
	pixels := make([]color.RGBA, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixels = append(pixels, picture.RGBAAt(x, y))
		}
	}
	return pixels
}

// TestAThemeTheLauncherDoesNotKnowIsLeftAlone keeps a theme that is not
// there — an older settings file, a theme that was removed — from painting
// the launcher with nothing.
func TestAThemeTheLauncherDoesNotKnowIsLeftAlone(t *testing.T) {
	a := listApp(t)
	withResults(a, 3)

	a.settings.Theme = core.DefaultThemeName
	a.list = ui.ListState{}
	before := ui.NewTester(a.view, 580, 200).Image()

	a.settings.Theme = "a theme that was never there"
	a.list = ui.ListState{}
	after := ui.NewTester(a.view, 580, 200).Image()

	if changed := differingPixels(before, after); changed != 0 {
		t.Errorf("a theme that is not there changed %d pixels", changed)
	}
}

// differingPixels counts the pixels two pictures disagree about.
func differingPixels(before, after *image.RGBA) int {
	bounds := before.Bounds()
	changed := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if before.RGBAAt(x, y) != after.RGBAAt(x, y) {
				changed++
			}
		}
	}
	return changed
}
