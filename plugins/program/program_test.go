package program

import (
	"context"
	"path/filepath"
	"testing"

	"launch/core"
)

// fakeFinder stands in for the machine, so that a test searches a list of
// its own rather than the machine's own Start Menu, and watches what the
// plugin asked to run rather than running it.
type fakeFinder struct {
	programs     []Program
	started      []string
	startedAdmin []bool
	startedAs    []string
	asDifferent  []string
}

func (f *fakeFinder) Find(Settings) ([]Program, error) { return f.programs, nil }

func (f *fakeFinder) Start(program Program, arguments string, runAsAdmin bool) error {
	f.started = append(f.started, program.ProgramFile())
	f.startedAdmin = append(f.startedAdmin, runAsAdmin)
	return nil
}

func (f *fakeFinder) StartAsDifferentUser(program Program, arguments string) error {
	f.asDifferent = append(f.asDifferent, program.ProgramFile())
	return nil
}

func newTestPlugin(t *testing.T, programs ...Program) (*Plugin, *fakeFinder) {
	t.Helper()

	finder := &fakeFinder{programs: programs}
	plugin := New()
	plugin.finder = finder
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	plugin.programs = programs
	return plugin, finder
}

func sample() []Program {
	return []Program{
		{
			Name: "Notepad", FullPath: `C:\Windows\notepad.exe`,
			ParentDirectory: `C:\Windows`, IcoPath: `C:\Windows\notepad.exe`,
			UniqueIdentifier: `C:\Windows\notepad.exe`, Enabled: true,
			Description: "Notepad",
		},
		{
			Name: "Google Chrome", FullPath: `C:\Start Menu\Google Chrome.lnk`,
			LnkResolvedPath: `C:\Program Files\Chrome\chrome.exe`,
			ExecutableName:  "chrome",
			ParentDirectory: `C:\Start Menu`, IcoPath: `C:\Start Menu\Google Chrome.lnk`,
			UniqueIdentifier: `C:\Start Menu\Google Chrome.lnk`, Enabled: true,
		},
		{
			Name: "Uninstall 7-Zip", FullPath: `C:\Start Menu\Uninstall 7-Zip.lnk`,
			LnkResolvedPath: `C:\Program Files\7-Zip\uninstall.exe`,
			ExecutableName:  "uninstall",
			ParentDirectory: `C:\Start Menu`, IcoPath: `C:\Start Menu\Uninstall 7-Zip.lnk`,
			UniqueIdentifier: `C:\Start Menu\Uninstall 7-Zip.lnk`, Enabled: true,
		},
	}
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestQueryFindsAProgramByName(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)
	plugin.settings.HideAppsPath = false

	results, err := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results %v, want one", len(results), titlesOf(results))
	}
	if results[0].Title != "Google Chrome" {
		t.Errorf("the title is %q", results[0].Title)
	}
	if results[0].SubTitle != `C:\Start Menu\Google Chrome.lnk` {
		t.Errorf("the subtitle is %q, want where the program lives", results[0].SubTitle)
	}
	if results[0].AutoCompleteText != "Google Chrome" {
		t.Errorf("AutoCompleteText is %q", results[0].AutoCompleteText)
	}
}

func TestQueryHighlightsTheMatchedCharacters(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrom"})
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if len(results[0].TitleHighlightData) == 0 {
		t.Error("nothing is highlighted")
	}
}

func TestQueryLeavesOutWhatDoesNotMatch(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "zzzz"})
	if len(results) != 0 {
		t.Errorf("got %v, want nothing", titlesOf(results))
	}
}

func TestHideUninstallers(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	plugin.settings.HideUninstallers = false
	results, _ := plugin.Query(context.Background(), core.Query{Search: "uninstall"})
	if len(results) == 0 {
		t.Fatal("the uninstaller was not found although it is not hidden")
	}

	plugin.settings.HideUninstallers = true
	results, _ = plugin.Query(context.Background(), core.Query{Search: "uninstall"})
	if len(results) != 0 {
		t.Errorf("got %v, want the uninstaller left out", titlesOf(results))
	}
}

