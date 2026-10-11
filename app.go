package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// app is the launcher: what the user typed, the results it found, and the
// plugins it found them with.
type app struct {
	dataDir  string
	settings *core.Settings
	plugins  *core.PluginsSettings
	manager  *core.PluginManager
	engine   *core.Engine
	history  *core.History
	selected *core.UserSelectedRecord
	topMost  *core.TopMostRecord
	api      *core.PublicAPI

	win         *mygo.Window
	tray        *mygo.Tray
	settingsWin *settingsWindow

	// query is what the query box holds, and results what the plugins
	// answered for it.
	query     string
	results   []core.Result
	selection int
	list      ui.ListState
	input     ui.Handle

	// searched is the text the results belong to, so that the view searches
	// again only when the text changed.
	searched string
	// searchID counts searches, so that the results of a search the user
	// has typed past are dropped.
	searchID int
	cancel   context.CancelFunc
	// searching is true while a query is in flight.
	searching bool

	// pluginIcon is the icon of the only plugin the query goes to, which
	// the query box shows in place of the search icon.
	pluginIcon string

	// preview shows the panel beside the results.
	preview bool

	// contextMenu holds the items of the result's menu while it is open.
	contextMenu  []core.Result
	contextIndex int
	menuList     ui.ListState

	// historyIndex walks the queries the user typed before.
	historyIndex int
	historyDraft string

	// store is the list of plugins the launcher can install.
	store *core.PluginStore

	icons *iconCache

	// lastHeight is the window height the results asked for, so that the
	// window is resized only when it changes.
	lastHeight int
	// lastWidth is the width the settings ask for.
	lastWidth int

	// keys are the keys the launcher answers to, read from the settings
	// when they change.
	keys hotkeys
}

// newApp makes the launcher and loads everything it needs: the settings,
// the plugins and what the user has opened before.
func newApp(dataDir string) (*app, error) {
	settings := core.LoadSettings(core.SettingsPath(dataDir))
	core.SetLanguage(settings.Language)

	pluginsSettings := core.LoadPluginsSettings(filepath.Join(dataDir, "Settings", "Plugins.json"))

	a := &app{
		dataDir:  dataDir,
		settings: settings,
		plugins:  pluginsSettings,
		history:  core.LoadHistory(filepath.Join(dataDir, "Settings", "History.json")),
		selected: core.LoadUserSelectedRecord(filepath.Join(dataDir, "Settings", "UserSelectedRecord.json")),
		topMost:  core.LoadTopMostRecord(filepath.Join(dataDir, "Settings", "MultipleTopMostRecord.json")),
		icons:    newIconCache(),
	}

	a.manager = core.NewPluginManager()
	a.manager.Settings = pluginsSettings
	a.api = core.NewPublicAPI(a.manager, dataDir, version)
	a.manager.API = a.api
	a.api.SetMatcher(matcherFor(settings))

	a.engine = core.NewEngine(a.manager, settings)
	a.engine.History = a.history
	a.engine.Selected = a.selected
	a.engine.TopMost = a.topMost
	a.engine.SearchDelayTime = 0
	if settings.SearchQueryResultsWithDelay {
		a.engine.SearchDelayTime = time.Duration(settings.SearchDelayTime) * time.Millisecond
	}
	a.engine.CustomShortcuts = settings.CustomShortcuts
	a.engine.BuiltinShortcuts = map[string]func() string{
		"{clipboard}": func() string { return mygo.Clipboard.ReadText() },
	}
	a.keys = hotkeysFrom(settings)
	a.api.Logger.SetLevel(settings.LogLevel)

	a.api.Hooks = a.hooks()
	// A plugin written in another language calls the launcher through the
	// same API its host uses.
	core.SetLauncher(a.api)

	// The plugin store is read through the launcher's own client, so that
	// the user's proxy setting is used.
	a.store = core.NewPluginStore(version, core.StoreCachePath(dataDir))
	a.store.Client = a.api.HTTPClient
	core.SetHistoryAction(func(query string) {
		a.api.BackToQueryResults()
		a.setQuery(query)
	})

	loadPlugins(a)
	return a, nil
}

// matcherFor is how the launcher matches a query against what it is
// compared with: the precision the user chose, whether accents are folded
// away, and, when the user asked for it, the pinyin a name written in
// Chinese can be found by.
func matcherFor(settings *core.Settings) core.StringMatcher {
	matcher := core.StringMatcher{
		UserSettingSearchPrecision: settings.QuerySearchPrecision,
		IgnoreAccents:              settings.IgnoreAccents,
	}
	if settings.ShouldUsePinyin {
		matcher.Alphabet = &core.PinyinAlphabet{
			UseDoublePinyin:              settings.UseDoublePinyin,
			Schema:                       settings.DoublePinyinSchema,
			UsePolyphonicPhraseOverrides: settings.UsePolyphonicPhraseOverrides,
		}
	}
	return matcher
}

