package copyq

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"launch/core"
)

// fakeClipboard stands in for the machine's clipboard, so that a test
// watches what the plugin read and wrote rather than reading and writing
// the machine's own.
type fakeClipboard struct {
	// content is what the clipboard holds now.
	content core.ClipboardContent
	// written are the items the plugin put back on it.
	written []Item
	// err, when set, is what reading answers with.
	err error
}

func (f *fakeClipboard) Read() (core.ClipboardContent, error) {
	if f.err != nil {
		return core.ClipboardContent{}, f.err
	}
	return f.content, nil
}

func (f *fakeClipboard) Write(item Item) error {
	f.written = append(f.written, item)
	return nil
}

// pastes counts what the plugin asked the launcher to paste.
type pastes struct{ count int }

func newTestPlugin(t *testing.T, clipboard *fakeClipboard) (*Plugin, *pastes) {
	t.Helper()

	plugin := New()
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	plugin.clipboard = clipboard
	plugin.cacheDir = t.TempDir()
	plugin.history = NewHistory(plugin.settings.MaxItems)
	plugin.now = func() time.Time { return time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC) }

	pasted := &pastes{}
	plugin.api.Hooks.PasteIntoWindow = func() { pasted.count++ }
	return plugin, pasted
}

// titlesOf are what the results say.
func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestTheClipboardIsKeptNewestFirst(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	for _, text := range []string{"first", "second", "third"} {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: text}
		plugin.capture()
	}

	items := plugin.items()
	if len(items) != 3 {
		t.Fatalf("the history holds %d items, want three", len(items))
	}
	if items[0].Text != "third" || items[2].Text != "first" {
		t.Errorf("the history is %q, want the newest first", []string{items[0].Text, items[1].Text, items[2].Text})
	}
}

func TestTheSameThingCopiedTwiceIsKeptOnce(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	for _, text := range []string{"one", "two", "two", "one"} {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: text}
		plugin.capture()
	}

	items := plugin.items()
	if len(items) != 2 {
		t.Fatalf("the history holds %d items, want two", len(items))
	}
	// The one copied last is the newest, though it was copied before.
	if items[0].Text != "one" || items[1].Text != "two" {
		t.Errorf("the history is %q, want the last one copied first", []string{items[0].Text, items[1].Text})
	}
}

func TestWhatTheUserDidNotAskForIsNotKept(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)
	plugin.settings.WatchText = false
	plugin.settings.WatchImages = true

	clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "not kept"}
	plugin.capture()
	if items := plugin.items(); len(items) != 0 {
		t.Errorf("the history holds %v, want text left out", items)
	}

	clipboard.content = core.ClipboardContent{Kind: core.ClipboardImage, Image: []byte("a picture")}
	plugin.capture()
	if items := plugin.items(); len(items) != 1 || items[0].Kind != KindImage {
		t.Errorf("the history holds %v, want the picture kept", items)
	}
}

func TestShortTextIsLeftOutWhenTheUserAsks(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)
	plugin.settings.IgnoreShortText = 3

	for _, text := range []string{"ab", "abc", "abcd"} {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: text}
		plugin.capture()
	}

	items := plugin.items()
	if len(items) != 2 {
		t.Fatalf("the history holds %d items, want the two that are long enough", len(items))
	}
	for _, item := range items {
		if len(item.Text) < 3 {
			t.Errorf("the history holds %q, which is too short to keep", item.Text)
		}
	}
}

func TestWhatTheLauncherItselfCopiesIsLeftOut(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	// The launcher's own shortcut is expanded to what the clipboard holds,
	// so keeping it would keep the history as text.
	clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "{clipboard}"}
	plugin.capture()

	if items := plugin.items(); len(items) != 0 {
		t.Errorf("the history holds %v, want the launcher's own shortcut left out", items)
	}
}

func TestOnlyTheNewestItemsAreKept(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)
	plugin.history = NewHistory(3)

	for i := 0; i < 6; i++ {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "item " + itoa(i)}
		plugin.capture()
	}

	items := plugin.items()
	if len(items) != 3 {
		t.Fatalf("the history holds %d items, want three", len(items))
	}
	if items[0].Text != "item 5" || items[2].Text != "item 3" {
		t.Errorf("the history is %v, want the newest three", titlesOf(plugin.results(t, "")))
	}
}

