package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// TestTheThemePageListsEveryTheme is what the user needs from the theme
// page: every theme of the reference's list is on it, and the list scrolls,
// so that the ones past the bottom of the window can be reached. A
// drop-down drew them beside the window, where the wheel could not reach
// them at all.
func TestTheThemePageListsEveryTheme(t *testing.T) {
	a := listApp(t)
	a.settings = core.DefaultSettings()

	window := &settingsWindow{app: a}
	window.draft = *a.settings
	window.page = pageOf(t, "theme")

	// The list keeps its own place, which is what the test reads: where a
	// label is drawn is not what moves, the list's own offset is.
	var state ui.ScrollState
	choiceListDebug = func(e ui.Element) { e.TrackScroll(&state) }
	defer func() { choiceListDebug = nil }()

	tester := ui.NewTester(window.view, 900, 620)

	themes := core.AvailableThemes()
	for _, theme := range themes {
		if !tester.HasText(theme.Name) {
			t.Fatalf("the theme %q is not on the page", theme.Name)
		}
	}

	if state.MaxY <= 0 {
		t.Fatalf("the list holds all %d themes without scrolling, which cannot be right", len(themes))
	}

	// The wheel over the rows scrolls the list, which is how the themes
	// past the bottom of the window are reached.
	first, ok := tester.Find(themes[0].Name)
	if !ok {
		t.Fatalf("the first theme is not on the page")
	}
	tester.Move(first.X+5, first.Y+first.H/2)
	tester.Scroll(first.X+5, first.Y+first.H/2, 0, 150)
	tester.Frame()

	if state.Y < 100 {
		t.Errorf("the wheel moved the list to %v, want it to scroll by what was asked (150)", state.Y)
	}

	// And clicking a theme chooses it, with the list scrolled back to its
	// start so that the row the click is aimed at is the row that is there.
	tester.Scroll(first.X+5, first.Y+first.H/2, 0, -150)
	tester.Frame()
	second, _ := tester.Find(themes[1].Name)
	tester.ClickAt(second.X+5, second.Y+second.H/2)
	if a.settings.Theme != themes[1].FileNameWithoutExtension {
		t.Errorf("clicking %q chose %q", themes[1].Name, a.settings.Theme)
	}
}
