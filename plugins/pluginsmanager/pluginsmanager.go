package pluginsmanager

import (
	"context"
	"embed"
	"path/filepath"
	"sort"
	"strings"

	"launch/core"
)

// The plugins manager's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// The words that choose what the plugin does, as the reference has them.
const (
	installCommand   = "install"
	uninstallCommand = "uninstall"
	updateCommand    = "update"
)

// Settings is what the user can change about installing plugins.
type Settings struct {
	// WarnFromUnknownSource asks before installing a plugin from
	// somewhere the launcher does not know.
	WarnFromUnknownSource bool `json:"WarnFromUnknownSource"`
	// AutoRestartAfterChanging starts the launcher again after a plugin is
	// installed, removed or updated, since a plugin is only loaded as the
	// launcher starts.
	AutoRestartAfterChanging bool `json:"AutoRestartAfterChanging"`
}

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{WarnFromUnknownSource: true}
}

// Plugin is the plugins manager: it installs, removes and updates the
// plugins of the launcher's own store.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	settings Settings
	words    *core.PluginTranslator
	// downloader fetches a plugin's file from the store, kept behind a
	// field so that a test does not reach the network.
	downloader Downloader
}

// Downloader fetches a plugin's file from where the store keeps it.
type Downloader interface {
	// Download writes what a URL answers with to a path.
	Download(ctx context.Context, url, path string) error
}

// httpDownloader fetches a file through the launcher's own HTTP client,
// which carries the user's proxy setting.
type httpDownloader struct{ api *core.PublicAPI }

// Download writes what a URL answers with to a path.
func (d httpDownloader) Download(ctx context.Context, url, path string) error {
	return d.api.HttpDownloadAsync(ctx, url, path, nil)
}

// New makes the plugins manager.
func New() *Plugin {
	return &Plugin{BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
		ID:             "9f8f9b14-2518-4907-b211-35ab6290dee7",
		Name:           "Plugins Manager",
		Description:    "Install, uninstall or update Flow Launcher plugins via the search window",
		Author:         "Jeremy Wu",
		Version:        "1.0.0",
		Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
		ActionKeywords: []string{"pm"},
		IcoPath:        "Images/pluginsmanager.png",
	}}, downloader: nil}
}

// Init reads the settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")
	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "Plugins Manager", "Settings.json")
	p.downloader = httpDownloader{api: ctx.API}
	return nil
}

// Settings hands the plugin's settings over, which the launcher shows and
// changes in a window of its own.
func (p *Plugin) Settings() any { return &p.settings }

// Save writes the settings out.
func (p *Plugin) Save() {
	_ = core.SaveSettingJsonStorage(p.api, "Plugins Manager", "Settings.json", p.settings)
}

// Query answers what the user typed: the three things the plugin does when
// nothing else is typed, and what each of them then answers with.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	search := strings.TrimSpace(query.Search)
	if search == "" {
		return p.hotkeys(), nil
	}

	switch strings.ToLower(query.FirstSearch()) {
	case installCommand:
		return p.installResults(ctx, query.SecondToEndSearch())
	case uninstallCommand:
		return p.uninstallResults(query.SecondToEndSearch()), nil
	case updateCommand:
		return p.updateResults(ctx, query.SecondToEndSearch())
	default:
		return p.matchingHotkeys(search), nil
	}
}

// hotkeys are the three things the plugin does, offered by name.
func (p *Plugin) hotkeys() []core.Result {
	keyword := p.Meta.ActionKeywords[0]

	results := make([]core.Result, 0, 3)
	for _, command := range []string{installCommand, uninstallCommand, updateCommand} {
		name := command
		result := core.NewResult(name, p.words.T("plugin_pluginsmanager_"+command+"_title"), "Images/pluginsmanager.png", 0)
		result.AutoCompleteText = keyword + core.TermSeparator + command + core.TermSeparator
		result.Action = func(core.ActionContext) bool {
			// Choosing one puts it in the query box, so that the user
			// carries on typing what to install.
			p.api.ChangeQuery(keyword+core.TermSeparator+name+core.TermSeparator, false)
			return false
		}
		results = append(results, result)
	}
	return results
}

