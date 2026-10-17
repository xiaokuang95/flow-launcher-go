package copyq

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"launch/core"
)

// The pictures this plugin shows its results with, which the launcher
// reads from the plugin's own directory.
//
//go:embed Images/*.png
var images embed.FS

// The plugin's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about the clipboard history.
type Settings struct {
	// ActionKeyword is what the history answers to.
	ActionKeyword string `json:"ActionKeyword"`
	// Hotkey shows the launcher with the history at one keystroke, as
	// CopyQ's own hotkey does. Empty binds nothing.
	Hotkey string `json:"Hotkey"`
	// MaxItems is how many items are kept, the pinned ones not counted.
	MaxItems int `json:"MaxItems"`
	// WatchText, WatchFiles and WatchImages say which of the things the
	// clipboard can hold are kept, as CopyQ's own settings say which
	// formats it watches.
	WatchText   bool `json:"WatchText"`
	WatchFiles  bool `json:"WatchFiles"`
	WatchImages bool `json:"WatchImages"`
	// PasteAfterChoosing puts the item into the window the user came from
	// as soon as it is chosen, which is what CopyQ does by default.
	PasteAfterChoosing bool `json:"PasteAfterChoosing"`
	// IgnoreShortText ignores text shorter than this many characters, so
	// that a single letter copied by accident does not fill the history.
	IgnoreShortText int `json:"IgnoreShortText"`
	// IgnoreShortcut is the word the launcher's own shortcut is written
	// with, which is left out of the history.
	IgnoreShortcut string `json:"IgnoreShortcut"`
	// Version is which build's settings the file holds, so that a file
	// written before a setting existed can be filled in.
	Version int `json:"Version"`
}

// applyDefaults fills in what a settings file written by an older build
// does not say.
func (s *Settings) applyDefaults() {
	defaults := DefaultSettings()

	if s.ActionKeyword == "" {
		s.ActionKeyword = defaults.ActionKeyword
	}
	if s.MaxItems <= 0 {
		s.MaxItems = defaults.MaxItems
	}
	if s.Hotkey == "" {
		s.Hotkey = defaults.Hotkey
	}
	if s.IgnoreShortcut == "" {
		s.IgnoreShortcut = defaults.IgnoreShortcut
	}
	// The switches cannot tell "off" from "not written", so a file that
	// was written before there were any is the one that does not say. It
	// is written from now on, which is what Version is for.
	if s.Version < 2 {
		s.WatchText = defaults.WatchText
		s.WatchFiles = defaults.WatchFiles
		s.WatchImages = defaults.WatchImages
		s.PasteAfterChoosing = defaults.PasteAfterChoosing
	}
	s.Version = settingsVersion
}

// settingsVersion is what the settings a file holds are as they are now,
// which says whether a file written by an older build needs filling in.
const settingsVersion = 2

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{
		ActionKeyword:      "cb",
		Hotkey:             "Alt+C",
		MaxItems:           defaultLimit,
		WatchText:          true,
		WatchFiles:         true,
		WatchImages:        true,
		PasteAfterChoosing: true,
		IgnoreShortText:    0,
		IgnoreShortcut:     "{clipboard}",
		Version:            settingsVersion,
	}
}

// Clipboard is what the plugin reads from the machine and does to it. It is
// kept behind a field so that a test watches what the plugin asked for
// rather than reading and writing the machine's own clipboard.
type Clipboard interface {
	// Read returns what the clipboard holds now, or nothing when it holds
	// something the plugin does not keep.
	Read() (core.ClipboardContent, error)
	// Write puts an item back on the clipboard.
	Write(item Item) error
}

// systemClipboard is the machine's own clipboard, through the launcher.
type systemClipboard struct{ api *core.PublicAPI }

func (c systemClipboard) Read() (core.ClipboardContent, error) { return c.api.ReadClipboard() }

func (c systemClipboard) Write(item Item) error {
	switch item.Kind {
	case KindFiles:
		// The launcher copies the first file as a file, which is what a
		// file manager pastes; the rest of a multiple selection are the
		// paths of it.
		if len(item.Files) > 1 {
			c.api.CopyToClipboard(strings.Join(item.Files, "\n"), false, false)
			return nil
		}
		if len(item.Files) == 1 {
			c.api.CopyToClipboard(item.Files[0], true, false)
		}
		return nil
	default:
		c.api.CopyToClipboard(item.Text, false, false)
		return nil
	}
}

