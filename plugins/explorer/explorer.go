package explorer

import (
	"context"
	"embed"
	"path/filepath"
	"strings"

	"launch/core"
)

// The explorer's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about finding files.
type Settings struct {
	// MaxResult is how many files a search may answer with.
	MaxResult int `json:"MaxResult"`
	// QuickAccessLinks are the folders the user pinned, which the empty
	// query lists.
	QuickAccessLinks []AccessLink `json:"QuickAccessLinks"`
	// IndexSearchExcludedSubdirectoryPaths are the folders a search of the
	// index leaves out.
	IndexSearchExcludedSubdirectoryPaths []AccessLink `json:"IndexSearchExcludedSubdirectoryPaths"`

	// EditorPath and FolderEditorPath are the programs Ctrl+E and
	// Ctrl+Shift+E open a file and a folder in.
	EditorPath       string `json:"EditorPath"`
	FolderEditorPath string `json:"FolderEditorPath"`
	// ShellPath is the shell Ctrl+Shift+E opens a folder in.
	ShellPath string `json:"ShellPath"`
	// ExcludedFileTypes are the file types a search leaves out.
	ExcludedFileTypes string `json:"ExcludedFileTypes"`

	// UseLocationAsWorkingDir starts a program in the folder it was found
	// in.
	UseLocationAsWorkingDir bool `json:"UseLocationAsWorkingDir"`
	// DefaultOpenFolderInFileManager opens a folder in the file manager
	// rather than searching inside it.
	DefaultOpenFolderInFileManager bool `json:"DefaultOpenFolderInFileManager"`
	// DisplayMoreInformationInToolTip shows a file's size and dates on
	// hover.
	DisplayMoreInformationInToolTip bool `json:"DisplayMoreInformationInToolTip"`
	// BoostHomeFolderScore lifts the results in the user's own folders.
	BoostHomeFolderScore bool `json:"BoostHomeFolderScore"`

	// SearchActionKeyword and the ones after it are what each kind of
	// search answers to, and whether it is on.
	SearchActionKeyword             string `json:"SearchActionKeyword"`
	SearchActionKeywordEnabled      bool   `json:"SearchActionKeywordEnabled"`
	PathSearchActionKeyword         string `json:"PathSearchActionKeyword"`
	PathSearchKeywordEnabled        bool   `json:"PathSearchKeywordEnabled"`
	IndexSearchActionKeyword        string `json:"IndexSearchActionKeyword"`
	IndexSearchKeywordEnabled       bool   `json:"IndexSearchKeywordEnabled"`
	FileContentSearchActionKeyword  string `json:"FileContentSearchActionKeyword"`
	FileContentSearchKeywordEnabled bool   `json:"FileContentSearchKeywordEnabled"`
	QuickAccessActionKeyword        string `json:"QuickAccessActionKeyword"`
	QuickAccessKeywordEnabled       bool   `json:"QuickAccessKeywordEnabled"`
	FolderSearchActionKeyword       string `json:"FolderSearchActionKeyword"`
	FolderSearchKeywordEnabled      bool   `json:"FolderSearchKeywordEnabled"`
	FileSearchActionKeyword         string `json:"FileSearchActionKeyword"`
	FileSearchKeywordEnabled        bool   `json:"FileSearchKeywordEnabled"`

	// WarnWindowsSearchServiceOff says whether to say that the Windows
	// index is not running when a search finds nothing because of it.
	WarnWindowsSearchServiceOff bool `json:"WarnWindowsSearchServiceOff"`

	// ShowFileSizeInPreviewPanel and the two after it say what the preview
	// panel shows about a file.
	ShowFileSizeInPreviewPanel     bool `json:"ShowFileSizeInPreviewPanel"`
	ShowCreatedDateInPreviewPanel  bool `json:"ShowCreatedDateInPreviewPanel"`
	ShowModifiedDateInPreviewPanel bool `json:"ShowModifiedDateInPreviewPanel"`
	ShowFileAgeInPreviewPanel      bool `json:"ShowFileAgeInPreviewPanel"`
	// PreviewPanelDateFormat and PreviewPanelTimeFormat are how those dates
	// are written.
	PreviewPanelDateFormat string `json:"PreviewPanelDateFormat"`
	PreviewPanelTimeFormat string `json:"PreviewPanelTimeFormat"`
}

