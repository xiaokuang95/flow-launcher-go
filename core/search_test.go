package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakePlugin answers with fixed results, so that the engine's own behaviour
// can be tested without a machine under it.
type fakePlugin struct {
	name     string
	results  []Result
	err      error
	delay    time.Duration
	homeOnly bool
	queries  []Query
}

func (f *fakePlugin) Metadata() PluginMetadata {
	return PluginMetadata{ID: f.name, Name: f.name, ActionKeywords: []string{"*"}}
}

func (f *fakePlugin) Init(*PluginInitContext) error { return nil }

func (f *fakePlugin) Query(ctx context.Context, query Query) ([]Result, error) {
	f.queries = append(f.queries, query)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func (f *fakePlugin) HomeQuery(ctx context.Context) ([]Result, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.results, nil
}

// newTestEngine makes an engine over the plugins given, with its settings
// and records in a temporary directory.
func newTestEngine(t *testing.T, pairs ...PluginPair) (*Engine, *PluginManager) {
	t.Helper()

	dir := t.TempDir()
	manager := NewPluginManager()
	manager.API = NewPublicAPI(manager, dir, "test")
	manager.API.SetMatcher(StringMatcher{UserSettingSearchPrecision: SearchPrecisionRegular})

	for _, pair := range pairs {
		manager.Load(pair)
	}
	manager.Initialize(context.Background())

	settings := DefaultSettings()
	settings.MaxResultsToShow = 10

	engine := NewEngine(manager, settings)
	engine.History = LoadHistory(filepath.Join(dir, "History.json"))
	engine.Selected = LoadUserSelectedRecord(filepath.Join(dir, "UserSelectedRecord.json"))
	engine.TopMost = LoadTopMostRecord(filepath.Join(dir, "MultipleTopMostRecord.json"))
	engine.SearchDelayTime = 0
	return engine, manager
}

// scoredPlugin is a plugin with one result at a score of its own.
func scoredPlugin(id, keyword string, score int, title string) PluginPair {
	return PluginPair{
		Plugin: &fakePlugin{name: id, results: []Result{NewResult(title, "", "", score)}},
		Metadata: PluginMetadata{
			ID:              id,
			Name:            id,
			ActionKeywords:  []string{keyword},
			PluginDirectory: "C:/plugins/" + id,
		},
	}
}

// resultsPlugin is a plugin that answers with the titles given, scoring the
// first highest.
func resultsPlugin(id, keyword string, titles ...string) PluginPair {
	results := make([]Result, 0, len(titles))
	for i, title := range titles {
		results = append(results, NewResult(title, "subtitle of "+title, "", 100-i*10))
	}
	return PluginPair{
		Plugin: &fakePlugin{name: id, results: results},
		Metadata: PluginMetadata{
			ID:              id,
			Name:            id,
			ActionKeywords:  []string{keyword},
			PluginDirectory: "C:/plugins/" + id,
		},
	}
}

func titlesOf(results []Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestSearchSortsByScore(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("one", "*", "Alpha", "Beta", "Gamma"))

	results := engine.Search(context.Background(), "a")
	want := []string{"Alpha", "Beta", "Gamma"}
	if got := titlesOf(results); !sameStrings(got, want) {
		t.Errorf("results are %v, want %v", got, want)
	}
}

func TestSearchStampsThePluginOntoItsResults(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("program", "*", "Notepad"))

	results := engine.Search(context.Background(), "note")
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].PluginID != "program" {
		t.Errorf("PluginID = %q, want program", results[0].PluginID)
	}
	if results[0].PluginDirectory != "C:/plugins/program" {
		t.Errorf("PluginDirectory = %q", results[0].PluginDirectory)
	}
	if results[0].OriginQuery == nil || results[0].OriginQuery.Search != "note" {
		t.Errorf("OriginQuery = %+v", results[0].OriginQuery)
	}
}

func TestSearchOnlyAsksThePluginsOfTheActionKeyword(t *testing.T) {
	engine, _ := newTestEngine(t,
		resultsPlugin("global", "*", "Everywhere"),
		resultsPlugin("files", "f", "File one"),
	)

	results := engine.Search(context.Background(), "f something")
	if got := titlesOf(results); !sameStrings(got, []string{"File one"}) {
		t.Errorf("results are %v, want only the files plugin's", got)
	}

	results = engine.Search(context.Background(), "something")
	if got := titlesOf(results); !sameStrings(got, []string{"Everywhere"}) {
		t.Errorf("results are %v, want only the global plugin's", got)
	}
}

