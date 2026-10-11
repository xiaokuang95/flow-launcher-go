package windowssettings

import (
	"context"
	"testing"

	"launch/core"
)

// fakeOpener stands in for the settings app, so that a test watches what
// was asked to be opened rather than opening the settings.
type fakeOpener struct {
	opened []string
}

func (f *fakeOpener) Open(command string) error {
	f.opened = append(f.opened, command)
	return nil
}

func newTestPlugin(t *testing.T) (*Plugin, *fakeOpener) {
	t.Helper()

	opener := &fakeOpener{}
	plugin := New()
	plugin.opener = opener
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	plugin.words = newTranslator()
	if err := plugin.Init(&core.PluginInitContext{API: plugin.api}); err != nil {
		t.Fatal(err)
	}
	plugin.opener = opener
	return plugin, opener
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestEverySettingIsListed(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(plugin.settings) {
		t.Fatalf("got %d results, want one per settings page", len(results))
	}
	if len(results) < 100 {
		t.Errorf("there are only %d settings pages, want the reference's own list", len(results))
	}
	for _, result := range results {
		if result.Title == "" || result.SubTitle == "" {
			t.Errorf("a settings page has no words: %+v", result)
		}
	}
}

func TestASettingIsFoundByName(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	// The pages are named in the launcher's language; the test looks for
	// the first one by its own name.
	name := plugin.nameOf(plugin.settings[0])
	results, err := plugin.Query(context.Background(), core.Query{Search: name})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatalf("no results for %q", name)
	}
	if results[0].Title != name {
		t.Errorf("the first result is %q, want %q", results[0].Title, name)
	}
	if len(results[0].TitleHighlightData) == 0 {
		t.Error("nothing is highlighted")
	}
}

func TestOpeningASettingAsksForItsCommand(t *testing.T) {
	plugin, opener := newTestPlugin(t)

	name := plugin.nameOf(plugin.settings[0])
	results, _ := plugin.Query(context.Background(), core.Query{Search: name})
	results[0].Action(core.ActionContext{})

	if len(opener.opened) != 1 {
		t.Fatalf("the settings app was asked to open %d pages, want one", len(opener.opened))
	}
	if opener.opened[0] != plugin.settings[0].Command {
		t.Errorf("the settings app was asked to open %q, want %q",
			opener.opened[0], plugin.settings[0].Command)
	}
}

// TestAPageFoundByItsAreaIsWorthLessThanOneFoundByName covers the
// adjustment the reference makes: a page whose part of the settings
// matched is worth less than a page whose own name matched.
func TestAPageFoundByItsAreaIsWorthLessThanOneFoundByName(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	// The same page, found once by its name and once by the part of the
	// settings it is in.
	setting := plugin.settings[0]
	name := plugin.nameOf(setting)
	area := plugin.areaOf(setting)

	byName, _ := plugin.Query(context.Background(), core.Query{Search: name})
	byArea, _ := plugin.Query(context.Background(), core.Query{Search: area})

	var nameScore, areaScore int
	for _, result := range byName {
		if result.Title == name {
			nameScore = result.Score
		}
	}
	for _, result := range byArea {
		if result.Title == name {
			areaScore = result.Score
		}
	}
	if nameScore == 0 {
		t.Fatalf("the page %q was not found by its own name", name)
	}
	if areaScore == 0 {
		t.Fatalf("the page %q was not found by the area %q", name, area)
	}
	if areaScore >= nameScore {
		t.Errorf("the page scores %d by its area and %d by its name, want the area below the name",
			areaScore, nameScore)
	}
}

func TestASettingIsFoundByAnotherOfItsNames(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	// Find a page that has another name.
	var setting Setting
	for _, candidate := range plugin.settings {
		if len(candidate.AltNames) > 0 {
			setting = candidate
			break
		}
	}
	if setting.Name == "" {
		t.Skip("no settings page has another name")
	}

	results, _ := plugin.Query(context.Background(), core.Query{Search: setting.AltNames[0]})
	found := false
	for _, result := range results {
		if result.Title == plugin.nameOf(setting) {
			found = true
		}
	}
	if !found {
		t.Errorf("the page %q was not found by its other name %q",
			plugin.nameOf(setting), setting.AltNames[0])
	}
}

func TestTheWordsAreReadFromTheReferenceResources(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	// A page the reference names, and the part of the settings it is in.
	table := plugin.words.table("en")
	if len(table) < 100 {
		t.Fatalf("the English words hold only %d entries", len(table))
	}
	for _, key := range []string{"AreaAccounts", "AppSettingsApp", "ControlPanel"} {
		if table[key] == "" {
			t.Errorf("the words do not hold %q", key)
		}
	}
}

func TestTheSettingsDataIsTheReferences(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	// Every page has a command that opens it.
	for _, setting := range plugin.settings {
		if setting.Command == "" {
			t.Errorf("the page %q has no command", setting.Name)
		}
		if setting.Name == "" || setting.Area == "" {
			t.Errorf("a page is missing its name or its area: %+v", setting)
		}
	}
}

func TestEveryTermHasToMatchAKeyword(t *testing.T) {
	keywords := [][]string{{"display", "screen"}, {"brightness"}}

	if !allTermsInKeywords([]string{"display"}, keywords) {
		t.Error("a word the page answers to did not match")
	}
	if !allTermsInKeywords([]string{"display", "brightness"}, keywords) {
		t.Error("two words the page answers to did not match")
	}
	if allTermsInKeywords([]string{"display", "printer"}, keywords) {
		t.Error("a word the page does not answer to matched")
	}
}

func TestContextMenuCopiesTheCommand(t *testing.T) {
	plugin, _ := newTestPlugin(t)
	copied := ""
	plugin.api.Hooks.CopyToClipboard = func(text string, directCopy, notify bool) { copied = text }

	name := plugin.nameOf(plugin.settings[0])
	results, _ := plugin.Query(context.Background(), core.Query{Search: name})
	items := plugin.LoadContextMenus(results[0])
	if len(items) != 1 {
		t.Fatalf("the menu has %d items, want one", len(items))
	}
	items[0].Action(core.ActionContext{})

	if copied != plugin.settings[0].Command {
		t.Errorf("the clipboard was given %q, want the command", copied)
	}
}
