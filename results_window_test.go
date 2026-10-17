package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// windowTitles are what a list of results says.
func windowTitles(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

// resultsWindowFor makes a plugin's window over a fixed list, which is what
// a plugin hands the launcher.
func resultsWindowFor(t *testing.T, a *app, items []core.Result) (*resultsWindow, *[]core.Result) {
	t.Helper()

	chosen := &[]core.Result{}
	window := &resultsWindow{app: a, window: core.ResultsWindow{
		PluginID:    "probe",
		Title:       "Clipboard history",
		Placeholder: "Search the clipboard history",
		Search: func(query string) []core.Result {
			found := make([]core.Result, 0, len(items))
			for _, item := range items {
				if query == "" || item.Title == query {
					found = append(found, item)
				}
			}
			return found
		},
		Choose: func(result core.Result) bool {
			*chosen = append(*chosen, result)
			return false
		},
	}}
	window.search("")
	return window, chosen
}

// TestAPluginsWindowShowsItsResults draws the window headless: what a plugin
// hands over is what the user sees.
func TestAPluginsWindowShowsItsResults(t *testing.T) {
	a := listApp(t)
	a.settings = core.DefaultSettings()

	window, _ := resultsWindowFor(t, a, []core.Result{
		core.NewResult("first item", "just now", "", 100),
		core.NewResult("second item", "2m ago", "", 100),
	})

	tester := ui.NewTester(window.view, 460, 420)

	for _, want := range []string{"first item", "second item"} {
		if !tester.HasText(want) {
			t.Errorf("the window does not show %q: %v", want, tester.Texts())
		}
	}
}

// TestTheKeysWalkAPluginsWindow is what the user does: the arrows move
// through the list, and Enter chooses what they are on.
func TestTheKeysWalkAPluginsWindow(t *testing.T) {
	a := listApp(t)
	a.settings = core.DefaultSettings()

	items := make([]core.Result, 0, 5)
	for i := 0; i < 5; i++ {
		items = append(items, core.NewResult("item "+itoa(i), "", "", 100-i))
	}
	window, chosen := resultsWindowFor(t, a, items)

	tester := ui.NewTester(window.view, 460, 420)
	tester.SetFocused(true)
	// The window puts the caret in its query box, which is what the keys go
	// to: without a window there is nothing to focus on its own.
	window.focusQuery()
	tester.Frame()

	// Down moves the choice, and Enter chooses it.
	for i := 0; i < 3; i++ {
		tester.Key(0, ui.KeyDown)
	}
	if window.selection != 3 {
		t.Fatalf("three Downs left the choice on %d, want the fourth result", window.selection)
	}
	tester.Key(0, ui.KeyEnter)

	if len(*chosen) != 1 || (*chosen)[0].Title != "item 3" {
		t.Fatalf("Enter chose %v, want the fourth result", windowTitles(*chosen))
	}

	// Up moves it back, and Escape closes the window without choosing.
	window.search("")
	tester.Frame()
	tester.Key(0, ui.KeyUp)
	if window.selection != 0 {
		t.Errorf("Up from the first result left the choice on %d", window.selection)
	}
}

// TestTypingNarrowsAPluginsWindow is the other half of what the user does.
func TestTypingNarrowsAPluginsWindow(t *testing.T) {
	a := listApp(t)
	a.settings = core.DefaultSettings()

	window, _ := resultsWindowFor(t, a, []core.Result{
		core.NewResult("alpha", "", "", 100),
		core.NewResult("beta", "", "", 100),
	})

	tester := ui.NewTester(window.view, 460, 420)
	tester.SetFocused(true)
	window.focusQuery()
	tester.Frame()

	// The query box is what the window is for: typing there narrows the
	// list the plugin answered with.
	window.search("beta")
	tester.Frame()

	if len(window.results) != 1 || window.results[0].Title != "beta" {
		t.Errorf("the window shows %v, want what the plugin answered for \"beta\"", windowTitles(window.results))
	}
}