func TestPinnedResultComesFirst(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("one", "*", "Alpha", "Beta", "Gamma"))

	before := engine.Search(context.Background(), "a")
	if before[0].Title != "Alpha" {
		t.Fatalf("before pinning the first is %q", before[0].Title)
	}

	// Pin the last result, as the context menu does.
	engine.TopMost.AddOrUpdate(before[2])

	after := engine.Search(context.Background(), "a")
	if after[0].Title != "Gamma" {
		t.Errorf("after pinning the first is %q, want Gamma", after[0].Title)
	}
	if after[0].Score != MaxScore {
		t.Errorf("the pinned result scores %d, want %d", after[0].Score, MaxScore)
	}
}

func TestPinnedResultsKeepTheirOrder(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("one", "*", "Alpha", "Beta", "Gamma"))

	results := engine.Search(context.Background(), "a")
	engine.TopMost.AddOrUpdate(results[1]) // Beta
	engine.TopMost.AddOrUpdate(results[2]) // Gamma, pinned last, so highest

	after := engine.Search(context.Background(), "a")
	if got := titlesOf(after); !sameStrings(got[:2], []string{"Gamma", "Beta"}) {
		t.Errorf("the pinned results are %v, want Gamma then Beta", got[:2])
	}
}

func TestSelectedCountLiftsAResult(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("one", "*", "Alpha", "Beta", "Gamma"))

	results := engine.Search(context.Background(), "a")
	beta := results[1]
	if beta.Title != "Beta" {
		t.Fatalf("the second result is %q", beta.Title)
	}

	// Pick Beta often, as the user would.
	for i := 0; i < 5; i++ {
		engine.Selected.Add(beta)
	}

	after := engine.Search(context.Background(), "a")
	if after[0].Title != "Beta" {
		t.Errorf("after picking Beta five times the first is %q", after[0].Title)
	}
}

func TestSelectedCountIsWeightedForTheQuery(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("one", "*", "Alpha", "Beta"))

	results := engine.Search(context.Background(), "a")
	alpha := results[0]
	engine.Selected.Add(alpha)

	// The count for the query weighs five times the count without it.
	if got := engine.Selected.GetSelectedCount(alpha); got != 6 {
		t.Errorf("GetSelectedCount = %d, want 6 (5 for the query and 1 for the result)", got)
	}

	// The same result under another query counts once.
	other := alpha
	other.OriginQuery = &Query{Search: "other", TrimmedQuery: "other"}
	if got := engine.Selected.GetSelectedCount(other); got != 1 {
		t.Errorf("GetSelectedCount under another query = %d, want 1", got)
	}
}

func TestPriorityLiftsEveryResultOfAPlugin(t *testing.T) {
	engine, manager := newTestEngine(t,
		scoredPlugin("low", "*", 100, "From the low plugin"),
		scoredPlugin("high", "*", 90, "From the high plugin"),
	)

	before := engine.Search(context.Background(), "plugin")
	if before[0].Title != "From the low plugin" {
		t.Fatalf("before the priority change the first is %q", before[0].Title)
	}

	// Give the other plugin priority, as the settings window does.
	manager.UpdateMetadata("high", func(metadata *PluginMetadata) { metadata.Priority = 10 })

	after := engine.Search(context.Background(), "plugin")
	if after[0].Title != "From the high plugin" {
		t.Errorf("after the priority change the first is %q", after[0].Title)
	}
}

func TestResultThatAsksNotToCountIsNotLifted(t *testing.T) {
	pair := resultsPlugin("one", "*", "Alpha", "Beta")
	pair.Plugin.(*fakePlugin).results[1].AddSelectedCount = false
	engine, _ := newTestEngine(t, pair)

	results := engine.Search(context.Background(), "a")
	beta := results[1]
	for i := 0; i < 10; i++ {
		engine.Selected.Add(beta)
	}

	after := engine.Search(context.Background(), "a")
	if after[0].Title != "Alpha" {
		t.Errorf("the first is %q, want Alpha: the result asked not to count", after[0].Title)
	}
}

