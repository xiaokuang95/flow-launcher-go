package core

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Plugin is what a plugin of any kind can do: say what it is, be started,
// and answer a query. Everything else is an optional interface a plugin may
// also implement.
type Plugin interface {
	// Metadata is what the launcher knows about the plugin: its id, its
	// name, and the action keywords that send a query to it.
	Metadata() PluginMetadata
	// Init is called once, before any query, with the plugin's metadata
	// and the API it may call back into.
	Init(ctx *PluginInitContext) error
	// Query answers a query with the results it found.
	Query(ctx context.Context, query Query) ([]Result, error)
}

// BasePlugin holds the parts of a plugin that most plugins do not change,
// so that a plugin is its metadata and its query. Embed it:
//
//	type Plugin struct {
//		core.BasePlugin
//	}
//
//	func New() *Plugin {
//		return &Plugin{BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
//			ID:             "...",
//			Name:           "Example",
//			ActionKeywords: []string{"*"},
//		}}}
//	}
type BasePlugin struct {
	// Meta is what the launcher knows about the plugin.
	Meta PluginMetadata
}

// Metadata is what the launcher knows about the plugin.
func (b BasePlugin) Metadata() PluginMetadata { return b.Meta }

// Init is called once, before the first query, and does nothing by default.
func (b BasePlugin) Init(*PluginInitContext) error { return nil }

// ContextMenuPlugin adds items to the menu a result shows on a right click.
type ContextMenuPlugin interface {
	// LoadContextMenus returns the items for a result.
	LoadContextMenus(selectedResult Result) []Result
}

// HomePlugin contributes to the home query, the one shown when the user has
// typed nothing.
type HomePlugin interface {
	// HomeQuery returns the plugin's home results.
	HomeQuery(ctx context.Context) ([]Result, error)
}

// PluginI18n translates a plugin's own name and description.
type PluginI18n interface {
	// GetTranslatedPluginTitle is the plugin's name in the user's language.
	GetTranslatedPluginTitle() string
	// GetTranslatedPluginDescription is its description in that language.
	GetTranslatedPluginDescription() string
	// OnCultureInfoChanged is called when the user changes language.
	OnCultureInfoChanged(language string)
}

// ReloadablePlugin reloads what it holds, as F5 asks.
type ReloadablePlugin interface {
	// ReloadData looks the plugin's data up again.
	ReloadData(ctx context.Context) error
}

// SavablePlugin writes its settings out.
type SavablePlugin interface {
	// Save persists what the plugin holds.
	Save()
}

// SettingProviderPlugin is a plugin with settings of its own, which the
// launcher shows and changes in a window of its own: the fields of what
// Settings returns are what the window is built from.
type SettingProviderPlugin interface {
	// Settings returns the plugin's settings, which the window changes in
	// place.
	Settings() any
}

// ExternalPreviewPlugin shows a result's file in a window of its own.
type ExternalPreviewPlugin interface {
	// OpenPreview opens the file's preview.
	OpenPreview(ctx context.Context, path string, sendFailToast bool) error
	// ClosePreview closes it.
	ClosePreview(ctx context.Context) error
	// SwitchPreview switches it to another file.
	SwitchPreview(ctx context.Context, path string, sendFailToast bool) error
	// AllowAlwaysPreview reports whether it may open without being asked.
	AllowAlwaysPreview() bool
}