func TestHiddenProgramsAreLeftOut(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)
	plugin.settings.DisabledProgramSources = []ProgramSource{
		{UniqueIdentifier: `C:\Start Menu\Google Chrome.lnk`, Enabled: false},
	}

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	if len(results) != 0 {
		t.Errorf("got %v, want the hidden program left out", titlesOf(results))
	}
}

func TestHidingAProgramAsksTheLauncherToSearchAgain(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)
	asked := false
	plugin.api.Hooks.ReQuery = func(bool) { asked = true }

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	items := plugin.LoadContextMenus(results[0])

	var hide core.Result
	for _, item := range items {
		if item.Title == plugin.words.T("flowlauncher_plugin_program_disable_program") {
			hide = item
		}
	}
	if hide.Title == "" {
		t.Fatal("the menu does not offer hiding the program")
	}
	hide.Action(core.ActionContext{})

	if !asked {
		t.Error("the launcher was not asked to search again")
	}
	results, _ = plugin.Query(context.Background(), core.Query{Search: "chrome"})
	if len(results) != 0 {
		t.Errorf("got %v, want the program left out after being hidden", titlesOf(results))
	}
}

func TestRunningAProgramAsksTheMachine(t *testing.T) {
	plugin, finder := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	results[0].Action(core.ActionContext{})

	if len(finder.started) != 1 {
		t.Fatalf("the machine was asked to run %d programs, want one", len(finder.started))
	}
	// A shortcut runs what it points at.
	if finder.started[0] != `C:\Program Files\Chrome\chrome.exe` {
		t.Errorf("the program run was %q, want what the shortcut points at", finder.started[0])
	}
	if finder.startedAdmin[0] {
		t.Error("the program was run as an administrator without being asked")
	}
}

func TestCtrlEnterOpensWhereTheProgramLives(t *testing.T) {
	plugin, finder := newTestPlugin(t, sample()...)
	opened := ""
	plugin.api.Hooks.OpenDirectory = func(directory, file string) { opened = directory + "|" + file }

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	results[0].Action(core.ActionContext{SpecialKeyState: core.SpecialKeyState{CtrlPressed: true}})

	if len(finder.started) != 0 {
		t.Error("the program was run although Ctrl was held")
	}
	if opened != `C:\Start Menu|C:\Start Menu\Google Chrome.lnk` {
		t.Errorf("the directory opened was %q", opened)
	}
}

func TestCtrlShiftEnterRunsAsAdministrator(t *testing.T) {
	plugin, finder := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	results[0].Action(core.ActionContext{SpecialKeyState: core.SpecialKeyState{CtrlPressed: true, ShiftPressed: true}})

	if len(finder.started) != 1 || !finder.startedAdmin[0] {
		t.Errorf("the program was run as %+v, want it elevated", finder.startedAdmin)
	}
}

func TestContextMenuOffersRunningItAnotherWay(t *testing.T) {
	plugin, finder := newTestPlugin(t, sample()...)
	plugin.api.Hooks.OpenDirectory = func(string, string) {}

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	items := plugin.LoadContextMenus(results[0])

	// Running it another way, where it lives, where its target lives, and
	// hiding it.
	if len(items) != 5 {
		t.Fatalf("the menu has %d items, want five", len(items))
	}
	if items[0].Title != plugin.words.T("flowlauncher_plugin_program_run_as_different_user") {
		t.Errorf("the first item is %q", items[0].Title)
	}

	items[0].Action(core.ActionContext{})
	if len(finder.asDifferent) != 1 {
		t.Errorf("the machine was asked to run %d programs as another user, want one", len(finder.asDifferent))
	}

	items[1].Action(core.ActionContext{})
	if len(finder.started) != 1 || !finder.startedAdmin[0] {
		t.Errorf("running as administrator asked for %+v", finder.startedAdmin)
	}
}

func TestDescriptionIsShownAndSearched(t *testing.T) {
	programs := []Program{{
		Name: "vlc", Description: "VLC media player",
		FullPath: `C:\Program Files\vlc\vlc.exe`, ParentDirectory: `C:\Program Files\vlc`,
		UniqueIdentifier: "vlc", Enabled: true,
	}}
	plugin, _ := newTestPlugin(t, programs...)
	plugin.settings.EnableDescription = true

	results, _ := plugin.Query(context.Background(), core.Query{Search: "media"})
	if len(results) != 1 {
		t.Fatalf("got %v, want the program found by its description", titlesOf(results))
	}
	if results[0].Title != "vlc: VLC media player" {
		t.Errorf("the title is %q, want the name and the description", results[0].Title)
	}
	// The highlight lands in the description, which is after the name and
	// the ": ".
	for _, index := range results[0].TitleHighlightData {
		if index < len("vlc: ") {
			t.Errorf("the highlight at %d is in the name, want it in the description", index)
		}
	}
}

