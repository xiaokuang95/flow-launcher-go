package shell

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"launch/core"
)

// fakeRunner stands in for the machine, so that a test watches what the
// plugin asked to run rather than running it.
type fakeRunner struct {
	ran            []Command
	ranAsOtherUser []Command
	expanded       string
}

func (f *fakeRunner) Run(command Command) error {
	f.ran = append(f.ran, command)
	return nil
}

func (f *fakeRunner) RunAsDifferentUser(command Command) error {
	f.ranAsOtherUser = append(f.ranAsOtherUser, command)
	return nil
}

func (f *fakeRunner) ExpandEnvironmentVariables(value string) string {
	if f.expanded != "" {
		return f.expanded
	}
	return value
}

// newTestPlugin makes a plugin over a runner that runs nothing.
func newTestPlugin(t *testing.T) (*Plugin, *fakeRunner) {
	t.Helper()

	plugin := New()
	runner := &fakeRunner{}
	plugin.runner = runner
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	plugin.settings.RunAsAdministrator = false
	return plugin, runner
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestQueryAnswersTheCommandItself(t *testing.T) {
	plugin, runner := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{Search: "echo hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].Title != "echo hello" {
		t.Errorf("the first result is %q, want the command itself", results[0].Title)
	}
	if results[0].Score != 5000 {
		t.Errorf("the command scores %d, want 5000", results[0].Score)
	}
	if results[0].CopyText() != "echo hello" {
		t.Errorf("CopyText is %q", results[0].CopyText())
	}

	// Running it asks the machine to run it.
	results[0].Action(core.ActionContext{})
	if len(runner.ran) != 1 {
		t.Fatalf("the machine was asked to run %d commands, want 1", len(runner.ran))
	}
	if runner.ran[0].FileName != "cmd.exe" {
		t.Errorf("the command runs in %q, want cmd.exe", runner.ran[0].FileName)
	}
	if len(runner.ran[0].Arguments) == 0 || runner.ran[0].Arguments[0] != "/c" {
		t.Errorf("the arguments are %v, want them to start with /c", runner.ran[0].Arguments)
	}
}

func TestQueryListsTheCommandsRunBefore(t *testing.T) {
	plugin, _ := newTestPlugin(t)
	plugin.settings.CommandHistory = map[string]int{
		"git status": 3,
		"git log":    7,
		"dir":        1,
	}

	results, err := plugin.Query(context.Background(), core.Query{Search: "git"})
	if err != nil {
		t.Fatal(err)
	}

	titles := titlesOf(results)
	if titles[0] != "git" {
		t.Errorf("the first result is %q, want the command itself", titles[0])
	}
	// The most used of the ones that contain what was typed comes first.
	if len(titles) < 3 || titles[1] != "git log" || titles[2] != "git status" {
		t.Errorf("the results are %v, want the history after the command itself", titles)
	}
}

func TestHistoryListsMostUsedFirst(t *testing.T) {
	plugin, _ := newTestPlugin(t)
	plugin.settings.CommandHistory = map[string]int{"a": 1, "b": 5, "c": 3}

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got := titlesOf(results); len(got) != 3 || got[0] != "b" || got[1] != "c" || got[2] != "a" {
		t.Errorf("the history is %v, want b, c, a", got)
	}
}

func TestHistoryCanBeLimitedToTheMostUsed(t *testing.T) {
	plugin, _ := newTestPlugin(t)
	plugin.settings.ShowOnlyMostUsedCMDs = true
	plugin.settings.ShowOnlyMostUsedCMDsNumber = 2
	plugin.settings.CommandHistory = map[string]int{"a": 1, "b": 5, "c": 3}

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got := titlesOf(results); len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Errorf("the history is %v, want the two most used", got)
	}
}