// showShellContextMenu shows the machine's own context menu for a path, at
// the place the mouse is, from the launcher's own window: the menu belongs
// to a window, and the shell's objects belong to the thread that made them.
func (a *app) showShellContextMenu(path string, extendedVerbs bool) bool {
	if path == "" {
		return false
	}
	owner := uintptr(0)
	if a.win != nil {
		owner = a.win.NativeHandle()
	}
	// The menu belongs to a window, and the shell's objects belong to the
	// thread that made them, so it is shown on the thread the launcher's
	// window lives on rather than on one of its own.
	mygo.RunOnMain(func() {
		if err := core.ShowShellContextMenu(path, core.ShellContextMenuPointAtCursor(), owner, extendedVerbs); err != nil {
			a.api.LogError("launcher", "showing the machine's own menu failed: "+err.Error())
		}
	})
	return true
}

// hooks are what the launcher does for a plugin.
func (a *app) hooks() core.Hooks {
	return core.Hooks{
		ChangeQuery:         func(query string, requery bool) { a.setQuery(query) },
		ReQuery:             func(reselect bool) { a.requery() },
		BackToQueryResults:  func() { a.closeContextMenu() },
		ShowMainWindow:      func() { a.show() },
		HideMainWindow:      func() { a.hide() },
		FocusQueryTextBox:   func() { a.focusQuery() },
		IsMainWindowVisible: func() bool { return a.win != nil && a.win.IsVisible() },
		OpenSettingDialog:   func() { openSettingsWindow(a) },
		OpenPluginSettings:  func(pluginID string) bool { return openPluginSettingsWindow(a, pluginID) },
		SaveAppAllSettings:  func() { a.saveSettings() },
		SavePluginSettings:  func() { a.plugins.Save() },
		ReloadAllPluginData: a.reloadPluginData,
		CopyToClipboard:     func(text string, directCopy, notify bool) { a.copyToClipboard(text, directCopy, notify) },
		ShowMsg:             func(title, subTitle, icon string) { a.notify(title, subTitle) },
		ShowMsgError:        func(title, subTitle string) { a.notify(title, subTitle) },
		OpenDirectory:       openDirectory,
		ShowShellContextMenu: func(path string, extendedVerbs bool) bool {
			return a.showShellContextMenu(path, extendedVerbs)
		},
		OpenURL:           openURL,
		OpenAppURI:        func(uri string) { openURL(uri, nil) },
		ShellRun:          shellRun,
		ToggleGameMode:    func() { a.setGameMode(!a.gameMode()) },
		SetGameMode:       a.setGameMode,
		IsGameModeOn:      a.gameMode,
		IsDarkTheme:       func() bool { return isDarkTheme() },
		UpdateManifest:    a.updatePluginManifest,
		GetPluginManifest: a.pluginManifest,
		InstallPlugin:     a.installPlugin,
		UninstallPlugin:   a.uninstallPlugin,
		LogDirectory:      func() string { return filepath.Join(a.dataDir, "Logs") },
		Exit:              func() { mygo.App.Quit() },
		SetOpenAtLogin:    a.setOpenAtLogin,
		ShowMsgBox:        a.showMessageBox,
	}
}

// view builds the launcher window.
func (a *app) view(c *ui.Context) {
	a.applyTheme(c)
	a.searchIfChanged()
	a.resizeWindow(c)

	t := c.Theme()
	root := ui.Column(c).Fill().Clip()
	if !c.Vibrancy() {
		root.Background(t.Background)
	}

	root.Children(func() {
		a.queryBox(c)
		// The menu of a result takes the place of the results while it is
		// open, which is how the reference shows it.
		if len(a.contextMenu) > 0 {
			a.contextMenuView(c)
			return
		}
		if len(a.results) > 0 {
			if a.previewOn() {
				// The panel takes the right of the window, and the results
				// what is left of it.
				width, _ := c.Size()
				resultsWidth := width - previewWidth
				if resultsWidth < 240 {
					resultsWidth = 240
				}
				ui.Split(c, &resultsWidth, func() { a.resultsView(c) }, func() { a.previewView(c) }).Grow(1)
			} else {
				a.resultsView(c)
			}
		}
	})
}