// PluginMetadata is what a plugin says about itself.
type PluginMetadata struct {
	ID           string
	Name         string
	Author       string
	Version      string
	Language     string
	Description  string
	Website      string
	Disabled     bool
	HomeDisabled bool

	// ExecuteFileName is the file the launcher runs for a plugin that lives
	// outside the launcher's own process.
	ExecuteFileName string
	// ExecuteFilePath is that file inside the plugin's directory.
	ExecuteFilePath string
	// PluginDirectory is where the plugin was loaded from.
	PluginDirectory string
	// IcoPath is the plugin's icon, inside PluginDirectory.
	IcoPath string

	// ActionKeyword is the plugin's first keyword.
	ActionKeyword string
	// ActionKeywords are all of them; the wildcard "*" means the plugin
	// answers every query.
	ActionKeywords []string
	// HideActionKeywordPanel hides the keyword box of the settings window.
	HideActionKeywordPanel bool

	// SearchDelayTime delays this plugin's queries, in milliseconds; nil
	// uses the user's setting.
	SearchDelayTime *int
	// Priority lifts every result of this plugin.
	Priority int

	// MinimumAppVersion is the launcher version the plugin needs.
	MinimumAppVersion string

	// PluginSettingsDirectoryPath is where the plugin keeps its settings.
	PluginSettingsDirectoryPath string
	// PluginCacheDirectoryPath is where it keeps its cache.
	PluginCacheDirectoryPath string

	// InitTime and AvgQueryTime are what the settings window shows.
	InitTime     int64
	AvgQueryTime int64
	QueryCount   int

	// Modified is set for a plugin the user changed, which the launcher
	// leaves out of queries until it is reloaded.
	Modified bool
}

// PluginPair is a plugin and what it says about itself.
type PluginPair struct {
	Plugin   Plugin
	Metadata PluginMetadata
}

// String is the plugin's name.
func (p PluginPair) String() string { return p.Metadata.Name }

// PluginInitContext is what a plugin is initialized with.
type PluginInitContext struct {
	CurrentPluginMetadata PluginMetadata
	API                   *PublicAPI
}

// PluginManager holds the plugins and asks them for results.
type PluginManager struct {
	mu sync.RWMutex

	allLoaded      map[string]PluginPair
	allInitialized map[string]PluginPair
	initFailed     map[string]PluginPair

	globalPlugins    map[string]PluginPair
	nonGlobalPlugins map[string][]PluginPair

	contextMenuPlugins    []PluginPair
	homePlugins           []PluginPair
	translationPlugins    []PluginPair
	externalPreviewPlugin []PluginPair

	// API is what plugins call back into.
	API *PublicAPI
	// Settings holds the plugins' action keywords and which are disabled.
	Settings *PluginsSettings
}

// NewPluginManager makes an empty manager.
func NewPluginManager() *PluginManager {
	return &PluginManager{
		allLoaded:        map[string]PluginPair{},
		allInitialized:   map[string]PluginPair{},
		initFailed:       map[string]PluginPair{},
		globalPlugins:    map[string]PluginPair{},
		nonGlobalPlugins: map[string][]PluginPair{},
	}
}

// Load adds a plugin to the manager, before it is initialized.
func (m *PluginManager) Load(pair PluginPair) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.allLoaded[pair.Metadata.ID]; exists {
		return
	}
	m.allLoaded[pair.Metadata.ID] = pair
}

// Initialize initializes every loaded plugin and files them by their action
// keywords. A plugin that fails is disabled and remembered, so that the
// settings window can still list it.
func (m *PluginManager) Initialize(ctx context.Context) []PluginPair {
	m.mu.RLock()
	pairs := make([]PluginPair, 0, len(m.allLoaded))
	for _, pair := range m.allLoaded {
		pairs = append(pairs, pair)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed []PluginPair

	for _, pair := range pairs {
		wg.Add(1)
		go func(pair PluginPair) {
			defer wg.Done()

			m.registerActionKeywords(pair)

			err := pair.Plugin.Init(&PluginInitContext{
				CurrentPluginMetadata: pair.Metadata,
				API:                   m.API,
			})

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if !pair.Metadata.Disabled || !pair.Metadata.HomeDisabled {
					pair.Metadata.Disabled = true
					pair.Metadata.HomeDisabled = true
				}
				m.mu.Lock()
				m.allInitialized[pair.Metadata.ID] = pair
				m.initFailed[pair.Metadata.ID] = pair
				m.mu.Unlock()
				failed = append(failed, pair)
				return
			}

			m.mu.Lock()
			m.addToLists(pair)
			m.mu.Unlock()
		}(pair)
	}
	wg.Wait()

	return failed
}

