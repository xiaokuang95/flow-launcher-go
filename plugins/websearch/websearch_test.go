package websearch

import (
	"context"
	"testing"

	"launch/core"
)

// fakeSuggester stands in for the site, so that a test offers suggestions
// of its own rather than asking the web.
type fakeSuggester struct {
	suggestions []string
	queries     []string
	err         error
}

func (f *fakeSuggester) Suggestions(ctx context.Context, query string) ([]string, error) {
	f.queries = append(f.queries, query)
	return f.suggestions, f.err
}

func newTestPlugin(t *testing.T) (*Plugin, *fakeSuggester, *[]string, *[]*bool) {
	t.Helper()

	suggester := &fakeSuggester{}
	var opened []string
	var private []*bool

	api := core.NewPublicAPI(nil, t.TempDir(), "test")
	api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	api.Hooks.OpenURL = func(rawURL string, inPrivate *bool) {
		opened = append(opened, rawURL)
		private = append(private, inPrivate)
	}

	plugin := New()
	plugin.api = api
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	plugin.suggester = suggester
	return plugin, suggester, &opened, &private
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

// TestTheWildcardSearchIsOfferedByNameWhenNothingIsTyped covers the query
// with nothing in it: only the search that answers every query is offered,
// and it is offered by its own name rather than by what was typed.
func TestTheWildcardSearchIsOfferedByNameWhenNothingIsTyped(t *testing.T) {
	plugin, _, _, _ := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results %v, want only the search that answers every query", len(results), titlesOf(results))
	}
	if results[0].Title != plugin.words.T("flowlauncher_plugin_websearch_search")+" Google" {
		t.Errorf("the result is %q, want the search offered by name", results[0].Title)
	}
	if results[0].SubTitle != "" {
		t.Errorf("the result has a subtitle %q, want it offered by name alone", results[0].SubTitle)
	}
}

func TestTheSearchesThatAnswerEveryQueryAreOffered(t *testing.T) {
	plugin, _, _, _ := newTestPlugin(t)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "golang", TrimmedQuery: "golang"})
	titles := titlesOf(results)

	if !contains(titles, "golang") {
		t.Errorf("the results are %v, want what was typed", titles)
	}
	// A search with its own keyword is not offered for a query that did
	// not use it.
	if contains(titles, "youtube") {
		t.Errorf("the results are %v, want only the searches that answer every query", titles)
	}
}

func TestASearchWithItsOwnKeywordIsOffered(t *testing.T) {
	plugin, _, opened, _ := newTestPlugin(t)

	results, _ := plugin.Query(context.Background(), core.Query{
		Search: "golang", ActionKeyword: "wiki",
	})
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].SubTitle == "" {
		t.Error("the search has no subtitle")
	}
	// A search the user asked for comes above the one that answers every
	// query.
	if results[0].Score <= scoreStandard {
		t.Errorf("the search scores %d, want it above the one that answers every query", results[0].Score)
	}

	results[0].Action(core.ActionContext{})
	if len(*opened) != 1 || (*opened)[0] != "http://en.wikipedia.org/wiki/golang" {
		t.Errorf("the browser was asked to open %v, want the wiki search", *opened)
	}
}

func TestTheWordsAreEscapedIntoTheSearch(t *testing.T) {
	plugin, _, opened, _ := newTestPlugin(t)

	results, _ := plugin.Query(context.Background(), core.Query{
		Search: "go & rust", ActionKeyword: "wiki",
	})
	results[0].Action(core.ActionContext{})

	if len(*opened) != 1 {
		t.Fatalf("the browser was asked to open %d addresses", len(*opened))
	}
	if (*opened)[0] != "http://en.wikipedia.org/wiki/go+%26+rust" {
		t.Errorf("the address is %q, want the words escaped", (*opened)[0])
	}
}

func TestASearchCanOpenPrivately(t *testing.T) {
	plugin, _, _, private := newTestPlugin(t)
	for i := range plugin.settings.Sources {
		if plugin.settings.Sources[i].ActionKeyword == "wiki" {
			plugin.settings.Sources[i].IsPrivateMode = true
		}
	}

	results, _ := plugin.Query(context.Background(), core.Query{Search: "golang", ActionKeyword: "wiki"})
	results[0].Action(core.ActionContext{})

	if len(*private) != 1 || (*private)[0] == nil || !*(*private)[0] {
		t.Errorf("the search was opened with %v, want a private window", *private)
	}
}

func TestSuggestionsAreOffered(t *testing.T) {
	plugin, suggester, _, _ := newTestPlugin(t)
	suggester.suggestions = []string{"golang tutorial", "golang playground"}
	plugin.settings.EnableSuggestion = true

	results, _ := plugin.Query(context.Background(), core.Query{Search: "golang", TrimmedQuery: "golang"})
	titles := titlesOf(results)

	if !contains(titles, "golang tutorial") || !contains(titles, "golang playground") {
		t.Errorf("the results are %v, want the suggestions", titles)
	}
	if len(suggester.queries) == 0 {
		t.Error("the site was not asked what it suggests")
	}
}