// applySettings tells the launcher about settings the user just changed.
func (a *app) applySettings() {
	a.engine.SearchDelayTime = 0
	if a.settings.SearchQueryResultsWithDelay {
		a.engine.SearchDelayTime = time.Duration(a.settings.SearchDelayTime) * time.Millisecond
	}
	a.engine.CustomShortcuts = a.settings.CustomShortcuts
	a.api.SetMatcher(matcherFor(a.settings))
	a.api.Logger.SetLevel(a.settings.LogLevel)
	a.keys = hotkeysFrom(a.settings)
	a.lastWidth, a.lastHeight = 0, 0
	a.requery()
}

// installPlugin puts a plugin of the store into the launcher's plugins
// directory, where it is read at the next start.
func (a *app) installPlugin(plugin core.UserPlugin, zipFilePath string) bool {
	directory := filepath.Join(a.dataDir, "Plugins")
	if err := core.InstallPluginFromStore(context.Background(), a.api, directory, plugin); err != nil {
		a.api.LogError("launcher", "installing "+plugin.Name+" failed: "+err.Error())
		return false
	}
	return true
}

// uninstallPlugin takes a plugin out of the launcher's plugins directory.
// A plugin the launcher was built with cannot be removed, since it is not
// there to remove.
func (a *app) uninstallPlugin(id string, removeSettings bool) error {
	pair, ok := a.manager.GetPluginForID(id)
	if !ok {
		return fmt.Errorf("there is no plugin with the id %s", id)
	}

	directory := pair.Metadata.PluginDirectory
	if directory == "" {
		return fmt.Errorf("%s is part of the launcher and cannot be removed", pair.Metadata.Name)
	}
	if !core.DirectoryExists(directory) {
		return fmt.Errorf("%s was not installed from a folder", pair.Metadata.Name)
	}

	// The settings are kept unless the user asked for them to go too.
	if removeSettings {
		_ = os.RemoveAll(filepath.Join(a.api.PluginSettingsDir, pair.Metadata.Name))
	}
	if err := os.RemoveAll(directory); err != nil {
		return err
	}

	a.plugins.RemovePluginSettings(id)
	_ = a.plugins.Save()
	a.api.LogInfo("launcher", "removed "+pair.Metadata.Name)
	return nil
}

// pluginManifest is the list of plugins the launcher can install.
func (a *app) pluginManifest() []core.UserPlugin {
	if a.store == nil {
		return nil
	}
	plugins, err := a.store.Plugins(context.Background(), false)
	if err != nil {
		a.api.LogError("launcher", "reading the plugin store failed: "+err.Error())
		return nil
	}
	return plugins
}

// applyTheme paints the theme the user chose onto the widgets, which the
// launcher's own "System" theme leaves as the desktop's.
func (a *app) applyTheme(c *ui.Context) {
	if a.settings.Theme == core.DefaultThemeName {
		return
	}
	reference, ok := core.FindTheme(a.settings.Theme)
	if !ok {
		return
	}

	theme := *c.Theme()
	core.PaintTheme(&theme, reference)
	c.SetTheme(&theme)
}

// updatePluginManifest reads the plugin store, and keeps what it read so
// that the plugin store is not empty offline.
func (a *app) updatePluginManifest(force bool) error {
	if a.store == nil {
		return nil
	}
	_, err := a.store.Plugins(context.Background(), force)
	return err
}

// searchIfChanged searches when the query box no longer matches the results.
func (a *app) searchIfChanged() {
	if a.query == a.searched {
		return
	}
	a.search(a.query)
}

// search runs a query and shows what the plugins answer as they answer.
func (a *app) search(text string) {
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	a.searched = text
	a.searchID++
	id := a.searchID
	a.historyIndex = -1

	engine := a.engine
	go func() {
		engine.SearchStreaming(ctx, text, func(batch core.ResultBatch) {
			a.update(func() {
				if id != a.searchID {
					return
				}
				a.results = batch.Results
				a.clampSelection()
			})
		})
	}()
}

// requery searches the same text again, as Ctrl+R asks.
func (a *app) requery() {
	a.searched = ""
	a.search(a.query)
}

// setQuery puts text in the query box and searches for it.
func (a *app) setQuery(text string) {
	a.query = text
	a.search(text)
	a.focusQuery()
}

// clampSelection keeps the choice on a result that is there.
func (a *app) clampSelection() {
	if len(a.results) == 0 {
		a.selection = 0
		return
	}
	if a.selection >= len(a.results) {
		a.selection = 0
	}
	if a.selection < 0 {
		a.selection = len(a.results) - 1
	}
}

