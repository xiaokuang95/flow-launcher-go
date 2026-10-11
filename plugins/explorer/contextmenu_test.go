package explorer

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"launch/core"
)

// fakeDeleter stands in for the machine's files, so that a test watches
// what was asked to be removed rather than losing a file of its own.
type fakeDeleter struct {
	removed   []string
	removedIs []bool
	err       error
}

func (f *fakeDeleter) Delete(path string, isDir bool) error {
	if f.err != nil {
		return f.err
	}
	f.removed = append(f.removed, path)
	f.removedIs = append(f.removedIs, isDir)
	return nil
}

// menuFor builds a plugin, lets the test change what it was built with,
// and asks it for the menu of one result.
func menuFor(t *testing.T, found SearchResult, setup func(*Plugin)) (*Plugin, []core.Result) {
	t.Helper()

	plugin, _ := newTestPlugin(t)
	plugin.deleter = &fakeDeleter{}
	if setup != nil {
		setup(plugin)
	}
	return plugin, plugin.LoadContextMenus(core.Result{ContextData: found})
}

// itemTitles are what the menu says.
func itemTitles(items []core.Result) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles
}

// item finds one item of the menu by what it says.
func item(t *testing.T, items []core.Result, title string) core.Result {
	t.Helper()

	for _, item := range items {
		if item.Title == title {
			return item
		}
	}
	t.Fatalf("the menu %v has no item saying %q", itemTitles(items), title)
	return core.Result{}
}

func TestAFileResultOffersWhatCanBeDoneWithIt(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\setup.exe`, Type: ResultTypeFile}
	_, items := menuFor(t, found, func(plugin *Plugin) {
		plugin.settings.EditorPath = `C:\Program Files\Notepad++\notepad++.exe`
	})

	titles := itemTitles(items)
	for _, want := range []string{
		"Open With Editor: notepad++",
		"Open containing folder",
		"Open With",
		"Add to Quick Access",
		"Copy path",
		"Copy name",
		"Copy",
		"Delete",
		"Run as different user",
	} {
		item(t, items, want)
	}
	// A file is not opened in a shell, and it is not a folder to exclude.
	for _, unwanted := range []string{"Open With Shell", "Exclude current"} {
		for _, title := range titles {
			if strings.Contains(title, unwanted) {
				t.Errorf("the menu says %q, which belongs to a folder", title)
			}
		}
	}
}

func TestAFolderResultOffersWhatCanBeDoneWithIt(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\Work`, Type: ResultTypeFolder}
	_, items := menuFor(t, found, func(plugin *Plugin) {
		plugin.settings.ShellPath = "cmd.exe"
	})

	item(t, items, "Open With Shell: cmd")
	item(t, items, "Open containing folder")
	item(t, items, "Exclude current and sub-directories from Index Search")
	item(t, items, "Add to Quick Access")
	item(t, items, "Copy path")
	item(t, items, "Delete")

	for _, title := range itemTitles(items) {
		if strings.Contains(title, "Run as different user") || title == "Open With" {
			t.Errorf("the menu says %q, which belongs to a file", title)
		}
	}
}

func TestAResultOfAnotherPluginHasNoMenu(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	items := plugin.LoadContextMenus(core.Result{ContextData: "something else"})
	if len(items) != 0 {
		t.Errorf("the menu is %v, want nothing for a result the explorer did not make", itemTitles(items))
	}
}

