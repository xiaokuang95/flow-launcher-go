package browserbookmark

import (
	"context"
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"launch/core"
)

// The browser bookmarks' own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about the bookmarks.
type Settings struct {
	// OpenInNewBrowserWindow opens a bookmark in a new window rather than
	// in a new tab of the browser the system has.
	OpenInNewBrowserWindow bool `json:"OpenInNewBrowserWindow"`
	// BrowserPath is the browser a bookmark is opened in, when the user
	// wants one of their own.
	BrowserPath string `json:"BrowserPath"`
	// EnableFavicons shows the picture a site gives itself rather than the
	// bookmark icon.
	EnableFavicons bool `json:"EnableFavicons"`

	// LoadChromeBookmark, LoadFirefoxBookmark and LoadEdgeBookmark say
	// which browsers are read.
	LoadChromeBookmark  bool `json:"LoadChromeBookmark"`
	LoadFirefoxBookmark bool `json:"LoadFirefoxBookmark"`
	LoadEdgeBookmark    bool `json:"LoadEdgeBookmark"`

	// CustomChromiumBrowsers are browsers of the user's own, which keep
	// their bookmarks the way the Chromium browsers do.
	CustomChromiumBrowsers []CustomBrowser `json:"CustomChromiumBrowsers"`
}

// CustomBrowser is a browser of the user's own.
type CustomBrowser struct {
	// Name is what it is called.
	Name string `json:"Name"`
	// DataPath is its user data folder.
	DataPath string `json:"DataPath"`
	// BookmarkFilePath is the file its bookmarks are in, for a browser
	// whose bookmarks are not where the Chromium browsers keep theirs.
	BookmarkFilePath string `json:"BookmarkFilePath"`
}

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{
		OpenInNewBrowserWindow: true,
		LoadChromeBookmark:     true,
		LoadFirefoxBookmark:    true,
		LoadEdgeBookmark:       true,
	}
}

// Bookmark is one bookmark.
type Bookmark struct {
	// Name is what it is called.
	Name string
	// URL is where it points.
	URL string
	// Source is the browser it came from.
	Source string
	// FaviconPath is the picture of the site, when one was found.
	FaviconPath string
}

// Reader reads the bookmarks of the browsers on the machine. It is kept
// behind a field so that a test reads a folder of its own rather than the
// machine's own browsers.
type Reader interface {
	// Read returns the bookmarks of the browsers the settings ask for.
	Read(settings Settings) ([]Bookmark, error)
}

// Plugin is the browser bookmarks: it finds what the user saved in their
// browsers.
type Plugin struct {
	core.BasePlugin

	api       *core.PublicAPI
	settings  Settings
	words     *core.PluginTranslator
	reader    Reader
	bookmarks []Bookmark
}

// New makes the browser bookmarks.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "0ECADE17459B49F587BF81DC3A125110",
			Name:           "Browser Bookmarks",
			Description:    "Search your browser bookmarks",
			Author:         "qianlifeng, Ioannis G.",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords: []string{"b"},
			IcoPath:        "Images/bookmark.png",
		}},
		reader: systemReader{},
	}
}

// Init reads the settings and the bookmarks.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")

	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "Browser Bookmarks", "Settings.json")
	if !p.settings.LoadChromeBookmark && !p.settings.LoadFirefoxBookmark &&
		!p.settings.LoadEdgeBookmark && len(p.settings.CustomChromiumBrowsers) == 0 {
		p.settings = DefaultSettings()
	}
	p.reload()
	return nil
}

// reload reads the bookmarks again.
func (p *Plugin) reload() {
	bookmarks, err := p.reader.Read(p.settings)
	if err != nil {
		p.api.LogError("BrowserBookmarks", "reading the bookmarks failed: "+err.Error())
		return
	}
	p.bookmarks = bookmarks
}

// ReloadData reads the bookmarks again, as F5 asks.
func (p *Plugin) ReloadData(ctx context.Context) error {
	p.reload()
	return nil
}