func TestRunningACommandRemembersIt(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	plugin.run("echo hello", false)

	if plugin.settings.CommandHistory["echo hello"] != 1 {
		t.Errorf("the command was remembered %d times, want 1",
			plugin.settings.CommandHistory["echo hello"])
	}
	plugin.run("echo hello", false)
	if plugin.settings.CommandHistory["echo hello"] != 2 {
		t.Errorf("the command was remembered %d times, want 2",
			plugin.settings.CommandHistory["echo hello"])
	}
}

func TestCtrlShiftRunsAsAdministrator(t *testing.T) {
	plugin, runner := newTestPlugin(t)
	results, err := plugin.Query(context.Background(), core.Query{Search: "echo hello"})
	if err != nil {
		t.Fatal(err)
	}

	results[0].Action(core.ActionContext{SpecialKeyState: core.SpecialKeyState{CtrlPressed: true, ShiftPressed: true}})

	if len(runner.ran) != 1 {
		t.Fatalf("the machine was asked to run %d commands", len(runner.ran))
	}
	if runner.ran[0].Verb != "runas" {
		t.Errorf("the command was run with verb %q, want runas", runner.ran[0].Verb)
	}
}

func TestCtrlShiftAltDoesNotElevate(t *testing.T) {
	plugin, runner := newTestPlugin(t)
	results, _ := plugin.Query(context.Background(), core.Query{Search: "echo hello"})

	results[0].Action(core.ActionContext{SpecialKeyState: core.SpecialKeyState{
		CtrlPressed: true, ShiftPressed: true, AltPressed: true,
	}})

	if len(runner.ran) != 1 {
		t.Fatalf("the machine was asked to run %d commands", len(runner.ran))
	}
	if runner.ran[0].Verb == "runas" {
		t.Error("the command was elevated although Alt was held")
	}
}

// TestPrepareForEachShell checks what each interpreter is given, which is
// what the reference's CreateProcessStartInfo does.
func TestPrepareForEachShell(t *testing.T) {
	cases := []struct {
		name     string
		settings Settings
		want     string
		wantArgs []string
	}{
		{
			name:     "cmd closes after the command",
			settings: Settings{Shell: ShellCmd},
			want:     "cmd.exe",
			wantArgs: []string{"/c", "echo hello"},
		},
		{
			name:     "cmd stays open",
			settings: Settings{Shell: ShellCmd, LeaveShellOpen: true},
			want:     "cmd.exe",
			wantArgs: []string{"/k", "echo hello"},
		},
		{
			name:     "cmd waits for a key",
			settings: Settings{Shell: ShellCmd, CloseShellAfterPress: true},
			want:     "cmd.exe",
			wantArgs: []string{"/c", "echo hello && echo press any key && pause > nul"},
		},
		{
			name:     "powershell closes after the command",
			settings: Settings{Shell: ShellPowershell},
			want:     "powershell.exe",
			wantArgs: []string{"-Command", "echo hello;"},
		},
		{
			name:     "pwsh stays open",
			settings: Settings{Shell: ShellPwsh, LeaveShellOpen: true},
			want:     "pwsh.exe",
			wantArgs: []string{"-NoExit", "echo hello"},
		},
		{
			name:     "windows terminal",
			settings: Settings{Shell: ShellCmd, UseWindowsTerminal: true},
			want:     "wt.exe",
			wantArgs: []string{"cmd", "/c", "echo hello"},
		},
		{
			name:     "run finds a program in the path",
			settings: Settings{Shell: ShellRunCommand},
			want:     "cmd.exe",
			wantArgs: []string{"/c echo hi"},
		},
		{
			name:     "run passes on a program it cannot find",
			settings: Settings{Shell: ShellRunCommand},
			want:     "no-such-program-4f2a1c --flag",
			wantArgs: nil,
		},
		{
			name: "a shell of the user's own",
			settings: Settings{Shell: ShellCustomTemplate, CustomTemplateShellConfig: CustomTemplate{
				ExecutablePath:    "C:/tools/bash.exe",
				ArgumentsTemplate: "-c \"{command}\"",
			}},
			want:     "C:/tools/bash.exe",
			wantArgs: []string{"-c \"echo hello\""},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			commandLine := "echo hello"
			if c.settings.Shell == ShellRunCommand {
				if c.wantArgs == nil {
					commandLine = c.want
				} else {
					commandLine = "cmd.exe /c echo hi"
				}
			}
			command, err := Prepare(commandLine, c.settings, false, "press any key")
			if err != nil {
				t.Fatal(err)
			}
			if command.FileName != c.want {
				t.Errorf("the program is %q, want %q", command.FileName, c.want)
			}
			if strings.Join(command.Arguments, "|") != strings.Join(c.wantArgs, "|") {
				t.Errorf("the arguments are %v, want %v", command.Arguments, c.wantArgs)
			}
		})
	}
}