// AccessLink is a folder the user pinned to the empty query.
type AccessLink struct {
	// Path is the folder.
	Path string `json:"Path"`
	// Name is what it is called; empty means the folder's own name.
	Name string `json:"Name"`
}

// The keywords the searches answer to by default, as the reference has
// them.
const (
	contentSearchKeyword = "doc:"
	globalKeyword        = "*"
)

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{
		MaxResult:                       100,
		SearchActionKeyword:             globalKeyword,
		SearchActionKeywordEnabled:      true,
		PathSearchActionKeyword:         globalKeyword,
		FileContentSearchActionKeyword:  contentSearchKeyword,
		FileContentSearchKeywordEnabled: true,
		IndexSearchActionKeyword:        globalKeyword,
		QuickAccessActionKeyword:        globalKeyword,
		FolderSearchActionKeyword:       globalKeyword,
		FileSearchActionKeyword:         globalKeyword,
		BoostHomeFolderScore:            true,
		WarnWindowsSearchServiceOff:     true,
		ShowFileSizeInPreviewPanel:      true,
		ShowCreatedDateInPreviewPanel:   true,
		ShowModifiedDateInPreviewPanel:  true,
		PreviewPanelDateFormat:          "2006-01-02",
		PreviewPanelTimeFormat:          "15:04",
	}
}

// Plugin is the explorer: it finds files and folders, and shows what a
// folder holds.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	settings Settings
	words    *core.PluginTranslator
	// index finds the files on the machine.
	index FileIndex
	// deleter removes what the user asks to remove, kept behind a field so
	// that a test watches what was asked for rather than losing a file.
	deleter Deleter
}

// New makes the explorer.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:                     "572be03c74c642baae319fc283e561a8",
			Name:                   "Explorer",
			Description:            "Find and manage files and folders via Windows Search or Everything",
			Author:                 "Jeremy Wu",
			Version:                "1.0.0",
			Website:                "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords:         []string{globalKeyword, contentSearchKeyword},
			HideActionKeywordPanel: true,
			IcoPath:                "Images/explorer.png",
		}},
		index:   &walkIndex{},
		deleter: systemDeleter{},
	}
}

// Init reads the explorer's settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")

	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "Explorer", "Settings.json")
	if p.settings.MaxResult == 0 {
		p.settings = DefaultSettings()
	}
	// A quick access link without a name is called after its folder.
	for i := range p.settings.QuickAccessLinks {
		if p.settings.QuickAccessLinks[i].Name == "" {
			p.settings.QuickAccessLinks[i].Name = filepath.Base(p.settings.QuickAccessLinks[i].Path)
		}
	}

	// The folders a search looks in are read now, in the background, so
	// that the first thing the user types does not have to wait for the
	// walk.
	if warmer, ok := p.index.(interface{ Warm() }); ok {
		warmer.Warm()
	}
	return nil
}

// ReloadData reads the folders again, as F5 asks.
func (p *Plugin) ReloadData(ctx context.Context) error {
	if reloadable, ok := p.index.(interface{ Reload() }); ok {
		reloadable.Reload()
	}
	return nil
}

// Query answers a search: what the folders the user pinned hold when
// nothing is typed, the contents of a folder when a path is typed, and the
// machine's index of its files otherwise.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	keyword := query.ActionKeyword
	if keyword == "" {
		keyword = globalKeyword
	}

	// A query the plugin does not answer to is left to the others.
	if !p.enabled(keyword) {
		return nil, nil
	}

	search := strings.TrimSpace(query.Search)

	// Nothing typed: the folders the user pinned.
	if search == "" {
		if p.usesQuickAccess(keyword) {
			return p.quickAccessResults(query, ""), nil
		}
		return nil, nil
	}

	// A path is read as a folder to look inside, or a folder to complete.
	if looksLikePath(search) {
		if p.usesPathSearch(keyword) {
			return p.pathSearch(ctx, query)
		}
		return nil, nil
	}

	if p.usesQuickAccess(keyword) {
		if results := p.quickAccessResults(query, search); len(results) > 0 {
			return results, nil
		}
	}

	if p.usesContentSearch(keyword) {
		return p.contentSearch(ctx, query)
	}
	if p.usesIndexSearch(keyword) {
		return p.indexSearch(ctx, query)
	}
	return nil, nil
}

