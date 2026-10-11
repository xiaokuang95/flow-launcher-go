package sys

import (
	"context"
	"errors"
	"testing"

	"launch/core"
)

// fakeMachine stands in for the machine, so that a test watches what a
// command asked for rather than turning the machine off. Nothing in these
// tests may lock the screen, sleep the machine or turn it off.
type fakeMachine struct {
	asked []string
	err   error
}

func (f *fakeMachine) note(what string) error {
	f.asked = append(f.asked, what)
	return f.err
}

func (f *fakeMachine) ShutDown() error                  { return f.note("shutdown") }
func (f *fakeMachine) Restart() error                   { return f.note("restart") }
func (f *fakeMachine) RestartToFirmwareSettings() error { return f.note("restart-to-firmware") }
func (f *fakeMachine) LogOff() error                    { return f.note("logoff") }
func (f *fakeMachine) Lock() error                      { return f.note("lock") }
func (f *fakeMachine) Sleep() error                     { return f.note("sleep") }
func (f *fakeMachine) Hibernate() error                 { return f.note("hibernate") }
func (f *fakeMachine) OpenIndexingOptions() error       { return f.note("indexing-options") }
func (f *fakeMachine) EmptyRecycleBin() error           { return f.note("empty-recycle-bin") }

// asked is what the plugin asked the machine to do, for the actions that
// do something to the machine itself.
func (f *fakeMachine) askedFor(what string) bool {
	for _, entry := range f.asked {
		if entry == what {
			return true
		}
	}
	return false
}

// fakeAPI remembers what the plugin asked the launcher to do.
type fakeAPI struct {
	hidden      bool
	saved       bool
	settings    bool
	restarted   bool
	quit        bool
	reloaded    bool
	updated     bool
	gameMode    bool
	query       string
	directories []string
	urls        []string
	answers     core.MessageBoxResult
	messages    []string
}

func newTestPlugin(t *testing.T) (*Plugin, *fakeMachine, *fakeAPI) {
	t.Helper()

	machine := &fakeMachine{}
	launcher := &fakeAPI{answers: core.MessageBoxResultYes}

	api := core.NewPublicAPI(nil, t.TempDir(), "test")
	api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	api.Hooks.HideMainWindow = func() { launcher.hidden = true }
	api.Hooks.SaveAppAllSettings = func() { launcher.saved = true }
	api.Hooks.OpenSettingDialog = func() { launcher.settings = true }
	api.Hooks.RestartApp = func() { launcher.restarted = true }
	api.Hooks.Exit = func() { launcher.quit = true }
	api.Hooks.ReloadAllPluginData = func() error { launcher.reloaded = true; return nil }
	api.Hooks.CheckForNewUpdate = func() { launcher.updated = true }
	api.Hooks.ToggleGameMode = func() { launcher.gameMode = true }
	api.Hooks.ChangeQuery = func(query string, requery bool) { launcher.query = query }
	api.Hooks.OpenDirectory = func(directory, file string) { launcher.directories = append(launcher.directories, directory) }
	api.Hooks.OpenURL = func(rawURL string, inPrivate *bool) { launcher.urls = append(launcher.urls, rawURL) }
	api.Hooks.ShowMsgBox = func(text, caption string, buttons core.MessageBoxButtons, icon core.MessageBoxIcon) core.MessageBoxResult {
		return launcher.answers
	}
	api.Hooks.ShowMsg = func(title, subTitle, iconPath string) {
		launcher.messages = append(launcher.messages, title+"|"+subTitle)
	}
	api.Hooks.ShowMsgError = func(title, subTitle string) {
		launcher.messages = append(launcher.messages, "error|"+title+"|"+subTitle)
	}
	api.Hooks.LogDirectory = func() string { return "C:/logs" }

	plugin := New()
	plugin.machine = machine
	plugin.api = api
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	plugin.nameCommands()
	return plugin, machine, launcher
}

func TestEveryCommandIsListed(t *testing.T) {
	plugin, _, _ := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(commandOrder) {
		t.Fatalf("got %d commands, want %d", len(results), len(commandOrder))
	}
	for _, result := range results {
		if result.Title == "" || result.SubTitle == "" {
			t.Errorf("the command %+v has no words", result)
		}
	}
}

func TestACommandIsFoundByItsNameAndItsKeyword(t *testing.T) {
	plugin, _, _ := newTestPlugin(t)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "lock"})
	if len(results) == 0 {
		t.Fatal("no results for lock")
	}
	if results[0].SubTitle == "" {
		t.Error("the command has no description")
	}
}

// TestPowerCommandsAskTheMachine covers the commands that turn the machine
// off and on: they must ask the machine through the field the test fills,
// and must never call the system themselves.
func TestPowerCommandsAskTheMachine(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"Shutdown", "shutdown"},
		{"Restart", "restart"},
		{"Restart With Advanced Boot Options", "restart-to-firmware"},
		{"Log Off/Sign Out", "logoff"},
		{"Lock", "lock"},
		{"Sleep", "sleep"},
		{"Hibernate", "hibernate"},
		{"Index Option", "indexing-options"},
	}

	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			plugin, machine, launcher := newTestPlugin(t)
			// The user has said not to be asked.
			plugin.settings.SkipPowerActionConfirmation = true

			result := resultFor(t, plugin, c.key)
			result.Action(core.ActionContext{})

			if !machine.askedFor(c.want) {
				t.Errorf("the command asked the machine for %v, want %q", machine.asked, c.want)
			}
			_ = launcher
		})
	}
}

