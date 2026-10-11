package main

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// menuPlugin is a plugin with a context menu of its own, so that the menu
// the launcher shows can be checked without a window.
type menuPlugin struct {
	core.BasePlugin
	titles []string
	chosen []string
}

func (p *menuPlugin) Metadata() core.PluginMetadata {
	return core.PluginMetadata{ID: "menu", Name: "Menu", ActionKeywords: []string{"*"}}
}

func (p *menuPlugin) Init(*core.PluginInitContext) error { return nil }

func (p *menuPlugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	result := core.NewResult("A result", "its subtitle", "", 100)
	return []core.Result{result}, nil
}

func (p *menuPlugin) LoadContextMenus(selected core.Result) []core.Result {
	items := make([]core.Result, 0, len(p.titles))
	for _, title := range p.titles {
		title := title
		item := core.NewResult(title, "", "", 0)
		item.Action = func(core.ActionContext) bool {
			p.chosen = append(p.chosen, title)
			return true
		}
		items = append(items, item)
	}
	return items
}

// newMenuApp makes the launcher over one plugin, with its records in a
// temporary directory.
func newMenuApp(t *testing.T) (*app, *menuPlugin) {
	t.Helper()

	plugin := &menuPlugin{titles: []string{"Do one thing", "Do another"}}
	manager := core.NewPluginManager()
	manager.Load(core.PluginPair{Plugin: plugin, Metadata: plugin.Metadata()})

	dir := t.TempDir()
	a := &app{
		dataDir:  dir,
		settings: core.DefaultSettings(),
		plugins:  &core.PluginsSettings{Plugins: map[string]core.PluginSettings{}},
		manager:  manager,
	}
	a.api = core.NewPublicAPI(manager, dir, "test")
	a.api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	a.manager.API = a.api
	a.api.Hooks = a.hooks()
	manager.Initialize(context.Background())

	a.engine = core.NewEngine(manager, a.settings)
	a.history = core.LoadHistory(dir + "/History.json")
	a.selected = core.LoadUserSelectedRecord(dir + "/Selected.json")
	a.topMost = core.LoadTopMostRecord(dir + "/TopMost.json")
	a.engine.History, a.engine.Selected, a.engine.TopMost = a.history, a.selected, a.topMost
	a.engine.SearchDelayTime = 0
	a.icons = newIconCache()

	a.results = a.engine.Search(context.Background(), "result")
	if len(a.results) == 0 {
		t.Fatal("the plugin answered nothing")
	}
	return a, plugin
}

func TestTheMenuOfAResultListsItsItems(t *testing.T) {
	a, _ := newMenuApp(t)

	a.openContextMenu()

	// The menu holds the launcher's own items and the plugin's.
	if len(a.contextMenu) < 4 {
		t.Fatalf("the menu holds %d items, want the launcher's own and the plugin's", len(a.contextMenu))
	}

	titles := make([]string, 0, len(a.contextMenu))
	for _, item := range a.contextMenu {
		titles = append(titles, item.Title)
	}
	for _, want := range []string{"Do one thing", "Do another"} {
		if !containsTitle(titles, want) {
			t.Errorf("the menu %v does not hold %q", titles, want)
		}
	}
	// The launcher's own items come first, so that pinning is the first
	// thing offered.
	if a.contextMenu[0].Title != core.T("", "setAsTopMostInThisQuery") {
		t.Errorf("the first item is %q, want pinning", a.contextMenu[0].Title)
	}
}

func TestTheMenuTakesTheKeys(t *testing.T) {
	a, _ := newMenuApp(t)
	a.openContextMenu()

	// Down and Up move through the menu rather than the results.
	a.key(0, ui.KeyDown)
	if a.contextIndex != 1 {
		t.Errorf("the choice is %d, want the second item", a.contextIndex)
	}
	a.key(0, ui.KeyUp)
	if a.contextIndex != 0 {
		t.Errorf("the choice is %d, want the first item", a.contextIndex)
	}
	// It wraps around.
	a.key(0, ui.KeyUp)
	if a.contextIndex != len(a.contextMenu)-1 {
		t.Errorf("the choice is %d, want the last item", a.contextIndex)
	}

	// Escape puts the results back.
	a.key(0, ui.KeyEscape)
	if len(a.contextMenu) != 0 {
		t.Error("the menu is still open after Escape")
	}
}

func TestChoosingAnItemRunsIt(t *testing.T) {
	a, plugin := newMenuApp(t)
	a.openContextMenu()

	// Find the plugin's own item and choose it.
	index := -1
	for i, item := range a.contextMenu {
		if item.Title == "Do another" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("the plugin's item is not in the menu")
	}

	a.contextIndex = index
	a.runContextItem(index)

	if len(plugin.chosen) != 1 || plugin.chosen[0] != "Do another" {
		t.Errorf("the plugin was asked to run %v, want the item chosen", plugin.chosen)
	}
	if len(a.contextMenu) != 0 {
		t.Error("the menu is still open after an item was chosen")
	}
}

func TestPinningFromTheMenu(t *testing.T) {
	a, _ := newMenuApp(t)
	a.openContextMenu()

	// The first item pins the result.
	a.runContextItem(0)

	if !a.topMost.IsTopMost(a.results[0]) {
		t.Error("the result was not pinned")
	}

	// And the menu then offers to take the pin away.
	a.openContextMenu()
	if a.contextMenu[0].Title != core.T("", "cancelTopMostInThisQuery") {
		t.Errorf("the first item is %q, want unpinning", a.contextMenu[0].Title)
	}
}

func TestTypingCarriesOnWhileTheMenuIsOpen(t *testing.T) {
	a, _ := newMenuApp(t)
	a.openContextMenu()

	// A letter is left to the query box, so that what is underneath can
	// still be narrowed.
	if a.key(0, ui.KeyA) {
		t.Error("the launcher took a letter while the menu was open")
	}
}

func containsTitle(titles []string, want string) bool {
	for _, title := range titles {
		if title == want {
			return true
		}
	}
	return false
}