func TestPrepareRunsAsAdministratorByDefault(t *testing.T) {
	command, err := Prepare("echo hello", Settings{Shell: ShellCmd, RunAsAdministrator: true}, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if command.Verb != "runas" {
		t.Errorf("the verb is %q, want runas", command.Verb)
	}
}

func TestSplitRunCommand(t *testing.T) {
	cases := []struct {
		command   string
		filename  string
		arguments string
	}{
		{"notepad.exe file.txt", "notepad.exe", "file.txt"},
		{"notepad.exe", "notepad.exe", ""},
		{`"C:\Program Files\app.exe" --flag`, `C:\Program Files\app.exe`, " --flag"},
	}
	for _, c := range cases {
		filename, arguments := splitRunCommand(c.command)
		if filename != c.filename || arguments != c.arguments {
			t.Errorf("splitRunCommand(%q) = %q, %q, want %q, %q",
				c.command, filename, arguments, c.filename, c.arguments)
		}
	}
}

func TestPathCompletions(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		if err := writeFile(filepath.Join(directory, name), "x"); err != nil {
			t.Fatal(err)
		}
	}

	plugin, _ := newTestPlugin(t)
	typed := filepath.Join(directory, "a")
	results := plugin.pathCompletions(typed, nil)

	if len(results) != 1 {
		t.Fatalf("got %d completions %v, want one", len(results), titlesOf(results))
	}
	if filepath.Base(results[0].Title) != "alpha.txt" {
		t.Errorf("the completion is %q, want alpha.txt", results[0].Title)
	}
}

func TestContextMenuOffersRunningAsAnotherUser(t *testing.T) {
	plugin, runner := newTestPlugin(t)

	items := plugin.LoadContextMenus(core.NewResult("echo hello", "", "", 0))
	if len(items) != 3 {
		t.Fatalf("the menu has %d items, want three", len(items))
	}
	if items[0].Title != plugin.words.T("flowlauncher_plugin_cmd_run_as_different_user") {
		t.Errorf("the first item is %q", items[0].Title)
	}

	items[0].Action(core.ActionContext{})
	if len(runner.ranAsOtherUser) != 1 {
		t.Errorf("the machine was asked to run %d commands as another user, want 1", len(runner.ranAsOtherUser))
	}

	items[1].Action(core.ActionContext{})
	if len(runner.ran) != 1 || runner.ran[0].Verb != "runas" {
		t.Errorf("running as administrator asked for %+v", runner.ran)
	}
}

func TestExpandWindowsVariables(t *testing.T) {
	t.Setenv("LAUNCH_TEST_VALUE", "hello")

	if got := expandWindowsVariables("%LAUNCH_TEST_VALUE% world"); got != "hello world" {
		t.Errorf("expandWindowsVariables = %q, want %q", got, "hello world")
	}
	if got := expandWindowsVariables("no variables"); got != "no variables" {
		t.Errorf("expandWindowsVariables = %q", got)
	}
	if got := expandWindowsVariables("half %VAR"); got != "half %VAR" {
		t.Errorf("expandWindowsVariables = %q", got)
	}
}