// watchAge is how often the clipboard is looked at. The desktop does not
// tell a launcher when the clipboard changes, and looking at it is cheap.
const watchAge = 400 * time.Millisecond

// Plugin is the clipboard history: what the clipboard held, newest first,
// searchable and kept between runs.
type Plugin struct {
	core.BasePlugin

	api       *core.PublicAPI
	words     *core.PluginTranslator
	settings  Settings
	clipboard Clipboard
	// cacheDir is where the history and the pictures it holds are kept.
	cacheDir string

	mu      sync.Mutex
	history *History
	// watching is true while the clipboard is being looked at, so that it
	// is looked at by one goroutine only.
	watching bool
	stop     chan struct{}

	// now is the clock, which a test sets so that what an item says about
	// when it was copied can be checked.
	now func() time.Time
}

// New makes the clipboard history.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "a8e6b0e4c1d94c9f8d3f5a2b7c0e6d41",
			Name:           "CopyQ",
			Description:    "The clipboard history, as CopyQ keeps it",
			Author:         "hluk",
			Version:        "1.0.0",
			Website:        "https://github.com/hluk/copyq",
			ActionKeywords: []string{"cb"},
			IcoPath:        "Images/copyq.png",
		}},
		// The settings a user who changed nothing gets, so that a plugin
		// built without Init still knows what to keep.
		settings:  DefaultSettings(),
		clipboard: nil,
		now:       time.Now,
	}
}

// WatchClipboard gives the plugin the clipboard to read and write, and
// takes what is on it now. A test gives it one of its own, so that the
// machine's own clipboard is not read or written; the launcher leaves it
// alone, and the plugin uses the machine's.
func (p *Plugin) WatchClipboard(clipboard Clipboard) {
	p.mu.Lock()
	p.clipboard = clipboard
	p.mu.Unlock()
	p.Capture()
}

// Capture takes what the clipboard holds into the history, which is what
// watching it does when it changes.
func (p *Plugin) Capture() { p.capture() }

// Init reads the settings and the history kept from earlier runs, and
// starts watching what the clipboard holds.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.installImages(ctx)
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")
	loaded := core.LoadSettingJsonStorage[Settings](ctx.API, "CopyQ", "Settings.json")
	p.settings = loaded
	if p.clipboard == nil {
		p.clipboard = systemClipboard{api: ctx.API}
	}
	p.cacheDir = ctx.API.PluginCacheDir

	p.history = NewHistory(p.settings.MaxItems)
	if kept, ok := core.LoadCacheBinaryStorage[[]Item](ctx.API, "CopyQ", "history"); ok {
		p.history.SetItems(kept)
		p.history.SortByTime()
	}

	// A settings file written by an older build says nothing about what it
	// did not have, so what it leaves empty is filled in, and written back
	// so that the file says what the launcher is doing.
	before := p.settings
	p.settings.applyDefaults()
	if before != p.settings {
		p.Save()
	}

	// The metadata the launcher matches the query against comes from the
	// settings, so that the user can change the keyword.
	p.Meta.ActionKeywords = []string{p.settings.ActionKeyword}

	// The key that shows the history in a window of its own, as CopyQ's
	// own hotkey shows it: the launcher's search box is left as it is.
	if p.settings.Hotkey != "" {
		p.api.RegisterHotkey(p.settings.Hotkey, p.showWindow)
	}

	p.watch()
	return nil
}

// installImages writes the pictures this plugin ships where the launcher
// reads them from, which is the plugin's own directory.
func (p *Plugin) installImages(ctx *core.PluginInitContext) {
	if err := core.InstallImages(ctx.CurrentPluginMetadata.PluginDirectory, images, "Images/*.png"); err != nil {
		ctx.API.LogError("CopyQ", "writing the plugin's pictures failed: "+err.Error())
	}
}

// Settings hands the plugin's settings over, which the launcher shows and
// changes in a window of its own.
func (p *Plugin) Settings() any { return &p.settings }

// Save writes the settings out.
func (p *Plugin) Save() {
	_ = core.SaveSettingJsonStorage(p.api, "CopyQ", "Settings.json", p.settings)
	p.saveHistory()
}