// registerActionKeywords files a plugin under each of its keywords.
func (m *PluginManager) registerActionKeywords(pair PluginPair) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, keyword := range distinctStrings(pair.Metadata.ActionKeywords) {
		if keyword == GlobalPluginWildcardSign {
			m.globalPlugins[pair.Metadata.ID] = pair
			continue
		}
		existing := m.nonGlobalPlugins[keyword]
		if !containsPair(existing, pair) {
			m.nonGlobalPlugins[keyword] = append(existing, pair)
		}
	}
}

// addToLists files a plugin under what else it can do.
func (m *PluginManager) addToLists(pair PluginPair) {
	m.allInitialized[pair.Metadata.ID] = pair

	if _, ok := pair.Plugin.(ContextMenuPlugin); ok {
		m.contextMenuPlugins = append(m.contextMenuPlugins, pair)
	}
	if _, ok := pair.Plugin.(HomePlugin); ok {
		m.homePlugins = append(m.homePlugins, pair)
	}
	if _, ok := pair.Plugin.(PluginI18n); ok {
		m.translationPlugins = append(m.translationPlugins, pair)
	}
	if _, ok := pair.Plugin.(ExternalPreviewPlugin); ok {
		m.externalPreviewPlugin = append(m.externalPreviewPlugin, pair)
	}
}

// AllLoadedPlugins returns every plugin the manager knows, initialized or
// not.
func (m *PluginManager) AllLoadedPlugins() []PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return sortedPairs(m.allLoaded)
}

// AllInitializedPlugins returns the plugins that initialized; with
// includeFailed, the ones that failed too.
func (m *PluginManager) AllInitializedPlugins(includeFailed bool) []PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]PluginPair, 0, len(m.allInitialized))
	for id, pair := range m.allInitialized {
		if !includeFailed {
			if _, failed := m.initFailed[id]; failed {
				continue
			}
		}
		out = append(out, pair)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Metadata.Name < out[j].Metadata.Name })
	return out
}

// FailedPlugins returns the plugins that failed to initialize.
func (m *PluginManager) FailedPlugins() []PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return sortedPairs(m.initFailed)
}

// GetPluginForID returns the plugin with an id, or false when there is none.
func (m *PluginManager) GetPluginForID(id string) (PluginPair, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pair, ok := m.allLoaded[id]
	return pair, ok
}

// GlobalPlugins returns the plugins that answer every query.
func (m *PluginManager) GlobalPlugins() []PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return sortedPairs(m.globalPlugins)
}

// NonGlobalPlugins returns the plugins filed under their action keywords,
// which is what the query builder looks a leading term up in.
func (m *PluginManager) NonGlobalPlugins() map[string][]PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string][]PluginPair, len(m.nonGlobalPlugins))
	for keyword, pairs := range m.nonGlobalPlugins {
		out[keyword] = append([]PluginPair(nil), pairs...)
	}
	return out
}

// HomePlugins returns the plugins that answer the home query.
func (m *PluginManager) HomePlugins() []PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return enabledPairs(m.homePlugins)
}

// TranslationPlugins returns the plugins that translate their own names.
func (m *PluginManager) TranslationPlugins() []PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return enabledPairs(m.translationPlugins)
}

// ExternalPreviewPlugins returns the plugins that preview a file in a
// window of their own.
func (m *PluginManager) ExternalPreviewPlugins() []PluginPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return enabledPairs(m.externalPreviewPlugin)
}

// IsHomePlugin reports whether a plugin answers the home query.
func (m *PluginManager) IsHomePlugin(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, pair := range m.homePlugins {
		if pair.Metadata.ID == id && !pair.Metadata.Modified {
			return true
		}
	}
	return false
}