func TestSuggestionsCanBeTurnedOff(t *testing.T) {
	plugin, suggester, _, _ := newTestPlugin(t)
	suggester.suggestions = []string{"golang tutorial"}
	plugin.settings.EnableSuggestion = false

	results, _ := plugin.Query(context.Background(), core.Query{Search: "golang", TrimmedQuery: "golang"})
	if contains(titlesOf(results), "golang tutorial") {
		t.Error("a suggestion was offered although suggestions are off")
	}
	if len(suggester.queries) != 0 {
		t.Error("the site was asked although suggestions are off")
	}
}

func TestDisabledSearchesAreLeftOut(t *testing.T) {
	plugin, _, _, _ := newTestPlugin(t)
	for i := range plugin.settings.Sources {
		if plugin.settings.Sources[i].ActionKeyword == "wiki" {
			plugin.settings.Sources[i].Enabled = false
		}
	}

	results, _ := plugin.Query(context.Background(), core.Query{Search: "golang", ActionKeyword: "wiki"})
	for _, result := range results {
		if result.SubTitle != "" && result.Title == "golang" {
			t.Errorf("a disabled search was offered: %v", titlesOf(results))
		}
	}
}

func TestAPathIsNotSearchedFor(t *testing.T) {
	plugin, _, _, _ := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{Search: `C:\Users\crazy`})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("got %v, want nothing for a path", titlesOf(results))
	}
}

func TestContextMenuCopiesWhereTheSearchGoes(t *testing.T) {
	plugin, _, _, _ := newTestPlugin(t)
	copied := ""
	plugin.api.Hooks.CopyToClipboard = func(text string, directCopy, notify bool) { copied = text }

	results, _ := plugin.Query(context.Background(), core.Query{Search: "golang", ActionKeyword: "wiki"})
	items := plugin.LoadContextMenus(results[0])
	if len(items) != 1 {
		t.Fatalf("the menu has %d items, want one", len(items))
	}
	items[0].Action(core.ActionContext{})

	if copied != "http://en.wikipedia.org/wiki/golang" {
		t.Errorf("the clipboard was given %q", copied)
	}
}

func TestReadingWhatASiteSuggests(t *testing.T) {
	// Google answers with a list whose second entry is the suggestions.
	google := `["golang", ["golang tutorial", "golang playground"]]`
	if got := suggestionsFrom("Google", []byte(google)); len(got) != 2 || got[0] != "golang tutorial" {
		t.Errorf("Google's suggestions are %v", got)
	}

	// Bing answers with objects.
	bing := `{"AS":{"Results":[{"Suggests":[{"Txt":"golang"},{"Txt":"golang tutorial"}]}]}}`
	if got := suggestionsFrom("Bing", []byte(bing)); len(got) != 2 || got[1] != "golang tutorial" {
		t.Errorf("Bing's suggestions are %v", got)
	}

	// Baidu answers with a named field.
	baidu := `{"q":"golang","s":["golang tutorial"]}`
	if got := suggestionsFrom("Baidu", []byte(baidu)); len(got) != 1 || got[0] != "golang tutorial" {
		t.Errorf("Baidu's suggestions are %v", got)
	}

	// DuckDuckGo answers with objects of its own.
	duck := `[{"phrase":"golang tutorial"}]`
	if got := suggestionsFrom("DuckDuckGo", []byte(duck)); len(got) != 1 || got[0] != "golang tutorial" {
		t.Errorf("DuckDuckGo's suggestions are %v", got)
	}
}

func TestSuggestionAddresses(t *testing.T) {
	address, err := suggestionURL("Google", "go & rust")
	if err != nil {
		t.Fatal(err)
	}
	if address != "https://www.google.com/complete/search?output=chrome&q=go+%26+rust" {
		t.Errorf("the address is %q", address)
	}

	if _, err := suggestionURL("Netflix", "anything"); err == nil {
		t.Error("a site that offers no suggestions was given an address")
	}
}

func TestTrimSuggestions(t *testing.T) {
	got := trimSuggestions([]string{"a", " a ", "", "b", "a", "c"}, 3)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("trimSuggestions = %v, want the first three without repeats", got)
	}
}

func TestEverySearchHasAKeyword(t *testing.T) {
	keywords := keywordsOf(defaultSources)
	if len(keywords) == 0 {
		t.Fatal("no keywords")
	}
	if !contains(keywords, "*") || !contains(keywords, "wiki") {
		t.Errorf("the keywords are %v, want the wildcard and the searches' own", keywords)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