// matchingHotkeys are the three, found by what the user typed.
func (p *Plugin) matchingHotkeys(search string) []core.Result {
	matcher := p.api.Matcher()

	var results []core.Result
	for _, result := range p.hotkeys() {
		match := matcher.FuzzyMatch(search, result.Title)
		if !match.IsSearchPrecisionScoreMet() {
			continue
		}
		result.Score = match.Score()
		result.TitleHighlightData = match.MatchData
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results
}

// installResults are the plugins of the store that are not installed yet.
func (p *Plugin) installResults(ctx context.Context, search string) ([]core.Result, error) {
	// A plugin is also installed from a file the user has, or from a link
	// to one.
	if isZipPath(search) {
		return p.installFromFile(search), nil
	}
	if isZipLink(search) {
		return p.installFromLink(search), nil
	}

	if err := p.api.UpdatePluginManifest(false); err != nil {
		p.api.LogError("PluginsManager", "reading the plugin store failed: "+err.Error())
	}

	matcher := p.api.Matcher()
	installed := p.installedIDs()

	var results []core.Result
	for _, plugin := range p.api.GetPluginManifest() {
		if installed[plugin.ID] || p.api.PluginModified(plugin.ID) {
			continue
		}

		title := plugin.Name + " " + core.T("", "author") + " " + plugin.Author
		score := 0
		var highlight []int
		if search != "" {
			match := matcher.FuzzyMatch(search, plugin.Name)
			if !match.IsSearchPrecisionScoreMet() {
				match = matcher.FuzzyMatch(search, plugin.Description)
			}
			if !match.IsSearchPrecisionScoreMet() {
				continue
			}
			score = match.Score()
			highlight = match.MatchData
		}

		result := p.storeResult(plugin, title, score, highlight)
		result.Action = func(action core.ActionContext) bool {
			if action.SpecialKeyState.CtrlPressed {
				// Ctrl+Enter opens the plugin's own page instead.
				p.api.OpenUrl(plugin.Website, nil)
				return true
			}
			p.install(plugin)
			return true
		}
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results, nil
}

// uninstallResults are the plugins the launcher has.
func (p *Plugin) uninstallResults(search string) []core.Result {
	matcher := p.api.Matcher()

	var results []core.Result
	for _, pair := range p.api.GetAllPlugins() {
		metadata := pair.Metadata

		score := 0
		var highlight []int
		if search != "" {
			match := matcher.FuzzyMatch(search, metadata.Name)
			if !match.IsSearchPrecisionScoreMet() {
				match = matcher.FuzzyMatch(search, metadata.ID)
			}
			if !match.IsSearchPrecisionScoreMet() {
				continue
			}
			score = match.Score()
			highlight = match.MatchData
		}

		id := metadata.ID
		result := core.NewResult(metadata.Name, metadata.Description, metadata.IcoPath, score)
		result.PluginDirectory = metadata.PluginDirectory
		result.TitleHighlightData = highlight
		result.ContextData = metadata
		result.Action = func(core.ActionContext) bool {
			p.uninstall(id)
			return true
		}
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results
}

// updateResults are the plugins the store has a newer version of.
func (p *Plugin) updateResults(ctx context.Context, search string) ([]core.Result, error) {
	if err := p.api.UpdatePluginManifest(false); err != nil {
		p.api.LogError("PluginsManager", "reading the plugin store failed: "+err.Error())
	}

	matcher := p.api.Matcher()
	installed := map[string]string{}
	for _, pair := range p.api.GetAllPlugins() {
		installed[pair.Metadata.ID] = pair.Metadata.Version
	}

	var results []core.Result
	for _, plugin := range p.api.GetPluginManifest() {
		version, isInstalled := installed[plugin.ID]
		if !isInstalled || !isNewer(plugin.Version, version) || p.api.PluginModified(plugin.ID) {
			continue
		}

		score := 0
		var highlight []int
		if search != "" {
			match := matcher.FuzzyMatch(search, plugin.Name)
			if !match.IsSearchPrecisionScoreMet() {
				continue
			}
			score = match.Score()
			highlight = match.MatchData
		}

		title := plugin.Name + " " + version + " → " + plugin.Version
		result := p.storeResult(plugin, title, score, highlight)
		result.Action = func(core.ActionContext) bool {
			p.update(plugin)
			return true
		}
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results, nil
}

// storeResult is one plugin of the store as a result.
func (p *Plugin) storeResult(plugin core.UserPlugin, title string, score int, highlight []int) core.Result {
	icon := plugin.IcoPath
	if icon == "" {
		icon = "Images/pluginsmanager.png"
	}
	result := core.NewResult(title, plugin.Description, icon, score)
	result.TitleHighlightData = highlight
	result.SetCopyText(plugin.Name)
	result.ContextData = plugin
	return result
}

// installedIDs are the plugins the launcher has.
func (p *Plugin) installedIDs() map[string]bool {
	installed := map[string]bool{}
	for _, pair := range p.api.GetAllPlugins() {
		installed[pair.Metadata.ID] = true
	}
	return installed
}

// install adds a plugin of the store, and says what came of it.
func (p *Plugin) install(plugin core.UserPlugin) {
	p.api.HideMainWindow()
	p.api.StartLoadingBar()

	zip, err := p.download(plugin)
	if err != nil || !p.api.InstallPlugin(plugin, zip) {
		p.api.StopLoadingBar()
		p.api.ShowMsgError(p.words.T("plugin_pluginsmanager_install_error_title"),
			p.words.Tf("plugin_pluginsmanager_install_error_subtitle", plugin.Name))
		return
	}

	p.api.StopLoadingBar()
	p.afterChange(p.successMessage(plugin.Name))
}

// uninstall removes a plugin, and says what came of it.
func (p *Plugin) uninstall(id string) {
	p.api.HideMainWindow()

	if err := p.api.UninstallPlugin(id, false); err != nil {
		p.api.ShowMsgError(p.words.T("plugin_pluginsmanager_uninstall_error_title"),
			p.words.Tf("plugin_pluginsmanager_uninstall_error_title", id))
		return
	}
	p.afterChange(p.uninstalledMessage(id))
}

// update replaces a plugin with the newer version of the store.
func (p *Plugin) update(plugin core.UserPlugin) {
	p.api.HideMainWindow()

	zip, err := p.download(plugin)
	if err != nil || p.api.UpdatePlugin(plugin.ID, plugin, zip) != nil {
		p.api.ShowMsgError(p.words.T("plugin_pluginsmanager_update_failed_title"),
			p.words.Tf("plugin_pluginsmanager_update_failed_subtitle", plugin.Name))
		return
	}
	p.afterChange(p.updatedMessage(plugin.Name))
}

// download fetches a plugin's file from where the store keeps it.
func (p *Plugin) download(plugin core.UserPlugin) (string, error) {
	if plugin.UrlDownload == "" {
		return "", errNoDownload
	}

	if p.downloader == nil {
		return "", errNoDownload
	}

	directory := filepath.Join(p.api.PluginCacheDir, "Downloads")
	path := filepath.Join(directory, plugin.ID+".zip")
	if err := p.downloader.Download(context.Background(), plugin.UrlDownload, path); err != nil {
		return "", err
	}
	return path, nil
}

// successMessage is what is said after a plugin is installed.
func (p *Plugin) successMessage(name string) string {
	if p.settings.AutoRestartAfterChanging {
		return p.words.Tf("plugin_pluginsmanager_install_success_restart", name)
	}
	return p.words.Tf("plugin_pluginsmanager_install_success_no_restart", name)
}

// uninstalledMessage is what is said after a plugin is removed.
func (p *Plugin) uninstalledMessage(name string) string {
	return p.words.Tf("plugin_pluginsmanager_uninstall_success_no_restart", name)
}

// updatedMessage is what is said after a plugin is updated.
func (p *Plugin) updatedMessage(name string) string {
	if p.settings.AutoRestartAfterChanging {
		return p.words.Tf("plugin_pluginsmanager_update_success_restart", name)
	}
	return p.words.Tf("plugin_pluginsmanager_update_success_no_restart", name)
}

// afterChange says the plugin was changed, and starts the launcher again
// when the user asked for that.
func (p *Plugin) afterChange(message string) {
	p.api.ShowMsg(p.words.T("plugin_pluginsmanager_plugin_name"), message, "")
	if p.settings.AutoRestartAfterChanging {
		p.api.RestartApp()
	}
}

// installFromFile is a plugin the user has the file of.
func (p *Plugin) installFromFile(path string) []core.Result {
	result := core.NewResult(p.words.Tf("plugin_pluginsmanager_install_from_web", filepath.Base(path)),
		path, "Images/pluginsmanager.png", 0)
	result.Action = func(core.ActionContext) bool {
		p.installFromZip(path, path)
		return true
	}
	return []core.Result{result}
}

// installFromLink is a plugin the user has a link to.
func (p *Plugin) installFromLink(address string) []core.Result {
	result := core.NewResult(p.words.T("plugin_pluginsmanager_install_from_web"),
		address, "Images/pluginsmanager.png", 0)
	result.Action = func(core.ActionContext) bool {
		p.api.HideMainWindow()
		p.api.StartLoadingBar()

		directory := filepath.Join(p.api.PluginCacheDir, "Downloads")
		path := filepath.Join(directory, "plugin.zip")
		if p.downloader == nil || p.downloader.Download(context.Background(), address, path) != nil {
			p.api.StopLoadingBar()
			p.api.ShowMsgError(p.words.T("plugin_pluginsmanager_install_error_title"),
				p.words.Tf("plugin_pluginsmanager_install_error_subtitle", address))
			return true
		}
		p.api.StopLoadingBar()
		p.installFromZip(path, address)
		return true
	}
	return []core.Result{result}
}

// installFromZip installs the plugin a zip file holds, saying where it came
// from when it cannot be read.
func (p *Plugin) installFromZip(path, shownAs string) {
	plugin, err := readPluginFile(path)
	if err != nil {
		p.api.ShowMsgError(p.words.T("plugin_pluginsmanager_invalid_zip_title"),
			p.words.Tf("plugin_pluginsmanager_invalid_zip_subtitle", shownAs))
		return
	}
	plugin.LocalInstallPath = path

	p.api.HideMainWindow()
	if !p.api.InstallPlugin(plugin, path) {
		p.api.ShowMsgError(p.words.T("plugin_pluginsmanager_install_error_title"),
			p.words.Tf("plugin_pluginsmanager_install_error_subtitle", plugin.Name))
		return
	}
	p.afterChange(p.successMessage(plugin.Name))
}

// isZipPath reports whether what was typed is a file the user has.
func isZipPath(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".zip") {
		return false
	}
	return core.FileExists(path)
}

// isZipLink reports whether what was typed is a link to a file.
func isZipLink(value string) bool {
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	return strings.HasSuffix(lower, ".zip")
}

// isNewer reports whether a version of the store is newer than the one the
// launcher has. Versions are compared part by part, so that 1.10 is newer
// than 1.9.
func isNewer(candidate, current string) bool {
	candidateParts := versionParts(candidate)
	currentParts := versionParts(current)

	for i := 0; i < len(candidateParts) || i < len(currentParts); i++ {
		var a, b int
		if i < len(candidateParts) {
			a = candidateParts[i]
		}
		if i < len(currentParts) {
			b = currentParts[i]
		}
		if a != b {
			return a > b
		}
	}
	return false
}

// versionParts are the numbers of a version.
func versionParts(version string) []int {
	var parts []int
	for _, part := range strings.Split(version, ".") {
		number := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			number = number*10 + int(r-'0')
		}
		parts = append(parts, number)
	}
	return parts
}

// errNoDownload is what a plugin of the store without a file to download
// answers.
var errNoDownload = errorString("the plugin has no file to download")

type errorString string

func (e errorString) Error() string { return string(e) }