func TestCopyingAPathCopiesTextAndCopyingTheItemCopiesTheFile(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\report.pdf`, Type: ResultTypeFile}
	plugin, items := menuFor(t, found, nil)

	var copied []string
	var direct []bool
	plugin.api.Hooks.CopyToClipboard = func(text string, directCopy, notify bool) {
		copied = append(copied, text)
		direct = append(direct, directCopy)
	}

	item(t, items, "Copy path").Action(core.ActionContext{})
	item(t, items, "Copy name").Action(core.ActionContext{})
	item(t, items, "Copy").Action(core.ActionContext{})

	want := []string{
		`C:\Users\crazy\Documents\report.pdf`,
		"report.pdf",
		`C:\Users\crazy\Documents\report.pdf`,
	}
	if len(copied) != len(want) {
		t.Fatalf("the clipboard was asked for %v, want %v", copied, want)
	}
	for i := range want {
		if copied[i] != want[i] {
			t.Errorf("the clipboard was asked for %q, want %q", copied[i], want[i])
		}
	}
	// The path and the name are text; the item itself is the file.
	if direct[0] || direct[1] || !direct[2] {
		t.Errorf("the items were copied as files %v, want text, text, file", direct)
	}
}

func TestDeletingAsksFirstAndThenRemovesIt(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\Work`, Type: ResultTypeFolder}
	plugin, items := menuFor(t, found, nil)
	deleter := &fakeDeleter{}
	plugin.deleter = deleter

	asked := 0
	plugin.api.Hooks.ShowMsgBox = func(text, caption string, buttons core.MessageBoxButtons, icon core.MessageBoxIcon) core.MessageBoxResult {
		asked++
		return core.MessageBoxResultCancel
	}

	// A user who says no loses nothing.
	item(t, items, "Delete").Action(core.ActionContext{})
	if asked != 1 {
		t.Errorf("the user was asked %d times, want once", asked)
	}
	if len(deleter.removed) != 0 {
		t.Errorf("the machine was asked to remove %v, want nothing", deleter.removed)
	}

	plugin.api.Hooks.ShowMsgBox = func(text, caption string, buttons core.MessageBoxButtons, icon core.MessageBoxIcon) core.MessageBoxResult {
		return core.MessageBoxResultOK
	}
	item(t, items, "Delete").Action(core.ActionContext{})

	if len(deleter.removed) != 1 || deleter.removed[0] != found.FullPath {
		t.Fatalf("the machine was asked to remove %v, want the folder", deleter.removed)
	}
	if !deleter.removedIs[0] {
		t.Errorf("the folder was removed as a file")
	}
}

func TestDeletingSaysSoWhenItFails(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\report.pdf`, Type: ResultTypeFile}
	plugin, items := menuFor(t, found, nil)
	plugin.deleter = &fakeDeleter{err: errors.New("in use")}
	plugin.api.Hooks.ShowMsgBox = func(text, caption string, buttons core.MessageBoxButtons, icon core.MessageBoxIcon) core.MessageBoxResult {
		return core.MessageBoxResultOK
	}

	said := ""
	plugin.api.Hooks.ShowMsgError = func(title, subTitle string) { said = title }

	item(t, items, "Delete").Action(core.ActionContext{})

	if !strings.Contains(said, "report.pdf") {
		t.Errorf("the user was told %q, want what could not be deleted", said)
	}
}

func TestQuickAccessIsPinnedAndUnpinned(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "Work")
	found := SearchResult{FullPath: directory, Type: ResultTypeFolder}
	plugin, items := menuFor(t, found, nil)

	item(t, items, "Add to Quick Access").Action(core.ActionContext{})
	if len(plugin.settings.QuickAccessLinks) != 1 || plugin.settings.QuickAccessLinks[0].Path != directory {
		t.Fatalf("the pinned folders are %v, want the one just pinned", plugin.settings.QuickAccessLinks)
	}
	if plugin.settings.QuickAccessLinks[0].Name != "Work" {
		t.Errorf("the pinned folder is called %q, want its own name", plugin.settings.QuickAccessLinks[0].Name)
	}

	// The same folder is now offered the other way round.
	items = plugin.LoadContextMenus(core.Result{ContextData: found})
	item(t, items, "Remove from Quick Access").Action(core.ActionContext{})

	if len(plugin.settings.QuickAccessLinks) != 0 {
		t.Errorf("the pinned folders are %v, want none", plugin.settings.QuickAccessLinks)
	}
}

func TestAFolderCanBeLeftOutOfASearch(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\Secrets`, Type: ResultTypeFolder}
	plugin, items := menuFor(t, found, nil)

	item(t, items, "Exclude current and sub-directories from Index Search").Action(core.ActionContext{})

	if len(plugin.settings.IndexSearchExcludedSubdirectoryPaths) != 1 ||
		plugin.settings.IndexSearchExcludedSubdirectoryPaths[0].Path != found.FullPath {
		t.Errorf("the folders left out are %v, want the one just excluded", plugin.settings.IndexSearchExcludedSubdirectoryPaths)
	}

	// Asking twice leaves one entry, not two.
	items = plugin.LoadContextMenus(core.Result{ContextData: found})
	item(t, items, "Exclude current and sub-directories from Index Search").Action(core.ActionContext{})
	if len(plugin.settings.IndexSearchExcludedSubdirectoryPaths) != 1 {
		t.Errorf("the folders left out are %v, want one", plugin.settings.IndexSearchExcludedSubdirectoryPaths)
	}
}