func TestSearchKeepsTheResultsOfThePluginsThatAnswered(t *testing.T) {
	engine, _ := newTestEngine(t,
		resultsPlugin("fast", "*", "Fast one"),
		resultsPlugin("slow", "*", "Slow one"),
	)
	engine.Manager.mu.Lock()
	engine.Manager.globalPlugins["slow"] = PluginPair{
		Plugin: &fakePlugin{name: "slow", results: []Result{
			NewResult("Slow one", "", "", 50),
		}, delay: 50 * time.Millisecond},
		Metadata: PluginMetadata{ID: "slow", Name: "slow", ActionKeywords: []string{"*"}},
	}
	engine.Manager.mu.Unlock()

	// The plugins answer at the same time, so the counting is guarded.
	var mu sync.Mutex
	var batches int
	var last []Result
	engine.SearchStreaming(context.Background(), "one", func(batch ResultBatch) {
		mu.Lock()
		defer mu.Unlock()
		batches++
		last = batch.Results
	})

	if batches != 2 {
		t.Errorf("the launcher was told about %d answers, want 2", batches)
	}
	if got := titlesOf(last); !sameStrings(got, []string{"Fast one", "Slow one"}) {
		t.Errorf("the last answer held %v, want both plugins' results", got)
	}
}

func TestSearchLimit(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("one", "*", "A", "B", "C", "D", "E", "F"))
	engine.Settings.MaxResultsToShow = 3

	if got := engine.Search(context.Background(), "a"); len(got) != 3 {
		t.Errorf("got %d results, want the limit of 3", len(got))
	}
}

func TestFailingPluginSaysSo(t *testing.T) {
	pair := PluginPair{
		Plugin:   &fakePlugin{name: "broken", err: fmt.Errorf("the plugin is broken")},
		Metadata: PluginMetadata{ID: "broken", Name: "Broken", ActionKeywords: []string{"*"}},
	}
	engine, _ := newTestEngine(t, pair)

	results := engine.Search(context.Background(), "anything")
	if len(results) != 1 {
		t.Fatalf("got %d results, want one saying the plugin failed", len(results))
	}
	if !strings.Contains(results[0].Title, "Broken") {
		t.Errorf("the result is %q, want it to name the plugin", results[0].Title)
	}
}

func TestHomeQueryAsksTheHomePluginsAndListsHistory(t *testing.T) {
	pair := PluginPair{
		Plugin:   &fakePlugin{name: "home", homeOnly: true, results: []Result{NewResult("Home result", "", "", 0)}},
		Metadata: PluginMetadata{ID: "home", Name: "Home", ActionKeywords: []string{"*"}},
	}
	engine, _ := newTestEngine(t, pair)
	engine.Settings.ShowHomePage = true
	engine.Settings.ShowHistoryResultsForHomePage = true

	// Open a result, which the home page should then list.
	engine.History.Add(Result{
		Title:           "Opened before",
		PluginID:        "one",
		PluginDirectory: "C:/plugins/one",
		OriginQuery:     &Query{TrimmedQuery: "opened"},
	})

	results := engine.Search(context.Background(), "")
	titles := titlesOf(results)

	if !contains(titles, "Home result") {
		t.Errorf("the home query answered %v, want the home plugin's result", titles)
	}
	if !contains(titles, "Opened before") {
		t.Errorf("the home query answered %v, want the history entry", titles)
	}
}

func TestHomeQueryDoesNotListHistoryWhenTheUserTurnedItOff(t *testing.T) {
	engine, _ := newTestEngine(t)
	engine.Settings.ShowHomePage = true
	engine.Settings.ShowHistoryResultsForHomePage = false
	engine.History.Add(Result{
		Title:       "Opened before",
		PluginID:    "one",
		OriginQuery: &Query{TrimmedQuery: "opened"},
	})

	if got := titlesOf(engine.Search(context.Background(), "")); contains(got, "Opened before") {
		t.Errorf("the home query answered %v, want no history", got)
	}
}

