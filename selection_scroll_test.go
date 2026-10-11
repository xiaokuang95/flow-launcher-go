package main

import (
	"os"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// listApp is a launcher with results of its own, which the tests below
// walk through. Making one loads the plugins, which takes seconds, so they
// share one: what they look at is the list, not the plugins.
func listApp(t *testing.T) *app {
	t.Helper()

	sharedListAppOnce.Do(func() {
		a, err := newApp(sharedListAppDir)
		if err != nil {
			sharedListAppErr = err
			return
		}
		a.settings.MaxResultsToShow = 5
		sharedListApp = a
	})
	if sharedListAppErr != nil {
		t.Fatal(sharedListAppErr)
	}
	if sharedListApp == nil {
		t.Fatal("the launcher could not be made")
	}

	// Each test starts from a list of its own.
	sharedListApp.contextMenu = nil
	sharedListApp.contextIndex = 0
	sharedListApp.preview = false
	return sharedListApp
}

var (
	sharedListAppOnce sync.Once
	sharedListApp     *app
	sharedListAppErr  error
)

// sharedListAppDir is where the launcher the tests share keeps its data. It
// is made once and left behind by the test runner, like any temporary file.
var sharedListAppDir = func() string {
	directory, err := os.MkdirTemp("", "flow-launcher-list-test")
	if err != nil {
		panic(err)
	}
	return directory
}()

// withResults gives the launcher a list of its own to walk through.
func withResults(a *app, count int) {
	a.results = make([]core.Result, 0, count)
	for i := 0; i < count; i++ {
		a.results = append(a.results, core.NewResult("result "+itoa(i), "", "", 100-i))
	}
	a.selection = 0
	a.query, a.searched = "result", "result"
}

// TestTheListFollowsTheChoiceOffTheScreen is what a user does: type a query
// that finds more results than fit, then press Down until the choice is
// past the bottom of the window. The list has to show it.
func TestTheListFollowsTheChoiceOffTheScreen(t *testing.T) {
	a := listApp(t)
	withResults(a, 30)

	tester := ui.NewTester(a.view, 580, 200)
	defer tester.Frame()

	// The first frame shows the top of the list.
	first, last := a.list.Visible()
	if first != 0 {
		t.Fatalf("the list starts at row %d, want the top", first)
	}
	if last <= 1 {
		t.Fatalf("the list shows rows %d..%d, want a window full of them", first, last)
	}

	// Down past the bottom: the list follows.
	for i := 0; i < last+3; i++ {
		a.moveSelection(1)
		tester.Frame()
	}

	if a.selection <= last {
		t.Fatalf("the choice is at %d, want it past the first window of rows (%d)", a.selection, last)
	}
	shownFirst, shownLast := a.list.Visible()
	if a.selection < shownFirst || a.selection > shownLast {
		t.Errorf("the choice is at %d and the list shows rows %d..%d, want the choice on the screen",
			a.selection, shownFirst, shownLast)
	}

	// And back up past the top.
	for i := 0; i < 40; i++ {
		a.moveSelection(-1)
		tester.Frame()
	}
	shownFirst, shownLast = a.list.Visible()
	if a.selection < shownFirst || a.selection > shownLast {
		t.Errorf("going back up, the choice is at %d and the list shows rows %d..%d",
			a.selection, shownFirst, shownLast)
	}
}

// TestTheListFollowsTheChoiceAPageAtATime is the same for the keys that
// move by a page and to the ends of the list.
func TestTheListFollowsTheChoiceAPageAtATime(t *testing.T) {
	a := listApp(t)
	withResults(a, 40)

	tester := ui.NewTester(a.view, 580, 200)
	defer tester.Frame()

	// PageDown twice, then End.
	for i := 0; i < 2; i++ {
		a.moveSelection(a.pageSize())
		tester.Frame()
	}
	if first, last := a.list.Visible(); a.selection < first || a.selection > last {
		t.Errorf("a page down put the choice at %d and the list shows %d..%d", a.selection, first, last)
	}

	a.selectIndex(len(a.results) - 1)
	tester.Frame()
	if first, last := a.list.Visible(); a.selection < first || a.selection > last {
		t.Errorf("the last result is at %d and the list shows %d..%d", a.selection, first, last)
	}

	a.selectIndex(0)
	tester.Frame()
	if first, last := a.list.Visible(); a.selection < first || a.selection > last {
		t.Errorf("the first result is at %d and the list shows %d..%d", a.selection, first, last)
	}
}

// TestTheMenuFollowsTheChoiceToo is the same for the menu of a result,
// which has a list of its own.
func TestTheMenuFollowsTheChoiceToo(t *testing.T) {
	a := listApp(t)
	a.settings.MaxResultsToShow = 3

	a.contextMenu = make([]core.Result, 0, 20)
	for i := 0; i < 20; i++ {
		a.contextMenu = append(a.contextMenu, core.NewResult("item "+itoa(i), "", "", 0))
	}
	withResults(a, 1)

	tester := ui.NewTester(a.view, 580, 200)
	defer tester.Frame()

	// The menu is drawn in place of the results while it is open.
	for i := 0; i < 12; i++ {
		a.moveMenuSelection(1)
		tester.Frame()
	}

	first, last := a.menuList.Visible()
	if a.contextIndex < first || a.contextIndex > last {
		t.Errorf("the menu choice is at %d and the menu shows %d..%d", a.contextIndex, first, last)
	}
}
