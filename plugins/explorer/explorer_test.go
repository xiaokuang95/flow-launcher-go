package explorer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"launch/core"
)

// fakeIndex stands in for the machine's files, so that a test searches a
// list of its own rather than the machine's own folders.
type fakeIndex struct {
	found   []SearchResult
	queries []string
}

func (f *fakeIndex) Available() (bool, string) { return true, "" }

func (f *fakeIndex) Search(query string, limit int, excluded []string) ([]SearchResult, error) {
	f.queries = append(f.queries, query)
	return f.found, nil
}

func (f *fakeIndex) ContentSearch(query string, limit int, excluded []string) ([]SearchResult, error) {
	f.queries = append(f.queries, query)
	return f.found, nil
}

func newTestPlugin(t *testing.T) (*Plugin, *fakeIndex) {
	t.Helper()

	index := &fakeIndex{}
	plugin := New()
	plugin.index = index
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	return plugin, index
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestPathSearchListsAFolder(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"alpha.txt", "beta.txt", "gamma.md"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(directory, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}

	plugin, _ := newTestPlugin(t)
	plugin.settings.PathSearchKeywordEnabled = true

	results, err := plugin.Query(context.Background(), core.Query{Search: directory})
	if err != nil {
		t.Fatal(err)
	}

	titles := titlesOf(results)
	// The folder itself is offered first, so that Enter opens it.
	if len(titles) == 0 || titles[0] != filepath.Base(directory) {
		t.Errorf("the results are %v, want the folder itself first", titles)
	}
	for _, want := range []string{"alpha.txt", "beta.txt", "gamma.md", "folder"} {
		if !contains(titles, want) {
			t.Errorf("the results %v do not hold %q", titles, want)
		}
	}
}

func TestPathSearchCompletesWhatIsTyped(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"alpha.txt", "alpine.txt", "beta.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	plugin, _ := newTestPlugin(t)
	plugin.settings.PathSearchKeywordEnabled = true

	results, _ := plugin.Query(context.Background(), core.Query{Search: filepath.Join(directory, "al")})
	titles := titlesOf(results)

	if !contains(titles, "alpha.txt") || !contains(titles, "alpine.txt") {
		t.Errorf("the results are %v, want the two that start with al", titles)
	}
	if contains(titles, "beta.txt") {
		t.Errorf("the results are %v, want nothing that does not match", titles)
	}
}

func TestSearchFindsFiles(t *testing.T) {
	plugin, index := newTestPlugin(t)
	index.found = []SearchResult{
		{FullPath: `C:\Users\crazy\Documents\report.pdf`, Type: ResultTypeFile, Score: 100},
	}
	plugin.settings.IndexSearchKeywordEnabled = true

	results, err := plugin.Query(context.Background(), core.Query{Search: "report"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %v, want the file", titlesOf(results))
	}
	if results[0].Title != "report.pdf" {
		t.Errorf("the title is %q", results[0].Title)
	}
	if results[0].CopyText() != `C:\Users\crazy\Documents\report.pdf` {
		t.Errorf("CopyText is %q, want the path", results[0].CopyText())
	}
	if len(index.queries) != 1 || index.queries[0] != "report" {
		t.Errorf("the files were searched for %v", index.queries)
	}
}

func TestQuickAccessListsThePinnedFolders(t *testing.T) {
	directory := t.TempDir()
	plugin, _ := newTestPlugin(t)
	plugin.settings.QuickAccessLinks = []AccessLink{
		{Path: directory, Name: "Work"},
		{Path: directory, Name: "Home"},
	}
	plugin.settings.QuickAccessKeywordEnabled = true

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %v, want the two pinned folders", titlesOf(results))
	}
	if results[0].Title != "Work" || results[1].Title != "Home" {
		t.Errorf("the results are %v", titlesOf(results))
	}
}

func TestQuickAccessMatchesWhatIsTyped(t *testing.T) {
	directory := t.TempDir()
	plugin, _ := newTestPlugin(t)
	plugin.settings.QuickAccessLinks = []AccessLink{
		{Path: directory, Name: "Work"},
		{Path: directory, Name: "Pictures"},
	}
	plugin.settings.QuickAccessKeywordEnabled = true

	results, _ := plugin.Query(context.Background(), core.Query{Search: "work"})
	if len(results) == 0 || results[0].Title != "Work" {
		t.Errorf("the results are %v, want the pinned folder that matched", titlesOf(results))
	}
}

func TestAQueryThePluginDoesNotAnswerIsLeftAlone(t *testing.T) {
	plugin, index := newTestPlugin(t)
	// Every search is turned off.
	plugin.settings.SearchActionKeywordEnabled = false
	plugin.settings.PathSearchKeywordEnabled = false
	plugin.settings.IndexSearchKeywordEnabled = false
	plugin.settings.QuickAccessKeywordEnabled = false

	results, err := plugin.Query(context.Background(), core.Query{Search: "anything"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("got %v, want nothing", titlesOf(results))
	}
	if len(index.queries) != 0 {
		t.Errorf("the files were searched although the plugin is off: %v", index.queries)
	}
}

func TestContentSearchUsesItsOwnKeyword(t *testing.T) {
	plugin, index := newTestPlugin(t)
	index.found = []SearchResult{{FullPath: `C:\notes.txt`, Type: ResultTypeFile, Score: 50}}

	results, err := plugin.Query(context.Background(), core.Query{
		Search: "invoice", ActionKeyword: contentSearchKeyword,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %v, want the file", titlesOf(results))
	}
	if len(index.queries) != 1 || index.queries[0] != "invoice" {
		t.Errorf("the files were searched for %v", index.queries)
	}
}

func TestLooksLikePath(t *testing.T) {
	paths := []string{`C:\Users`, `c:/users`, `C:`, `%appdata%\x`, `\\server\share`, "./here"}
	for _, path := range paths {
		if !looksLikePath(path) {
			t.Errorf("%q should be read as a path", path)
		}
	}
	for _, search := range []string{"report", "my document", "2026 budget"} {
		if looksLikePath(search) {
			t.Errorf("%q should be read as a search", search)
		}
	}
}

func TestPreviousDirectory(t *testing.T) {
	cases := map[string]string{
		`C:\Users\crazy\`:    `C:\Users\crazy\`,
		`C:\Users\crazy\doc`: `C:\Users\crazy`,
	}
	for path, want := range cases {
		if got := previousDirectory(path); got != want {
			t.Errorf("previousDirectory(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestExpandEnvironmentVariables(t *testing.T) {
	t.Setenv("LAUNCH_TEST_DIR", `C:\somewhere`)

	if got := expandEnvironmentVariables(`%LAUNCH_TEST_DIR%\file.txt`); got != `C:\somewhere\file.txt` {
		t.Errorf("expandEnvironmentVariables = %q", got)
	}
	if got := expandEnvironmentVariables("no variables"); got != "no variables" {
		t.Errorf("expandEnvironmentVariables = %q", got)
	}
}

func TestWalkIndexFindsFilesInTheUsersFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	documents := filepath.Join(home, "Documents")
	if err := os.MkdirAll(documents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(documents, "quarterly-report.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A folder a search never looks in.
	hidden := filepath.Join(home, "AppData", "Local")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "quarterly-secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := (&walkIndex{}).Search("quarterly", 10, nil)
	if err != nil {
		t.Fatal(err)
	}

	paths := make([]string, 0, len(found))
	for _, entry := range found {
		paths = append(paths, entry.FullPath)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "quarterly-report.txt" {
		t.Errorf("the search found %v, want only the report", paths)
	}
}

func TestWalkIndexLeavesOutExcludedFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	documents := filepath.Join(home, "Documents")
	if err := os.MkdirAll(documents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(documents, "report.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := (&walkIndex{}).Search("report", 10, []string{documents})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("the search found %v, want nothing from the excluded folder", found)
	}
}

func TestWarmReadsTheFoldersInTheBackground(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	documents := filepath.Join(home, "Documents")
	if err := os.MkdirAll(documents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(documents, "quarterly-report.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	index := &walkIndex{}
	index.Warm()

	// Warm does not read the folders itself: what the walk found is there
	// a moment later.
	waitFor(t, func() bool {
		found, err := index.Search("quarterly", 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		return len(found) == 1
	})
}

func TestAWalkThatIsOldIsAnsweredFromWhatItHas(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	documents := filepath.Join(home, "Documents")
	if err := os.MkdirAll(documents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(documents, "quarterly-report.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	index := &walkIndex{}
	if _, err := index.Search("quarterly", 10, nil); err != nil {
		t.Fatal(err)
	}

	// A file that appeared after the walk, and a walk that has gone old.
	if err := os.WriteFile(filepath.Join(documents, "later.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	index.mu.Lock()
	index.walkedAt = time.Now().Add(-2 * indexAge)
	index.mu.Unlock()

	// The search is answered with what the walk found, rather than waiting
	// for the folders to be read again.
	found, err := index.Search("later", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("the search found %d files, want the search answered from the walk that was kept", len(found))
	}

	// Which it is, in the background.
	waitFor(t, func() bool {
		found, err := index.Search("later", 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		return len(found) == 1
	})
}

// waitFor waits for something to become true, as a walk in the background
// takes a moment.
func waitFor(t *testing.T, done func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("it did not happen")
		}
		time.Sleep(10 * time.Millisecond)
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