// enabled reports whether the plugin answers to a keyword at all.
func (p *Plugin) enabled(keyword string) bool {
	for _, kind := range p.kinds() {
		if p.keyword(kind) == keyword && p.on(kind) {
			return true
		}
	}
	return false
}

// active is the searches a keyword turns on, which is what the reference
// works out from the keyword and from which searches are enabled.
func (p *Plugin) active(keyword string) map[kind]bool {
	active := map[kind]bool{}
	for _, k := range p.kinds() {
		if p.keyword(k) == keyword && p.on(k) {
			active[k] = true
		}
	}
	return active
}

// usesPathSearch reports whether a path is read as a folder to look in.
// The general search reads a path the same way the path search does.
func (p *Plugin) usesPathSearch(keyword string) bool {
	active := p.active(keyword)
	return active[kindPathSearch] || active[kindSearch]
}

// usesIndexSearch reports whether the files are searched. The general
// search, the file search, the folder search and the index search all do
// it.
func (p *Plugin) usesIndexSearch(keyword string) bool {
	active := p.active(keyword)
	return active[kindFileSearch] || active[kindFolderSearch] ||
		active[kindIndexSearch] || active[kindSearch]
}

// usesContentSearch reports whether files are searched by what is in them.
func (p *Plugin) usesContentSearch(keyword string) bool {
	return p.active(keyword)[kindContentSearch]
}

// usesQuickAccess reports whether the pinned folders are listed.
func (p *Plugin) usesQuickAccess(keyword string) bool {
	return p.active(keyword)[kindQuickAccess]
}

// kind is one of the searches the explorer can do.
type kind int

// The searches the explorer can do.
const (
	kindSearch kind = iota
	kindPathSearch
	kindIndexSearch
	kindContentSearch
	kindQuickAccess
	kindFolderSearch
	kindFileSearch
)

// kinds are all of them, in the order the reference lists them.
func (p *Plugin) kinds() []kind {
	return []kind{kindSearch, kindPathSearch, kindIndexSearch, kindContentSearch, kindQuickAccess, kindFolderSearch, kindFileSearch}
}

// keyword is what a search answers to.
func (p *Plugin) keyword(k kind) string {
	switch k {
	case kindPathSearch:
		return p.settings.PathSearchActionKeyword
	case kindIndexSearch:
		return p.settings.IndexSearchActionKeyword
	case kindContentSearch:
		return p.settings.FileContentSearchActionKeyword
	case kindQuickAccess:
		return p.settings.QuickAccessActionKeyword
	case kindFolderSearch:
		return p.settings.FolderSearchActionKeyword
	case kindFileSearch:
		return p.settings.FileSearchActionKeyword
	default:
		return p.settings.SearchActionKeyword
	}
}

// on reports whether a search is turned on.
func (p *Plugin) on(k kind) bool {
	switch k {
	case kindPathSearch:
		return p.settings.PathSearchKeywordEnabled
	case kindIndexSearch:
		return p.settings.IndexSearchKeywordEnabled
	case kindContentSearch:
		return p.settings.FileContentSearchKeywordEnabled
	case kindQuickAccess:
		return p.settings.QuickAccessKeywordEnabled
	case kindFolderSearch:
		return p.settings.FolderSearchKeywordEnabled
	case kindFileSearch:
		return p.settings.FileSearchKeywordEnabled
	default:
		return p.settings.SearchActionKeywordEnabled
	}
}

// Settings hands the plugin's settings over, which the launcher shows and
// changes in a window of its own.
func (p *Plugin) Settings() any { return &p.settings }

// Save writes the settings out.
func (p *Plugin) Save() {
	_ = core.SaveSettingJsonStorage(p.api, "Explorer", "Settings.json", p.settings)
}

// ReloadData reads the folders again, as F5 asks.