func TestAPinnedItemIsKeptThroughThePruning(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)
	plugin.history = NewHistory(2)

	clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "keep me"}
	plugin.capture()
	plugin.pin(0, true)

	for i := 0; i < 5; i++ {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "item " + itoa(i)}
		plugin.capture()
	}

	items := plugin.items()
	if len(items) == 0 || items[0].Text != "keep me" {
		t.Fatalf("the history is %v, want the pinned item kept", titlesOf(plugin.results(t, "")))
	}
	for _, item := range items[1:] {
		if item.Text == "keep me" {
			t.Errorf("the pinned item is in the history twice")
		}
	}
}

// results asks the plugin what it answers with.
func (p *Plugin) results(t *testing.T, search string) []core.Result {
	t.Helper()

	results, err := p.Query(context.Background(), core.Query{Search: search, TrimmedQuery: search})
	if err != nil {
		t.Fatal(err)
	}
	return results
}

func TestAQueryFindsWhatTheClipboardHeld(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	for _, text := range []string{"the first thing", "another thing", "https://example.com/page"} {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: text}
		plugin.capture()
	}

	results := plugin.results(t, "another")
	if len(results) != 1 {
		t.Fatalf("the search found %v, want one", titlesOf(results))
	}
	if results[0].Title != "another thing" {
		t.Errorf("the search found %q", results[0].Title)
	}

	// A query that matches nothing answers with nothing, rather than with
	// the whole history.
	if results := plugin.results(t, "nothing here at all"); len(results) != 0 {
		t.Errorf("the search found %v, want nothing", titlesOf(results))
	}

	// An empty query is the whole history.
	if results := plugin.results(t, ""); len(results) != 3 {
		t.Errorf("the empty query found %v, want everything", titlesOf(results))
	}
}

func TestAFileIsFoundByItsNameAndPastItsFolder(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	clipboard.content = core.ClipboardContent{
		Kind:  core.ClipboardFiles,
		Files: []string{`C:\Users\crazy\Documents\quarterly-report.pdf`},
	}
	plugin.capture()

	if results := plugin.results(t, "quarterly"); len(results) != 1 {
		t.Fatalf("the search found %v, want the file by its name", titlesOf(results))
	}
	if results := plugin.results(t, "Documents"); len(results) != 0 {
		t.Errorf("the search found %v, want nothing: a file is found by its name", titlesOf(results))
	}
}

func TestChoosingAnItemPutsItBackAndPastesIt(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, pasted := newTestPlugin(t, clipboard)

	clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "something to paste"}
	plugin.capture()

	results := plugin.results(t, "")
	if len(results) != 1 {
		t.Fatalf("the history holds %v", titlesOf(results))
	}
	results[0].Action(core.ActionContext{})

	if len(clipboard.written) != 1 || clipboard.written[0].Text != "something to paste" {
		t.Fatalf("the clipboard was given %v, want the item", clipboard.written)
	}
	if pasted.count != 1 {
		t.Errorf("the item was pasted %d times, want once", pasted.count)
	}
}

func TestChoosingWithoutPastingLeavesItOnTheClipboard(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, pasted := newTestPlugin(t, clipboard)

	clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "copy me"}
	plugin.capture()

	// Ctrl+Enter copies it and leaves it there.
	plugin.results(t, "")[0].Action(core.ActionContext{
		SpecialKeyState: core.SpecialKeyState{CtrlPressed: true},
	})

	if len(clipboard.written) != 1 {
		t.Fatalf("the clipboard was given %v, want the item", clipboard.written)
	}
	if pasted.count != 0 {
		t.Errorf("the item was pasted %d times, want none", pasted.count)
	}
}

func TestAnItemCopiedBackIsTheNewestOne(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	for _, text := range []string{"one", "two", "three"} {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: text}
		plugin.capture()
	}

	// The oldest item is chosen, which puts it at the front.
	plugin.results(t, "")[2].Action(core.ActionContext{})

	items := plugin.items()
	if len(items) != 3 {
		t.Fatalf("the history holds %d items, want three", len(items))
	}
	if items[0].Text != "one" {
		t.Errorf("the history is %q, want the chosen one first", []string{items[0].Text, items[1].Text, items[2].Text})
	}
}