// update runs a function on the main thread and draws a frame.
func (a *app) update(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

// selectedResult is the result the user is on.
func (a *app) selectedResult() (core.Result, bool) {
	if a.selection < 0 || a.selection >= len(a.results) {
		return core.Result{}, false
	}
	return a.results[a.selection], true
}

// runResult runs a result's action, remembering it for the next time.
func (a *app) runResult(index int, state core.SpecialKeyState) {
	if index < 0 || index >= len(a.results) {
		return
	}
	result := a.results[index]
	a.runAction(result, state)
}

// runAction runs one action of a result.
func (a *app) runAction(result core.Result, state core.SpecialKeyState) {
	a.selected.Add(result)
	a.history.Add(result)

	hide := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				a.api.LogError("launcher", fmt.Sprintf("the action of %q panicked: %v", result.Title, r))
			}
		}()
		hide = result.Execute(context.Background(), core.ActionContext{SpecialKeyState: state})
	}()

	a.saveHistory()
	if hide {
		a.hide()
	}
}

// saveHistory writes what the user opened out.
func (a *app) saveHistory() {
	if err := a.history.Save(); err != nil {
		a.api.LogError("launcher", "saving the history failed: "+err.Error())
	}
	if err := a.selected.Save(); err != nil {
		a.api.LogError("launcher", "saving what the user picked failed: "+err.Error())
	}
}

// saveSettings writes the launcher's settings out.
func (a *app) saveSettings() {
	if err := a.settings.Save(); err != nil {
		a.api.LogError("launcher", "saving the settings failed: "+err.Error())
	}
	if err := a.plugins.Save(); err != nil {
		a.api.LogError("launcher", "saving the plugin settings failed: "+err.Error())
	}
}

// reloadPluginData asks every plugin to look its data up again, as F5 does.
func (a *app) reloadPluginData() error {
	var firstError error
	for _, pair := range a.manager.AllInitializedPlugins(false) {
		reloadable, ok := pair.Plugin.(core.ReloadablePlugin)
		if !ok {
			continue
		}
		if err := reloadable.ReloadData(context.Background()); err != nil && firstError == nil {
			firstError = err
		}
	}
	a.requery()
	return firstError
}

// iconCache turns the icons of results into bitmaps, once each.
//
// Reading an icon can take a tenth of a second: a shortcut has to be
// resolved before the shell will say what its picture is. That must not
// happen while the window is being drawn, so an icon that is not known yet
// is read in the background and the row is drawn without it until it is
// there.
type iconCache struct {
	mu      sync.Mutex
	bitmaps map[string]*ui.Bitmap
	// loading are the paths being read right now, so that a path is read
	// once however many rows ask for it.
	loading map[string]bool
	// ready is called when an icon has been read, so that the window can
	// draw a frame with it.
	ready func()
}

func newIconCache() *iconCache {
	return &iconCache{
		bitmaps: map[string]*ui.Bitmap{},
		loading: map[string]bool{},
	}
}

// bitmap is the bitmap of an icon path, or nil when it has none or has not
// been read yet.
func (c *iconCache) bitmap(path string) *ui.Bitmap {
	if path == "" {
		return nil
	}

	c.mu.Lock()
	if bitmap, ok := c.bitmaps[path]; ok {
		c.mu.Unlock()
		return bitmap
	}
	if c.loading[path] {
		c.mu.Unlock()
		return nil
	}
	c.loading[path] = true
	ready := c.ready
	c.mu.Unlock()

	go func() {
		content := core.IconBytes(path)

		var bitmap *ui.Bitmap
		if len(content) > 0 {
			if decoded, err := ui.DecodeBitmap(content); err == nil {
				bitmap = decoded
			}
		}

		c.mu.Lock()
		c.bitmaps[path] = bitmap
		delete(c.loading, path)
		c.mu.Unlock()

		// The window draws a frame with the icon in it.
		if ready != nil {
			ready()
		}
	}()

	return nil
}

// bitmapFor loads the bitmap of an icon path that a plugin gave, resolving
// it against the plugin's directory.
func (a *app) bitmapFor(path string) *ui.Bitmap {
	return a.icons.bitmap(a.resolveIconPath(path))
}

// resolveIconPath finds the file an icon path names, which a plugin may give
// relative to its own directory, as an absolute path or as a URL.
func (a *app) resolveIconPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(a.dataDir, "Plugins", path)
}

// imageBitmap is the bitmap of a picture a plugin drew.
func (a *app) imageBitmap(image image.Image) *ui.Bitmap {
	if image == nil {
		return nil
	}
	return ui.NewBitmap(image)
}

// dataDirectory is where the launcher keeps its settings and plugins.
func dataDirectory() (string, error) {
	if override := os.Getenv("LAUNCH_DATA_DIR"); override != "" {
		return override, nil
	}
	return mygo.App.Path(mygo.PathUserData)
}