// IsInitializing reports whether a plugin has yet to initialize.
func (m *PluginManager) IsInitializing(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, loaded := m.allLoaded[id]; !loaded {
		return false
	}
	_, done := m.allInitialized[id]
	return !done
}

// IsInitializationFailed reports whether a plugin failed to initialize.
func (m *PluginManager) IsInitializationFailed(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, loaded := m.allLoaded[id]; !loaded {
		return false
	}
	if _, done := m.allInitialized[id]; !done {
		return false
	}
	_, failed := m.initFailed[id]
	return failed
}

// IsInitializingOrFailed reports either of the two.
func (m *PluginManager) IsInitializingOrFailed(id string) bool {
	return m.IsInitializing(id) || m.IsInitializationFailed(id)
}

// ValidPluginsForQuery returns the plugins a query should go to: those filed
// under its action keyword, or every global plugin when it has none.
func (m *PluginManager) ValidPluginsForQuery(query *Query) []PluginPair {
	if query == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	pairs, ok := m.nonGlobalPlugins[query.ActionKeyword]
	if !ok {
		return enabledPairs(sortedPairs(m.globalPlugins))
	}

	out := make([]PluginPair, 0, len(pairs))
	for _, pair := range pairs {
		if pair.Metadata.Disabled || pair.Metadata.Modified {
			continue
		}
		out = append(out, pair)
	}
	return out
}

// ContextMenusForPlugin returns the menu items a plugin adds to a result.
func (m *PluginManager) ContextMenusForPlugin(result Result) []Result {
	m.mu.RLock()
	var pair PluginPair
	found := false
	for _, candidate := range m.contextMenuPlugins {
		if candidate.Metadata.ID == result.PluginID && !candidate.Metadata.Modified {
			pair, found = candidate, true
			break
		}
	}
	m.mu.RUnlock()

	if !found {
		return nil
	}

	menuPlugin, ok := pair.Plugin.(ContextMenuPlugin)
	if !ok {
		return nil
	}

	results := menuPlugin.LoadContextMenus(result)
	for i := range results {
		results[i].PluginDirectory = pair.Metadata.PluginDirectory
		results[i].PluginID = pair.Metadata.ID
		results[i].OriginQuery = result.OriginQuery
	}
	return results
}

// UpdateMetadata changes what the manager knows about a plugin, as the
// settings window does when the user gives it a priority or another
// keyword.
func (m *PluginManager) UpdateMetadata(id string, change func(*PluginMetadata)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pair, ok := m.allLoaded[id]
	if !ok {
		return
	}
	change(&pair.Metadata)
	m.allLoaded[id] = pair

	if initialized, ok := m.allInitialized[id]; ok {
		initialized.Metadata = pair.Metadata
		m.allInitialized[id] = initialized
	}
	if failed, ok := m.initFailed[id]; ok {
		failed.Metadata = pair.Metadata
		m.initFailed[id] = failed
	}
	if global, ok := m.globalPlugins[id]; ok {
		global.Metadata = pair.Metadata
		m.globalPlugins[id] = global
	}
	for keyword, pairs := range m.nonGlobalPlugins {
		for i, candidate := range pairs {
			if candidate.Metadata.ID == id {
				pairs[i].Metadata = pair.Metadata
			}
		}
		m.nonGlobalPlugins[keyword] = pairs
	}
}

