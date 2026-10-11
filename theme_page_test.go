package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// TestTheThemeDropDownIsEveryTheme is what the user needs from the theme
// page: every theme of the reference's list is in the drop-down, the list
// scrolls with the wheel and with the keys, and choosing one wears it. A
// drop-down drew its list beside the window, where neither the wheel nor
// the keys could reach past the window's bottom.
func TestTheThemeDropDownIsEveryTheme(t *testing.T) {
	a := listApp(t)
	a.settings = core.DefaultSettings()

	window := &settingsWindow{app: a}
	window.draft = *a.settings
	window.page = pageOf(t, "theme")

	tester := ui.NewTester(window.view, 900, 620)
	tester.SetFocused(true)

	themes := core.AvailableThemes()
	if err := tester.Click(themes[0].Name); err != nil {
		t.Fatalf("the theme drop-down could not be opened: %v", err)
	}

	// Every theme is in the list.
	for _, theme := range themes {
		if !tester.HasText(theme.Name) {
			t.Fatalf("the theme %q is not in the drop-down", theme.Name)
		}
	}

	state := dropDownState("theme")
	if state.MaxY <= 0 {
		t.Fatalf("the list of %d themes does not scroll, which cannot be right", len(themes))
	}

	// The wheel over the list scrolls it, which is how the themes past the
	// window's bottom are reached. The chosen one is shown in the trigger
	// as well, so the wheel is pointed at a theme that is only in the list.
	row, ok := tester.Find(themes[2].Name)
	if !ok {
		t.Fatalf("the third theme is not in the drop-down")
	}
	tester.Move(row.X+5, row.Y+row.H/2)
	tester.Scroll(row.X+5, row.Y+row.H/2, 0, 120)
	tester.Frame()
	if state.Y < 100 {
		t.Errorf("the wheel scrolled the list to %v, want it to scroll by what was asked (120)", state.Y)
	}

	// The keys walk the list, and choosing one wears it: the list follows
	// the highlight, which is what the arrow keys move.
	for i := 0; i < 6; i++ {
		tester.Key(0, ui.KeyDown)
	}
	tester.Key(0, ui.KeyEnter)

	chosen := a.settings.Theme
	if chosen == "" || chosen == themes[0].FileNameWithoutExtension {
		t.Errorf("Enter chose %q, want a theme the keys walked to", chosen)
	}
	found := false
	for _, theme := range themes {
		if theme.FileNameWithoutExtension == chosen {
			found = true
		}
	}
	if !found {
		t.Errorf("Enter chose %q, which is not one of the themes", chosen)
	}
	t.Logf("six Downs and Enter chose %q", chosen)
}