func TestCustomShortcutExpandsWhatTheUserTyped(t *testing.T) {
	engine, _ := newTestEngine(t, resultsPlugin("one", "*", "Expanded"))
	engine.CustomShortcuts = []CustomShortcut{{Key: "gg", Expanded: "google"}}

	engine.Search(context.Background(), "gg")

	plugin := engine.Manager.AllLoadedPlugins()[0].Plugin.(*fakePlugin)
	if len(plugin.queries) == 0 {
		t.Fatal("the plugin was not asked")
	}
	if got := plugin.queries[0].Search; got != "google" {
		t.Errorf("the plugin was asked for %q, want the expanded query", got)
	}
}

func TestHistoryKeepsTheLastOpenedAndItsLimit(t *testing.T) {
	dir := t.TempDir()
	history := LoadHistory(filepath.Join(dir, "History.json"))

	for i := 0; i < HistoryLimit+20; i++ {
		history.Add(Result{
			Title:       fmt.Sprintf("Result %d", i),
			PluginID:    "one",
			OriginQuery: &Query{TrimmedQuery: fmt.Sprintf("query %d", i)},
		})
	}

	if len(history.LastOpenedHistoryItems) > HistoryLimit+1 {
		t.Errorf("history holds %d entries, want at most %d", len(history.LastOpenedHistoryItems), HistoryLimit+1)
	}

	// Saving and reading it back keeps what it held.
	if err := history.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded := LoadHistory(filepath.Join(dir, "History.json"))
	if len(reloaded.LastOpenedHistoryItems) != len(history.LastOpenedHistoryItems) {
		t.Errorf("reloaded %d entries, want %d",
			len(reloaded.LastOpenedHistoryItems), len(history.LastOpenedHistoryItems))
	}
	last := reloaded.LastOpenedHistoryItems[len(reloaded.LastOpenedHistoryItems)-1]
	if last.Title != fmt.Sprintf("Result %d", HistoryLimit+19) {
		t.Errorf("the last entry is %q", last.Title)
	}
}

func TestHistoryDoesNotRepeatTheLastOpenedResult(t *testing.T) {
	history := LoadHistory(filepath.Join(t.TempDir(), "History.json"))
	result := Result{Title: "One", PluginID: "p", OriginQuery: &Query{TrimmedQuery: "q"}}

	history.Add(result)
	history.Add(result)
	history.Add(result)

	if len(history.LastOpenedHistoryItems) != 1 {
		t.Errorf("history holds %d entries, want one", len(history.LastOpenedHistoryItems))
	}
}

func TestHistorySkipsResultsWithNoQueryOrNoPlugin(t *testing.T) {
	history := LoadHistory(filepath.Join(t.TempDir(), "History.json"))

	history.Add(Result{Title: "No query", PluginID: "p"})
	history.Add(Result{Title: "No plugin", OriginQuery: &Query{TrimmedQuery: "q"}})

	if len(history.LastOpenedHistoryItems) != 0 {
		t.Errorf("history holds %d entries, want none", len(history.LastOpenedHistoryItems))
	}
}

func TestTopMostRecordRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "MultipleTopMostRecord.json")
	record := LoadTopMostRecord(path)

	result := Result{
		Title:       "Pinned",
		PluginID:    "p",
		OriginQuery: &Query{TrimmedQuery: "query"},
	}
	record.AddOrUpdate(result)
	if err := record.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded := LoadTopMostRecord(path)
	if !reloaded.IsTopMost(result) {
		t.Error("the pinned result was not remembered")
	}
	if got := reloaded.GetTopMostIndex(result); got != 0 {
		t.Errorf("GetTopMostIndex = %d, want 0", got)
	}

	reloaded.Remove(result)
	if reloaded.IsTopMost(result) {
		t.Error("the result is still pinned after being unpinned")
	}
}

func TestUserSelectedRecordRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "UserSelectedRecord.json")
	record := LoadUserSelectedRecord(path)

	result := Result{Title: "Picked", PluginID: "p", OriginQuery: &Query{TrimmedQuery: "query"}}
	record.Add(result)
	if err := record.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded := LoadUserSelectedRecord(path)
	// Six for the query and the result together, which is what the
	// reference counts too: with no action keyword and no search text the
	// two hashes of a result are the same, so the one pick counts twice.
	if got := reloaded.GetSelectedCount(result); got != 12 {
		t.Errorf("GetSelectedCount = %d, want 12", got)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