func TestTheMenuPinsAndRemovesAnItem(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	for _, text := range []string{"one", "two"} {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: text}
		plugin.capture()
	}

	results := plugin.results(t, "")
	items := plugin.LoadContextMenus(results[0])
	if len(items) == 0 {
		t.Fatalf("the menu is empty")
	}

	// The first item of the menu pins the item it was opened on, which is
	// the newest one.
	items[0].Action(core.ActionContext{})
	kept := plugin.items()
	if !kept[0].Pinned || kept[0].Text != "two" {
		t.Fatalf("the item was not pinned: %+v", kept)
	}

	// The menu of another item takes that one out.
	results = plugin.results(t, "one")
	items = plugin.LoadContextMenus(results[0])
	items[1].Action(core.ActionContext{})

	kept = plugin.items()
	if len(kept) != 1 || kept[0].Text != "two" {
		t.Errorf("the history is %v, want the pinned one left", titlesOf(plugin.results(t, "")))
	}
}

func TestClearingLeavesWhatTheUserPinned(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	for _, text := range []string{"pinned one", "plain"} {
		clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: text}
		plugin.capture()
	}
	plugin.pin(1, true)

	plugin.clear()

	items := plugin.items()
	if len(items) != 1 || items[0].Text != "pinned one" {
		t.Errorf("the history is %v, want only what was pinned", titlesOf(plugin.results(t, "")))
	}
}

func TestChoosingAnItemWritesItThroughTheLaunchersOwnCopy(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	// The launcher's own copy is what puts an item back: the test watches
	// what the plugin asked it for, which is what a window does.
	copied := make([]string, 0, 2)
	plugin.api.Hooks.CopyToClipboard = func(text string, directCopy, notify bool) {
		copied = append(copied, text)
	}
	plugin.clipboard = writingClipboard{api: plugin.api, content: core.ClipboardContent{
		Kind: core.ClipboardText, Text: "the item to put back",
	}}
	plugin.Capture()

	plugin.results(t, "")[0].Action(core.ActionContext{})

	if len(copied) != 1 || copied[0] != "the item to put back" {
		t.Errorf("the launcher was asked to copy %v, want the chosen item", copied)
	}
}

// writingClipboard reads a fixed thing and writes through the launcher's
// own copy, as the machine's clipboard does.
type writingClipboard struct {
	api     *core.PublicAPI
	content core.ClipboardContent
}

func (c writingClipboard) Read() (core.ClipboardContent, error) { return c.content, nil }

func (c writingClipboard) Write(item Item) error {
	c.api.CopyToClipboard(item.Text, false, false)
	return nil
}

func TestTheHotkeyShowsAWindowOfItsOwn(t *testing.T) {
	clipboard := &fakeClipboard{content: core.ClipboardContent{Kind: core.ClipboardText, Text: "what the clipboard held"}}
	plugin, _ := newTestPlugin(t, clipboard)

	// What the launcher is asked to show, which is what the hotkey does.
	var shown core.ResultsWindow
	plugin.api.Hooks.ShowResultsWindow = func(window core.ResultsWindow) { shown = window }
	plugin.api.Hooks.RegisterHotkey = func(hotkey string, action func()) bool {
		if hotkey != "Alt+C" {
			t.Errorf("the key is %q, want the one in the settings", hotkey)
		}
		action()
		return true
	}

	// Init is what binds the key, and pressing it is what shows the window.
	plugin.Init(&core.PluginInitContext{API: plugin.api, CurrentPluginMetadata: plugin.Meta})

	if shown.PluginID != plugin.Meta.ID {
		t.Fatalf("the window is the plugin's %q", shown.PluginID)
	}
	if shown.Title == "" || shown.Placeholder == "" {
		t.Errorf("the window has no title or placeholder: %+v", shown)
	}

	// Its search is the history, and choosing one is what Enter does.
	results := shown.Search("")
	if len(results) != 1 || results[0].Title != "what the clipboard held" {
		t.Fatalf("the window shows %v", titlesOf(results))
	}
	if keep := shown.Choose(results[0]); keep {
		t.Errorf("the window stays open after a choice")
	}
	if len(clipboard.written) != 1 || clipboard.written[0].Text != "what the clipboard held" {
		t.Errorf("the clipboard was given %v, want the chosen item", clipboard.written)
	}

	// And it is not the launcher's search box that was filled in.
	if plugin.api.Hooks.ChangeQuery != nil {
		t.Errorf("the hotkey changed the launcher's query, which is what the window is for")
	}
}

