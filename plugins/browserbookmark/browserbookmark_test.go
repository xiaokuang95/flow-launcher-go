package browserbookmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"launch/core"
)

// fakeReader stands in for the browsers, so that a test reads a list of its
// own rather than the machine's own browsers.
type fakeReader struct {
	bookmarks []Bookmark
	settings  []Settings
}

func (f *fakeReader) Read(settings Settings) ([]Bookmark, error) {
	f.settings = append(f.settings, settings)
	return f.bookmarks, nil
}

func newTestPlugin(t *testing.T, bookmarks ...Bookmark) (*Plugin, *fakeReader) {
	t.Helper()

	reader := &fakeReader{bookmarks: bookmarks}
	plugin := New()
	plugin.reader = reader
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	plugin.bookmarks = bookmarks
	return plugin, reader
}

func sample() []Bookmark {
	return []Bookmark{
		{Name: "Go by Example", URL: "https://gobyexample.com", Source: chromeSource},
		{Name: "The Go Programming Language", URL: "https://go.dev", Source: chromeSource},
		{Name: "Hacker News", URL: "https://news.ycombinator.com", Source: edgeSource},
	}
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestQueryListsEveryBookmarkWhenNothingIsTyped(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results %v, want three", len(results), titlesOf(results))
	}
	for _, result := range results {
		if result.Score != 5 {
			t.Errorf("%q scores %d, want 5 when nothing is typed", result.Title, result.Score)
		}
	}
}

func TestQueryFindsABookmarkByName(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "hacker"})
	if len(results) != 1 || results[0].Title != "Hacker News" {
		t.Fatalf("the results are %v", titlesOf(results))
	}
	if results[0].SubTitle != "https://news.ycombinator.com" {
		t.Errorf("the subtitle is %q, want where it points", results[0].SubTitle)
	}
	if results[0].CopyText() != "https://news.ycombinator.com" {
		t.Errorf("CopyText is %q, want the address", results[0].CopyText())
	}
}

func TestQueryFindsABookmarkByItsAddress(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "ycombinator"})
	if len(results) != 1 || results[0].Title != "Hacker News" {
		t.Errorf("the results are %v, want the bookmark whose address matched", titlesOf(results))
	}
}

func TestOpeningABookmarkAsksTheLauncher(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)
	opened := ""
	plugin.api.Hooks.OpenURL = func(rawURL string, inPrivate *bool) { opened = rawURL }

	results, _ := plugin.Query(context.Background(), core.Query{Search: "hacker"})
	results[0].Action(core.ActionContext{})

	if opened != "https://news.ycombinator.com" {
		t.Errorf("the launcher was asked to open %q", opened)
	}
}

func TestContextMenuCopiesTheAddress(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)
	copied := ""
	plugin.api.Hooks.CopyToClipboard = func(text string, directCopy, notify bool) { copied = text }

	results, _ := plugin.Query(context.Background(), core.Query{Search: "hacker"})
	items := plugin.LoadContextMenus(results[0])
	if len(items) != 1 {
		t.Fatalf("the menu has %d items, want one", len(items))
	}
	items[0].Action(core.ActionContext{})

	if copied != "https://news.ycombinator.com" {
		t.Errorf("the clipboard was given %q, want the address", copied)
	}
}

func TestReloadReadsTheBookmarksAgain(t *testing.T) {
	plugin, reader := newTestPlugin(t, sample()...)
	reader.bookmarks = []Bookmark{{Name: "New", URL: "https://example.com"}}

	if err := plugin.ReloadData(context.Background()); err != nil {
		t.Fatal(err)
	}
	results, _ := plugin.Query(context.Background(), core.Query{})
	if len(results) != 1 || results[0].Title != "New" {
		t.Errorf("the results are %v, want what was read again", titlesOf(results))
	}
}

// TestReadingAChromiumBookmarkFile covers the shape of the file the
// Chromium browsers keep their bookmarks in.
func TestReadingAChromiumBookmarkFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Bookmarks")
	content := `{
	  "roots": {
	    "bookmark_bar": {
	      "children": [
	        {"type": "url", "name": "Example", "url": "https://example.com"},
	        {"type": "folder", "name": "Work", "children": [
	          {"type": "url", "name": "Reports", "url": "https://reports.example.com"}
	        ]}
	      ]
	    },
	    "other": {"children": []}
	  }
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	bookmarks := readBookmarkFile(path, "Test Browser")
	if len(bookmarks) != 2 {
		t.Fatalf("read %d bookmarks, want two", len(bookmarks))
	}
	if bookmarks[0].Name != "Example" || bookmarks[0].URL != "https://example.com" {
		t.Errorf("the first bookmark is %+v", bookmarks[0])
	}
	// A bookmark inside a folder is read too.
	if bookmarks[1].Name != "Reports" {
		t.Errorf("the second bookmark is %+v, want the one inside the folder", bookmarks[1])
	}
}

func TestReadingProfilesOfAChromiumBrowser(t *testing.T) {
	root := t.TempDir()
	for _, profile := range []string{"Default", "Profile 1"} {
		directory := filepath.Join(root, profile)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		content := `{"roots": {"bookmark_bar": {"children": [{"type": "url", "name": "` + profile + `", "url": "https://example.com"}]}}}`
		if err := os.WriteFile(filepath.Join(directory, "Bookmarks"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bookmarks := readChromiumBrowsers([]browserSource{{path: root, name: "Test Browser"}})
	if len(bookmarks) != 2 {
		t.Fatalf("read %d bookmarks, want one per profile", len(bookmarks))
	}
	if bookmarks[0].Source != "Test Browser" {
		t.Errorf("the first profile is called %q, want the browser's own name", bookmarks[0].Source)
	}
	if bookmarks[1].Source != "Test Browser (Profile 1)" {
		t.Errorf("the second profile is called %q, want it named after its folder", bookmarks[1].Source)
	}
}
