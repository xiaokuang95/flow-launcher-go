package pluginindicator

import (
	"context"
	"testing"

	"launch/core"
)

// newTestPlugin makes a plugin over a launcher holding the plugins given,
// so that the plugin points at them exactly as it does when the launcher is
// running.
func newTestPlugin(t *testing.T, pairs ...core.PluginPair) (*Plugin, *core.PublicAPI) {
	t.Helper()

	manager := core.NewPluginManager()
	for _, pair := range pairs {
		manager.Load(pair)
	}

	api := core.NewPublicAPI(manager, t.TempDir(), "test")
	api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})

	plugin := New()
	plugin.api = api
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	return plugin, api
}

func TestQueryListsEveryActionKeyword(t *testing.T) {
	plugin, _ := newTestPlugin(t,
		pair("calculator", []string{"*"}, "Calculator"),
		pair("shell", []string{">"}, "Shell"),
		pair("websearch", []string{"sc", "wiki"}, "Web Search"),
	)

	results, err := plugin.Query(context.Background(), core.Query{Search: ""})
	if err != nil {
		t.Fatal(err)
	}

	titles := titlesOf(results)
	if len(titles) != 3 {
		t.Fatalf("got %d results %v, want the three keywords", len(titles), titles)
	}
	for _, want := range []string{">", "sc", "wiki"} {
		if !contains(titles, want) {
			t.Errorf("the results %v do not name %q", titles, want)
		}
	}
	// A plugin that answers every query has no keyword to show.
	if contains(titles, "*") {
		t.Error("the wildcard keyword was listed")
	}
}

func TestQueryMatchesAKeyword(t *testing.T) {
	plugin, _ := newTestPlugin(t,
		pair("shell", []string{">"}, "Shell"),
		pair("websearch", []string{"sc", "wiki"}, "Web Search"),
	)

	results, err := plugin.Query(context.Background(), core.Query{Search: "wiki"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].Title != "wiki" {
		t.Errorf("the first result is %q, want the keyword", results[0].Title)
	}
}

func TestQueryMatchesAPluginName(t *testing.T) {
	plugin, _ := newTestPlugin(t,
		pair("shell", []string{">"}, "Shell"),
		pair("websearch", []string{"sc"}, "Web Search"),
	)

	results, err := plugin.Query(context.Background(), core.Query{Search: "shell"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 || results[0].Title != ">" {
		t.Errorf("the results are %v, want the shell keyword", titlesOf(results))
	}
}

func TestChoosingAKeywordTypesIt(t *testing.T) {
	plugin, _ := newTestPlugin(t, pair("shell", []string{">"}, "Shell"))

	results, _ := plugin.Query(context.Background(), core.Query{Search: ""})
	if results[0].AutoCompleteText != "> " {
		t.Errorf("AutoCompleteText is %q, want %q", results[0].AutoCompleteText, "> ")
	}

	// Choosing the result asks the launcher to put the keyword in the query
	// box and leaves the launcher up.
	changed := ""
	plugin.api.Hooks.ChangeQuery = func(query string, requery bool) { changed = query }
	if hide := results[0].Action(core.ActionContext{}); hide {
		t.Error("choosing a keyword hid the launcher")
	}
	if changed != "> " {
		t.Errorf("the query was changed to %q, want %q", changed, "> ")
	}
}

func TestDisabledPluginsAreLeftOut(t *testing.T) {
	disabled := pair("shell", []string{">"}, "Shell")
	disabled.Metadata.Disabled = true
	plugin, _ := newTestPlugin(t, disabled, pair("websearch", []string{"sc"}, "Web Search"))

	results, _ := plugin.Query(context.Background(), core.Query{Search: ""})
	if contains(titlesOf(results), ">") {
		t.Error("a disabled plugin's keyword was listed")
	}
}

func TestContextMenuOpensThePluginSettings(t *testing.T) {
	plugin, _ := newTestPlugin(t, pair("shell", []string{">"}, "Shell"))

	results, _ := plugin.Query(context.Background(), core.Query{Search: ""})
	items := plugin.LoadContextMenus(results[0])
	if len(items) != 1 {
		t.Fatalf("the menu has %d items, want one", len(items))
	}

	opened := ""
	plugin.api.Hooks.OpenPluginSettings = func(id string) bool {
		opened = id
		return true
	}
	items[0].Action(core.ActionContext{})
	if opened != "shell" {
		t.Errorf("the settings opened were %q, want the plugin's", opened)
	}
}

// pair is a plugin with some keywords and a name.
func pair(id string, keywords []string, display string) core.PluginPair {
	return core.PluginPair{
		Metadata: core.PluginMetadata{
			ID:             id,
			Name:           display,
			ActionKeywords: keywords,
			IcoPath:        "Images/" + id + ".png",
		},
	}
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