// watch looks at the clipboard until the plugin is told to stop.
func (p *Plugin) watch() {
	p.mu.Lock()
	if p.watching {
		p.mu.Unlock()
		return
	}
	p.watching = true
	p.stop = make(chan struct{})
	stop := p.stop
	p.mu.Unlock()

	go func() {
		ticker := time.NewTicker(watchAge)
		defer ticker.Stop()

		// Whatever is on the clipboard as the launcher starts is the first
		// thing the history holds, as CopyQ takes it.
		p.capture()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				p.capture()
			}
		}
	}()
}

// showWindow shows the history in a window of the plugin's own, which is
// what the hotkey does: the launcher's own window keeps its search box.
func (p *Plugin) showWindow() {
	p.capture()
	p.api.ShowResultsWindow(core.ResultsWindow{
		PluginID:    p.Meta.ID,
		Title:       p.words.T("plugin_copyq_window_title"),
		Placeholder: p.words.T("plugin_copyq_window_placeholder"),
		Search:      p.resultsFor,
		Choose: func(result core.Result) bool {
			p.chooseResult(result)
			return false
		},
		Delete: p.deleteResult,
	})
}

// resultsFor is the history as results for what the user typed in the
// plugin's own window.
func (p *Plugin) resultsFor(search string) []core.Result {
	results, err := p.Query(context.Background(), core.Query{
		Search:        search,
		TrimmedQuery:  search,
		OriginalQuery: search,
	})
	if err != nil {
		return nil
	}
	return results
}

// deleteResult takes the item a result stands for out of the history, which
// is what Delete does in the plugin's window: what was copied by mistake is
// in the way of what was meant.
func (p *Plugin) deleteResult(result core.Result) bool {
	item, ok := result.ContextData.(Item)
	if !ok {
		return false
	}
	index := p.indexOf(item)
	if index < 0 {
		return false
	}
	p.remove(index)
	return true
}

// chooseResult puts the item a result stands for back on the clipboard, and
// pastes it when the user asked for that.
func (p *Plugin) chooseResult(result core.Result) {
	item, ok := result.ContextData.(Item)
	if !ok {
		return
	}
	index := p.indexOf(item)
	if index < 0 {
		return
	}
	p.choose(index, p.settings.PasteAfterChoosing)
}

// Stop leaves off watching the clipboard.
func (p *Plugin) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.watching {
		return
	}
	close(p.stop)
	p.watching = false
}

// capture takes what the clipboard holds into the history, when it is
// something the user asked to keep and is not what was kept last.
func (p *Plugin) capture() {
	p.mu.Lock()
	clipboard := p.clipboard
	p.mu.Unlock()
	if clipboard == nil {
		return
	}
	content, err := clipboard.Read()
	if err != nil {
		p.api.LogError("CopyQ", "reading the clipboard failed: "+err.Error())
		return
	}

	item, ok := p.itemFor(content)
	if !ok {
		return
	}
	if p.add(item) {
		p.saveHistory()
	}
}

// itemFor is what the clipboard holds as a history item, and whether it is
// one the user asked to keep.
func (p *Plugin) itemFor(content core.ClipboardContent) (Item, bool) {
	now := p.now()

	switch content.Kind {
	case core.ClipboardText:
		if !p.settings.WatchText {
			return Item{}, false
		}
		text := strings.TrimSpace(content.Text)
		if text == "" {
			return Item{}, false
		}
		if p.settings.IgnoreShortText > 0 && len([]rune(text)) < p.settings.IgnoreShortText {
			return Item{}, false
		}
		if p.settings.IgnoreShortcut != "" && text == p.settings.IgnoreShortcut {
			return Item{}, false
		}
		return Item{Kind: KindText, Text: content.Text, Time: now}, true

	case core.ClipboardFiles:
		if !p.settings.WatchFiles || len(content.Files) == 0 {
			return Item{}, false
		}
		return Item{Kind: KindFiles, Files: content.Files, Time: now}, true

	case core.ClipboardImage:
		if !p.settings.WatchImages || len(content.Image) == 0 {
			return Item{}, false
		}
		name, err := p.writeImage(content.Image)
		if err != nil {
			p.api.LogError("CopyQ", "keeping a picture failed: "+err.Error())
			return Item{}, false
		}
		return Item{Kind: KindImage, Image: name, Time: now}, true
	}
	return Item{}, false
}