func TestRunningAsAnotherUserIsOnlyOfferedForPrograms(t *testing.T) {
	for _, test := range []struct {
		path string
		want bool
	}{
		{`C:\Users\crazy\setup.exe`, true},
		{`C:\Users\crazy\run.bat`, true},
		{`C:\Users\crazy\installer.msi`, true},
		{`C:\Users\crazy\report.pdf`, false},
		{`C:\Users\crazy\notes.txt`, false},
	} {
		if got := canRunAsDifferentUser(test.path); got != test.want {
			t.Errorf("canRunAsDifferentUser(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}

func TestTheMenuOffersTheMachinesOwnMenu(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\report.pdf`, Type: ResultTypeFile}
	plugin, items := menuFor(t, found, nil)

	var shown []string
	var extended []bool
	plugin.api.Hooks.ShowShellContextMenu = func(path string, extendedVerbs bool) bool {
		shown = append(shown, path)
		extended = append(extended, extendedVerbs)
		return true
	}

	item(t, items, "Show Windows Context Menu").Action(core.ActionContext{})

	if len(shown) != 1 || shown[0] != found.FullPath {
		t.Fatalf("the launcher was asked for the machine's menu of %v, want the file", shown)
	}
	if extended[0] {
		t.Errorf("the plain menu was asked for the extended one")
	}
}

func TestAltOverAFileAsksForTheMachinesOwnMenu(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\report.pdf`, Type: ResultTypeFile}
	plugin, _ := newTestPlugin(t)

	asked := ""
	extended := true
	plugin.api.Hooks.ShowShellContextMenu = func(path string, extendedVerbs bool) bool {
		asked, extended = path, extendedVerbs
		return true
	}

	// Alt is the key the reference's explorer answers with the shell's own
	// menu, and Alt+Shift with the menu's extended items.
	plugin.fileResult(found, core.Query{}).Action(
		core.ActionContext{SpecialKeyState: core.SpecialKeyState{AltPressed: true}})
	if asked != found.FullPath || extended {
		t.Errorf("Alt asked for %q (extended %v), want the plain menu of the file", asked, extended)
	}

	asked, extended = "", false
	plugin.fileResult(found, core.Query{}).Action(
		core.ActionContext{SpecialKeyState: core.SpecialKeyState{AltPressed: true, ShiftPressed: true}})
	if asked != found.FullPath || !extended {
		t.Errorf("Alt+Shift asked for %q (extended %v), want the extended menu", asked, extended)
	}
}

func TestAltOverAFolderAsksForTheMachinesOwnMenu(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\Work`, Type: ResultTypeFolder}
	plugin, _ := newTestPlugin(t)

	asked := ""
	plugin.api.Hooks.ShowShellContextMenu = func(path string, extendedVerbs bool) bool {
		asked = path
		return true
	}

	plugin.folderResult(found, core.Query{}).Action(
		core.ActionContext{SpecialKeyState: core.SpecialKeyState{AltPressed: true}})

	if asked != found.FullPath {
		t.Errorf("Alt asked for the machine's menu of %q, want the folder", asked)
	}
}

func TestAMachineMenuThatCannotBeShownOpensTheFile(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\report.pdf`, Type: ResultTypeFile}
	plugin, _ := newTestPlugin(t)
	plugin.api.Hooks.ShowShellContextMenu = func(path string, extendedVerbs bool) bool { return false }

	// A launcher that cannot show the shell's menu still does something
	// with the key rather than nothing at all.
	plugin.fileResult(found, core.Query{}).Action(
		core.ActionContext{SpecialKeyState: core.SpecialKeyState{AltPressed: true}})
}

func TestOpeningContainingFolderAsksTheLauncher(t *testing.T) {
	found := SearchResult{FullPath: `C:\Users\crazy\Documents\Work\report.pdf`, Type: ResultTypeFile}
	plugin, items := menuFor(t, found, nil)

	directory := ""
	file := ""
	plugin.api.Hooks.OpenDirectory = func(directoryPath, fileNameOrFilePath string) {
		directory, file = directoryPath, fileNameOrFilePath
	}

	item(t, items, "Open containing folder").Action(core.ActionContext{})

	if directory != `C:\Users\crazy\Documents\Work` || file != found.FullPath {
		t.Errorf("the launcher was asked to open %q with %q picked out", directory, file)
	}
}