func TestPowerCommandsAskBeforeTurningTheMachineOff(t *testing.T) {
	plugin, machine, launcher := newTestPlugin(t)
	// The user has not said not to be asked, and says no.
	launcher.answers = core.MessageBoxResultNo

	result := resultFor(t, plugin, "Shutdown")
	result.Action(core.ActionContext{})

	if len(machine.asked) != 0 {
		t.Errorf("the machine was asked to %v although the user said no", machine.asked)
	}
}

func TestEmptyRecycleBinAsksTheMachine(t *testing.T) {
	plugin, machine, _ := newTestPlugin(t)

	result := resultFor(t, plugin, "Empty Recycle Bin")
	result.Action(core.ActionContext{})

	if !machine.askedFor("empty-recycle-bin") {
		t.Errorf("the machine was asked for %v, want the recycle bin emptied", machine.asked)
	}
}

func TestSettingsCommandsAskTheLauncher(t *testing.T) {
	cases := []struct {
		key   string
		check func(*fakeAPI) bool
		what  string
	}{
		{"Settings", func(l *fakeAPI) bool { return l.settings }, "the settings window"},
		{"Save Settings", func(l *fakeAPI) bool { return l.saved }, "the settings saved"},
		{"Restart Flow Launcher", func(l *fakeAPI) bool { return l.restarted }, "the launcher restarted"},
		{"Exit", func(l *fakeAPI) bool { return l.quit }, "the launcher quit"},
		{"Reload Plugin Data", func(l *fakeAPI) bool { return l.reloaded }, "the plugins reloaded"},
		{"Check For Update", func(l *fakeAPI) bool { return l.updated }, "an update checked for"},
		{"Toggle Game Mode", func(l *fakeAPI) bool { return l.gameMode }, "game mode toggled"},
	}

	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			plugin, _, launcher := newTestPlugin(t)

			result := resultFor(t, plugin, c.key)
			result.Action(core.ActionContext{})

			if !c.check(launcher) {
				t.Errorf("the command did not ask for %s", c.what)
			}
		})
	}
}

func TestOpenCommandsAskTheLauncher(t *testing.T) {
	plugin, _, launcher := newTestPlugin(t)

	resultFor(t, plugin, "Open Log Location").Action(core.ActionContext{})
	if len(launcher.directories) != 1 || launcher.directories[0] != "C:/logs" {
		t.Errorf("the launcher was asked to open %v, want the log folder", launcher.directories)
	}

	resultFor(t, plugin, "Flow Launcher Tips").Action(core.ActionContext{})
	if len(launcher.urls) != 1 || launcher.urls[0] != documentation {
		t.Errorf("the launcher was asked to open %v, want the tips", launcher.urls)
	}
}

func TestTheThemeSelectorIsTypedIntoTheQueryBox(t *testing.T) {
	plugin, _, launcher := newTestPlugin(t)

	result := resultFor(t, plugin, "Set Flow Launcher Theme")
	hide := result.Action(core.ActionContext{})

	if hide {
		t.Error("the launcher hid, want it to stay up")
	}
	if launcher.query != themeKeyword+" " {
		t.Errorf("the query box holds %q, want the theme keyword", launcher.query)
	}
}

func TestACommandTheMachineRefusedSaysSo(t *testing.T) {
	plugin, machine, launcher := newTestPlugin(t)
	plugin.settings.SkipPowerActionConfirmation = true
	machine.err = errors.New("the machine said no")

	resultFor(t, plugin, "Lock").Action(core.ActionContext{})

	found := false
	for _, message := range launcher.messages {
		if len(message) > 5 && message[:5] == "error" {
			found = true
		}
	}
	if !found {
		t.Errorf("the launcher was told %v, want it told the command failed", launcher.messages)
	}
}

func TestTheUserCanRenameACommand(t *testing.T) {
	plugin, _, _ := newTestPlugin(t)
	for i := range plugin.settings.Commands {
		if plugin.settings.Commands[i].Key == "Lock" {
			plugin.settings.Commands[i].Name = "Lock the screen"
			plugin.settings.Commands[i].Keyword = "screensaver"
		}
	}

	results, _ := plugin.Query(context.Background(), core.Query{Search: "screensaver"})
	if len(results) == 0 {
		t.Fatal("the command was not found by its new keyword")
	}
	if results[0].Title != "Lock the screen" {
		t.Errorf("the title is %q, want the name the user gave it", results[0].Title)
	}
}

// resultFor finds a command's result by its key, however it is translated.
func resultFor(t *testing.T, plugin *Plugin, key string) core.Result {
	t.Helper()

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		for _, command := range plugin.settings.Commands {
			if command.Key == key && result.Title == command.Name {
				return result
			}
		}
	}
	t.Fatalf("the command %q is not among the %d results", key, len(results))
	return core.Result{}
}