func TestHideAppsPathHidesWhereAProgramLives(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)
	plugin.settings.HideAppsPath = true

	results, _ := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].SubTitle != "" {
		t.Errorf("the subtitle is %q, want it hidden", results[0].SubTitle)
	}
}

func TestReloadDataLooksTheProgramsUpAgain(t *testing.T) {
	plugin, finder := newTestPlugin(t, sample()...)
	finder.programs = []Program{{Name: "New", FullPath: "C:/new.exe", UniqueIdentifier: "new", Enabled: true}}

	if err := plugin.ReloadData(context.Background()); err != nil {
		t.Fatal(err)
	}
	results, _ := plugin.Query(context.Background(), core.Query{Search: "new"})
	if len(results) != 1 {
		t.Errorf("got %v, want the program that appeared", titlesOf(results))
	}
}

func TestSettingsSuffixesAndProtocols(t *testing.T) {
	settings := DefaultSettings()

	suffixes := settings.Suffixes()
	if !contains(suffixes, "lnk") || !contains(suffixes, "exe") || !contains(suffixes, "appref-ms") {
		t.Errorf("the suffixes are %v, want the built-in ones", suffixes)
	}
	// A URL is looked for while a protocol is asked for, which steam and
	// epic are by default.
	if !contains(suffixes, "url") {
		t.Errorf("the suffixes are %v, want the URL while a protocol is on", suffixes)
	}
	if !contains(settings.Protocols(), "steam://run/") {
		t.Errorf("the protocols are %v, want steam", settings.Protocols())
	}

	// With every protocol turned off, a URL is not looked for.
	settings.BuiltinProtocolsStatus = map[string]bool{"steam": false, "epic": false, "http": false}
	if contains(settings.Suffixes(), "url") {
		t.Errorf("the suffixes are %v, want no URL while no protocol is on", settings.Suffixes())
	}
	if len(settings.Protocols()) != 0 {
		t.Errorf("the protocols are %v, want none", settings.Protocols())
	}

	settings.UseCustomSuffixes = true
	settings.CustomSuffixes = []string{"bat"}
	if !contains(settings.Suffixes(), "bat") {
		t.Errorf("the suffixes are %v, want the custom one", settings.Suffixes())
	}
}

func TestProgramFromAPath(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "example.exe")
	if err := writeFile(executable, "not really a program"); err != nil {
		t.Fatal(err)
	}

	program := programFromPath(executable, nil)
	if program.Name != "example" {
		t.Errorf("the name is %q, want the file's name without its extension", program.Name)
	}
	if program.FullPath != executable {
		t.Errorf("the path is %q", program.FullPath)
	}
	if program.ParentDirectory != directory {
		t.Errorf("the directory is %q, want %q", program.ParentDirectory, directory)
	}
	if !program.Enabled {
		t.Error("the program is not enabled")
	}
}

func TestProgramFromAnInternetShortcut(t *testing.T) {
	directory := t.TempDir()
	shortcut := filepath.Join(directory, "Steam Game.url")
	content := "[InternetShortcut]\nURL=steam://run/12345\nIconFile=C:\\icons\\game.ico\n"
	if err := writeFile(shortcut, content); err != nil {
		t.Fatal(err)
	}

	// A shortcut to a place the user did not ask to search is not a program.
	if program := programFromPath(shortcut, []string{"http://"}); program.FullPath != "" {
		t.Errorf("the shortcut was taken although its protocol is not searched for: %+v", program)
	}

	program := programFromPath(shortcut, []string{"steam://run/"})
	if program.LnkResolvedPath != "steam://run/12345" {
		t.Errorf("the URL is %q", program.LnkResolvedPath)
	}
	if program.IcoPath != `C:\icons\game.ico` {
		t.Errorf("the icon is %q", program.IcoPath)
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