func TestTheHistoryIsKeptBetweenRuns(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	clipboard.content = core.ClipboardContent{Kind: core.ClipboardText, Text: "kept for later"}
	plugin.capture()
	plugin.Save()

	// A launcher started again reads what was kept.
	kept, ok := core.LoadCacheBinaryStorage[[]Item](plugin.api, "CopyQ", "history")
	if !ok || len(kept) != 1 || kept[0].Text != "kept for later" {
		t.Fatalf("what was kept is %v", kept)
	}
}

func TestAQueryWithAnEmptyHistoryAnswersWithNothing(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	if results := plugin.results(t, "anything"); len(results) != 0 {
		t.Errorf("the search found %v, want nothing", titlesOf(results))
	}
}

func TestAPictureSaysHowBigItIs(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)

	// A 3×2 picture, which is what a result is named after.
	picture := pngOfSize(t, 3, 2)
	clipboard.content = core.ClipboardContent{Kind: core.ClipboardImage, Image: picture}
	plugin.capture()

	items := plugin.items()
	if len(items) != 1 {
		t.Fatalf("the history holds %d items", len(items))
	}
	if got := items[0].Title(); got != "Image 3×2" {
		t.Errorf("the picture is called %q, want its size", got)
	}

	// And it is what a result offers to show.
	result := plugin.results(t, "")[0]
	if result.Preview.PreviewImagePath != items[0].Image {
		t.Errorf("the result shows %q, want the picture itself", result.Preview.PreviewImagePath)
	}
}

// pngOfSize is a picture of the size given.
func pngOfSize(t *testing.T, width, height int) []byte {
	t.Helper()

	var buffer bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestAPictureIsKeptAsAFile(t *testing.T) {
	clipboard := &fakeClipboard{}
	plugin, _ := newTestPlugin(t, clipboard)
	plugin.settings.WatchImages = true

	picture := []byte("\x89PNG\r\n\x1a\n the bytes of a picture")
	clipboard.content = core.ClipboardContent{Kind: core.ClipboardImage, Image: picture}
	plugin.capture()

	items := plugin.items()
	if len(items) != 1 || items[0].Kind != KindImage {
		t.Fatalf("the history holds %v, want the picture", items)
	}
	if !strings.HasSuffix(items[0].Image, ".png") {
		t.Errorf("the picture is kept as %q, want a png", items[0].Image)
	}

	// The same picture again is the same item, not a second one.
	plugin.capture()
	if items := plugin.items(); len(items) != 1 {
		t.Errorf("the history holds %d items, want the picture once", len(items))
	}
}

func TestWhatIsOnTheClipboardIsTakenWhenThePluginStarts(t *testing.T) {
	clipboard := &fakeClipboard{content: core.ClipboardContent{Kind: core.ClipboardText, Text: "already there"}}
	plugin, _ := newTestPlugin(t, clipboard)

	plugin.capture()

	if items := plugin.items(); len(items) != 1 || items[0].Text != "already there" {
		t.Errorf("the history holds %v, want what the clipboard held", items)
	}
}

func TestAnItemSaysWhatItIsAndWhen(t *testing.T) {
	when := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)

	text := Item{Kind: KindText, Text: "the first line\nthe second line", Time: when}
	if got := text.Title(); got != "the first line" {
		t.Errorf("a text is called %q, want its first line", got)
	}
	if got := text.Subtitle(when.Add(5 * time.Minute)); got != "the second line · 5m ago" {
		t.Errorf("the subtitle is %q", got)
	}

	files := Item{Kind: KindFiles, Files: []string{`C:\a\report.pdf`}, Time: when}
	if got := files.Title(); got != "report.pdf" {
		t.Errorf("a file is called %q, want its name", got)
	}
	if got := files.Subtitle(when); !strings.Contains(got, `C:\a`) {
		t.Errorf("the subtitle is %q, want the folder", got)
	}

	several := Item{Kind: KindFiles, Files: []string{`C:\a\one.pdf`, `C:\a\two.pdf`}, Time: when}
	if got := several.Title(); got != "one.pdf + 1" {
		t.Errorf("several files are called %q", got)
	}
}