// Query answers what the user typed with the bookmarks whose name or
// address matches it, and lists every bookmark when nothing is typed.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	search := strings.TrimSpace(query.Search)
	matcher := p.api.Matcher()

	results := make([]core.Result, 0, len(p.bookmarks))
	for _, bookmark := range p.bookmarks {
		score := 5
		var highlight []int

		if search != "" {
			// A bookmark is found by its name, or by its address.
			match := matcher.FuzzyMatch(search, bookmark.Name)
			if !match.IsSearchPrecisionScoreMet() {
				match = matcher.FuzzyMatch(search, bookmark.URL)
			} else {
				highlight = match.MatchData
			}
			if !match.IsSearchPrecisionScoreMet() {
				continue
			}
			score = match.Score()
		}

		results = append(results, p.resultFor(bookmark, score, highlight))
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results, nil
}

// resultFor is one bookmark as a result.
func (p *Plugin) resultFor(bookmark Bookmark, score int, highlight []int) core.Result {
	icon := "Images/bookmark.png"
	if bookmark.FaviconPath != "" && core.FileExists(bookmark.FaviconPath) {
		icon = bookmark.FaviconPath
	}

	result := core.NewResult(bookmark.Name, bookmark.URL, icon, score)
	result.TitleHighlightData = highlight
	result.SetCopyText(bookmark.URL)
	result.ContextData = bookmark
	result.Action = func(core.ActionContext) bool {
		p.api.OpenWebUrl(bookmark.URL, nil)
		return true
	}
	return result
}

// Settings hands the plugin's settings over, which the launcher shows and
// changes in a window of its own.
func (p *Plugin) Settings() any { return &p.settings }

// Save writes the settings out.
func (p *Plugin) Save() {
	_ = core.SaveSettingJsonStorage(p.api, "Browser Bookmarks", "Settings.json", p.settings)
}

// LoadContextMenus offers the bookmark's own menu: opening it in a new
// window, and copying where it points.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	bookmark, ok := selectedResult.ContextData.(Bookmark)
	if !ok {
		return nil
	}

	copyItem := core.NewResult(p.words.T("flowlauncher_plugin_browserbookmark_copyurl_title"), bookmark.URL, "Images/copy.png", 0)
	copyItem.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: "\ue8c8"}
	copyItem.Action = func(core.ActionContext) bool {
		p.api.CopyToClipboard(bookmark.URL, false, true)
		return true
	}
	return []core.Result{copyItem}
}

// The names the browsers are called in a result.
const (
	chromeSource     = "Google Chrome"
	canarySource     = "Google Chrome Canary"
	chromiumSource   = "Chromium"
	edgeSource       = "Microsoft Edge"
	edgeDevSource    = "Microsoft Edge Dev"
	edgeCanarySource = "Microsoft Edge Canary"
)

// systemReader reads the bookmarks of the browsers on the machine.
type systemReader struct{}

// Read returns the bookmarks of the browsers the settings ask for.
func (systemReader) Read(settings Settings) ([]Bookmark, error) {
	var bookmarks []Bookmark
	seen := map[string]bool{}

	add := func(found []Bookmark) {
		for _, bookmark := range found {
			// The same bookmark is often in more than one profile.
			key := bookmark.Name + "\x00" + bookmark.URL
			if seen[key] {
				continue
			}
			seen[key] = true
			bookmarks = append(bookmarks, bookmark)
		}
	}

	if settings.LoadChromeBookmark {
		add(readChromiumBrowsers([]browserSource{
			{path: filepath.Join(localAppData(), `Google\Chrome\User Data`), name: chromeSource},
			{path: filepath.Join(localAppData(), `Google\Chrome SxS\User Data`), name: canarySource},
			{path: filepath.Join(localAppData(), `Chromium\User Data`), name: chromiumSource},
		}))
	}

	if settings.LoadEdgeBookmark {
		add(readChromiumBrowsers([]browserSource{
			{path: filepath.Join(localAppData(), `Microsoft\Edge\User Data`), name: edgeSource},
			{path: filepath.Join(localAppData(), `Microsoft\Edge Dev\User Data`), name: edgeDevSource},
			{path: filepath.Join(localAppData(), `Microsoft\Edge SxS\User Data`), name: edgeCanarySource},
		}))
	}

	for _, custom := range settings.CustomChromiumBrowsers {
		if custom.BookmarkFilePath != "" {
			add(readBookmarkFile(custom.BookmarkFilePath, custom.Name))
			continue
		}
		if custom.DataPath != "" {
			add(readChromiumBrowsers([]browserSource{{path: custom.DataPath, name: custom.Name}}))
		}
	}

	// Firefox keeps its bookmarks in a database rather than in a file this
	// plugin can read without a database of its own, so its bookmarks are
	// left out.
	return bookmarks, nil
}