// writeImage keeps a picture in the plugin's cache, named after what it
// holds, so that the same picture copied twice is the same file.
func (p *Plugin) writeImage(image []byte) (string, error) {
	if p.cacheDir == "" {
		return "", os.ErrNotExist
	}
	directory := filepath.Join(p.cacheDir, "CopyQ", "images")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}

	sum := sha256.Sum256(image)
	name := hex.EncodeToString(sum[:8]) + ".png"
	path := filepath.Join(directory, name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return path, os.WriteFile(path, image, 0o644)
}

// add puts an item in the history and reports whether it was new.
func (p *Plugin) add(item Item) bool {
	p.mu.Lock()
	if p.history == nil {
		p.history = NewHistory(p.settings.MaxItems)
	}
	before := append([]Item(nil), p.history.Items()...)
	added := p.history.Add(item)
	dropped := p.history.dropped(before)
	p.mu.Unlock()

	p.removePictures(dropped)
	return added
}

// removePictures deletes the pictures the history let go.
func (p *Plugin) removePictures(paths []string) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			p.api.LogError("CopyQ", "removing a picture failed: "+err.Error())
		}
	}
}

// saveHistory keeps the history for the next run.
func (p *Plugin) saveHistory() {
	p.mu.Lock()
	items := append([]Item(nil), p.history.Items()...)
	p.mu.Unlock()

	_ = core.SaveCacheBinaryStorage(p.api, "CopyQ", "history", items)
}

// items is the history as it stands.
func (p *Plugin) items() []Item {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.history == nil {
		return nil
	}
	return append([]Item(nil), p.history.Items()...)
}

// Query answers what the user typed with the items the clipboard held that
// match it, newest first.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	// What is on the clipboard now is taken first, so that a thing copied
	// and searched for at once is there.
	p.capture()

	items := p.items()
	if len(items) == 0 {
		return nil, nil
	}

	// The precision is the lowest one, as a search of what the clipboard
	// held needs: a word in the middle of a long text is what the user is
	// looking for, and the matcher's own precision would leave it out.
	matcher := p.api.Matcher()
	matcher.UserSettingSearchPrecision = core.SearchPrecisionLow
	search := strings.TrimSpace(query.Search)

	var results []core.Result
	for index, item := range items {
		score, highlight := p.score(matcher, search, item)
		if search != "" && score == 0 {
			continue
		}
		results = append(results, p.resultFor(index, item, score, highlight))
	}

	sortByScore(results)
	return results, nil
}

// score is how well an item answers the query, and which characters of its
// title matched.
func (p *Plugin) score(matcher core.StringMatcher, search string, item Item) (int, []int) {
	if search == "" {
		return 100, nil
	}

	// A search finds an item by what it holds: its text, or the names of
	// its files.
	match := matcher.FuzzyMatch(search, item.Search())
	if !match.IsSearchPrecisionScoreMet() {
		return 0, nil
	}

	// The highlight is in the title, which may be the first line of a
	// longer text or the name of a file among several.
	title := matcher.FuzzyMatch(search, item.Title())
	if title.IsSearchPrecisionScoreMet() {
		return match.Score(), title.MatchData
	}
	return match.Score(), nil
}

// resultFor is one item as a result, with what choosing it does.
func (p *Plugin) resultFor(index int, item Item, score int, highlight []int) core.Result {
	title := item.Title()
	result := core.NewResult(title, item.Subtitle(p.now()), p.iconFor(item), score)
	result.TitleHighlightData = highlight
	result.AutoCompleteText = p.settings.ActionKeyword + " " + title
	result.ContextData = item
	if item.Kind == KindImage && item.Image != "" {
		result.Preview.FilePath = item.Image
		result.Preview.PreviewImagePath = item.Image
	}
	if item.Kind == KindFiles && len(item.Files) > 0 {
		result.Preview.FilePath = item.Files[0]
	}
	if item.Pinned {
		result.ShowBadge = true
		result.BadgeIcoPath = pinnedIcon
	}

	result.Action = func(action core.ActionContext) bool {
		switch {
		case action.SpecialKeyState.CtrlPressed && !action.SpecialKeyState.ShiftPressed:
			// Ctrl+Enter puts it on the clipboard and leaves it there,
			// which is what choosing without pasting does.
			p.choose(index, false)
		case action.SpecialKeyState.ShiftPressed && !action.SpecialKeyState.CtrlPressed:
			// Shift+Enter pins it, so that it is kept.
			p.pin(index, !item.Pinned)
		default:
			p.choose(index, p.settings.PasteAfterChoosing)
		}
		return true
	}
	return result
}