// AddActionKeyword files a plugin under another keyword, as the web search
// plugin does for each of its searches.
func (m *PluginManager) AddActionKeyword(id, keyword string) {
	pair, ok := m.GetPluginForID(id)
	if !ok {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if keyword == GlobalPluginWildcardSign {
		m.globalPlugins[id] = pair
	} else if existing := m.nonGlobalPlugins[keyword]; !containsPair(existing, pair) {
		m.nonGlobalPlugins[keyword] = append(existing, pair)
	}

	if !containsString(pair.Metadata.ActionKeywords, keyword) {
		pair.Metadata.ActionKeywords = append(pair.Metadata.ActionKeywords, keyword)
	}
	if len(pair.Metadata.ActionKeywords) > 0 {
		pair.Metadata.ActionKeyword = pair.Metadata.ActionKeywords[0]
	} else {
		pair.Metadata.ActionKeyword = ""
	}
	m.allLoaded[id] = pair
}

// RemoveActionKeyword takes a plugin out from under a keyword.
func (m *PluginManager) RemoveActionKeyword(id, keyword string) {
	pair, ok := m.GetPluginForID(id)
	if !ok {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if keyword == GlobalPluginWildcardSign && countString(pair.Metadata.ActionKeywords, keyword) == 1 {
		delete(m.globalPlugins, id)
	}

	if keyword != GlobalPluginWildcardSign {
		existing := m.nonGlobalPlugins[keyword]
		kept := existing[:0]
		for _, candidate := range existing {
			if candidate.Metadata.ID != id {
				kept = append(kept, candidate)
			}
		}
		if len(kept) == 0 {
			delete(m.nonGlobalPlugins, keyword)
		} else {
			m.nonGlobalPlugins[keyword] = kept
		}
	}

	keywords := pair.Metadata.ActionKeywords[:0]
	for _, candidate := range pair.Metadata.ActionKeywords {
		if candidate != keyword {
			keywords = append(keywords, candidate)
		}
	}
	pair.Metadata.ActionKeywords = keywords
	if len(keywords) > 0 {
		pair.Metadata.ActionKeyword = keywords[0]
	} else {
		pair.Metadata.ActionKeyword = ""
	}
	m.allLoaded[id] = pair
}

// UpdatePluginMetadata stamps a plugin's results with where they came from,
// and with the query that produced them.
func UpdatePluginMetadata(results []Result, metadata PluginMetadata, query *Query) {
	for i := range results {
		results[i].PluginDirectory = metadata.PluginDirectory
		results[i].PluginID = metadata.ID
		results[i].OriginQuery = query

		// A plugin with several keywords sets the result's own keyword, so
		// the launcher only fills it in for one.
		if len(metadata.ActionKeywords) == 1 {
			results[i].ActionKeywordAssigned = query.ActionKeyword
		}
	}
}

// enabledPairs returns the pairs that are not disabled or modified.
func enabledPairs(pairs []PluginPair) []PluginPair {
	out := make([]PluginPair, 0, len(pairs))
	for _, pair := range pairs {
		if pair.Metadata.Disabled || pair.Metadata.Modified {
			continue
		}
		out = append(out, pair)
	}
	return out
}

func sortedPairs(m map[string]PluginPair) []PluginPair {
	out := make([]PluginPair, 0, len(m))
	for _, pair := range m {
		out = append(out, pair)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Metadata.Name < out[j].Metadata.Name })
	return out
}

func containsPair(pairs []PluginPair, pair PluginPair) bool {
	for _, candidate := range pairs {
		if candidate.Metadata.ID == pair.Metadata.ID {
			return true
		}
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func countString(values []string, value string) int {
	count := 0
	for _, candidate := range values {
		if candidate == value {
			count++
		}
	}
	return count
}

func distinctStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// pluginDirForName is where a plugin's data lives: its name under the
// plugin settings directory.
func pluginDirForName(root, name string) string { return filepath.Join(root, name) }

// SanitizePluginPath keeps a plugin's paths inside its own directory.
func SanitizePluginPath(dir, name string) string {
	joined := filepath.Join(dir, name)
	cleanDir := filepath.Clean(dir)
	if strings.HasPrefix(filepath.Clean(joined), cleanDir+string(filepath.Separator)) || filepath.Clean(joined) == cleanDir {
		return joined
	}
	return cleanDir
}