// browserSource is a browser's user data folder and what it is called.
type browserSource struct {
	path string
	name string
}

// readChromiumBrowsers reads the bookmarks of every profile of a Chromium
// browser.
func readChromiumBrowsers(sources []browserSource) []Bookmark {
	var bookmarks []Bookmark
	for _, source := range sources {
		profiles, err := os.ReadDir(source.path)
		if err != nil {
			continue
		}
		for _, profile := range profiles {
			if !profile.IsDir() {
				continue
			}
			name := source.name
			if profile.Name() != "Default" {
				name = source.name + " (" + profile.Name() + ")"
			}
			file := filepath.Join(source.path, profile.Name(), "Bookmarks")
			bookmarks = append(bookmarks, readBookmarkFile(file, name)...)
		}
	}
	return bookmarks
}

// readBookmarkFile reads one browser's bookmarks file, which is JSON: the
// bookmarks are in named roots, each holding folders and bookmarks.
func readBookmarkFile(path, source string) []Bookmark {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var file struct {
		Roots map[string]json.RawMessage `json:"roots"`
	}
	if err := json.Unmarshal(content, &file); err != nil {
		return nil
	}

	var bookmarks []Bookmark
	for name, root := range file.Roots {
		// Opera keeps its bookmarks one level further in.
		if name == "custom_root" {
			var nested struct {
				Roots map[string]json.RawMessage `json:"roots"`
			}
			if err := json.Unmarshal(root, &nested); err == nil && len(nested.Roots) > 0 {
				for _, inner := range nested.Roots {
					bookmarks = append(bookmarks, readFolder(inner, source)...)
				}
				continue
			}
		}
		bookmarks = append(bookmarks, readFolder(root, source)...)
	}
	return bookmarks
}

// bookmarkNode is one entry of a bookmarks file: a bookmark, a folder of
// them, or one of the named roots, which hold folders.
type bookmarkNode struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	URL      string         `json:"url"`
	Children []bookmarkNode `json:"children"`
}

// readFolder reads a folder of bookmarks, and the folders inside it.
func readFolder(raw json.RawMessage, source string) []Bookmark {
	var node bookmarkNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil
	}
	return walkBookmarkNode(node, source)
}

// walkBookmarkNode reads a bookmark, or everything inside a folder.
func walkBookmarkNode(node bookmarkNode, source string) []Bookmark {
	switch node.Type {
	case "url":
		if node.URL == "" {
			return nil
		}
		return []Bookmark{{Name: node.Name, URL: node.URL, Source: source}}
	case "folder", "workspace":
		// An Edge workspace holds bookmarks the way a folder does.
	default:
		// A root has no type of its own and holds folders.
	}

	var bookmarks []Bookmark
	for _, child := range node.Children {
		bookmarks = append(bookmarks, walkBookmarkNode(child, source)...)
	}
	return bookmarks
}

// localAppData is where the browsers keep their user data.
func localAppData() string {
	if value := os.Getenv("LOCALAPPDATA"); value != "" {
		return value
	}
	return os.Getenv("APPDATA")
}