// iconFor is the picture a result is drawn with.
func (p *Plugin) iconFor(item Item) string {
	switch item.Kind {
	case KindFiles:
		if len(item.Files) > 0 {
			return item.Files[0]
		}
		return fileIcon
	case KindImage:
		if item.Image != "" {
			return item.Image
		}
		return imageIcon
	default:
		return textIcon
	}
}

// choose puts an item back on the clipboard, and pastes it into the window
// the user came from when they asked for that.
func (p *Plugin) choose(index int, paste bool) {
	items := p.items()
	if index < 0 || index >= len(items) {
		return
	}

	item := items[index]
	p.mu.Lock()
	clipboard := p.clipboard
	p.mu.Unlock()
	if clipboard == nil {
		return
	}
	if err := clipboard.Write(item); err != nil {
		p.api.LogError("CopyQ", "putting an item back on the clipboard failed: "+err.Error())
		return
	}

	// The item is now what the clipboard holds, so it is the newest one,
	// and the history does not take it again as a new one.
	p.mu.Lock()
	if p.history != nil {
		p.history.Remove(index)
	}
	p.mu.Unlock()
	p.add(item)

	if paste {
		p.api.PasteIntoWindow()
		return
	}
	p.api.HideMainWindow()
}

// pin keeps an item, or lets it go.
func (p *Plugin) pin(index int, pinned bool) {
	p.mu.Lock()
	ok := p.history.Pin(index, pinned)
	p.mu.Unlock()

	if ok {
		p.saveHistory()
		p.api.ReQuery(true)
	}
}

// remove takes an item out of the history.
func (p *Plugin) remove(index int) {
	p.mu.Lock()
	before := append([]Item(nil), p.history.Items()...)
	ok := p.history.Remove(index)
	dropped := p.history.dropped(before)
	p.mu.Unlock()

	p.removePictures(dropped)
	if ok {
		p.saveHistory()
		p.api.ReQuery(true)
	}
}

// clear takes everything out but what the user pinned.
func (p *Plugin) clear() {
	p.mu.Lock()
	before := append([]Item(nil), p.history.Items()...)
	p.history.Clear()
	dropped := p.history.dropped(before)
	p.mu.Unlock()

	p.removePictures(dropped)
	p.saveHistory()
	p.api.ReQuery(true)
}

// LoadContextMenus offers what can be done with an item: keeping it,
// taking it out, and emptying the history.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	item, ok := selectedResult.ContextData.(Item)
	if !ok {
		return nil
	}

	index := p.indexOf(item)
	if index < 0 {
		return nil
	}

	words := p.words
	var items []core.Result

	keep := core.NewResult(words.T("plugin_copyq_pin"), "", pinnedIcon, 0)
	keep.Action = func(core.ActionContext) bool {
		p.pin(index, !item.Pinned)
		return true
	}
	items = append(items, keep)

	take := core.NewResult(words.T("plugin_copyq_remove"), "", removeIcon, 0)
	take.Action = func(core.ActionContext) bool {
		p.remove(index)
		return true
	}
	items = append(items, take)

	if len(p.items()) > 1 {
		empty := core.NewResult(words.T("plugin_copyq_clear"), "", removeIcon, 0)
		empty.Action = func(core.ActionContext) bool {
			p.clear()
			return true
		}
		items = append(items, empty)
	}
	return items
}

// indexOf is where an item is in the history, by what it holds.
func (p *Plugin) indexOf(item Item) int {
	for i, candidate := range p.items() {
		if sameItem(candidate, item) {
			return i
		}
	}
	return -1
}

// sortByScore puts the best answer first, and keeps the newest of the
// answers that are equal first, which is the order the history is in.
func sortByScore(results []core.Result) {
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
}

// The pictures the plugin draws its results with, which the reference's
// own clipboard manager has no equivalent of: the launcher's own icons are
// used where a thing has one, and these where it has not.
const (
	textIcon   = "Images/copyq.png"
	fileIcon   = "Images/copyq.png"
	imageIcon  = "Images/copyq.png"
	pinnedIcon = "Images/copyq.png"
	removeIcon = "Images/copyq.png"
)
