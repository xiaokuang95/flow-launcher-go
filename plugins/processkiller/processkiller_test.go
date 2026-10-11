package processkiller

import (
	"context"
	"testing"

	"launch/core"
)

// fakeProcesses stands in for the machine, so that a test watches what the
// plugin asked to end rather than ending the machine's own processes.
type fakeProcesses struct {
	running []Process
	killed  []Process
}

func (f *fakeProcesses) Running() ([]Process, error) { return f.running, nil }

func (f *fakeProcesses) Kill(process Process) error {
	f.killed = append(f.killed, process)
	return nil
}

func newTestPlugin(t *testing.T, running ...Process) (*Plugin, *fakeProcesses) {
	t.Helper()

	plugin := New()
	processes := &fakeProcesses{running: running}
	plugin.processes = processes
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	return plugin, processes
}

func sample() []Process {
	return []Process{
		{Name: "notepad", ID: 100, Path: `C:\Windows\notepad.exe`, WindowTitle: "Untitled - Notepad"},
		{Name: "chrome", ID: 200, Path: `C:\Program Files\Chrome\chrome.exe`, WindowTitle: "Google"},
		{Name: "chrome", ID: 201, Path: `C:\Program Files\Chrome\chrome.exe`, WindowTitle: "GitHub"},
		{Name: "background", ID: 300, Path: `C:\tools\background.exe`},
	}
}

func TestQueryListsEveryProcess(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, err := plugin.Query(context.Background(), core.Query{Search: ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("got %d results, want four", len(results))
	}
	// The results are listed by title.
	for i := 1; i < len(results); i++ {
		if results[i-1].Title > results[i].Title {
			t.Errorf("the results are not in order: %q before %q", results[i-1].Title, results[i].Title)
			break
		}
	}
}

func TestQueryShowsTheWindowTitle(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, err := plugin.Query(context.Background(), core.Query{Search: "github"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want one", len(results))
	}
	if results[0].Title != "GitHub" {
		t.Errorf("the title is %q, want the window title", results[0].Title)
	}
	if results[0].SubTitle != `C:\Program Files\Chrome\chrome.exe` {
		t.Errorf("the subtitle is %q, want the path", results[0].SubTitle)
	}
}

func TestQueryFindsAProcessByName(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, err := plugin.Query(context.Background(), core.Query{Search: "notepad"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Untitled - Notepad" {
		t.Fatalf("the results are %v", titlesOf(results))
	}
}

func TestKillingAProcessAsksTheMachine(t *testing.T) {
	plugin, processes := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "github"})
	results[0].Action(core.ActionContext{})

	if len(processes.killed) != 1 {
		t.Fatalf("the machine was asked to end %d processes, want 1", len(processes.killed))
	}
	if processes.killed[0].ID != 201 {
		t.Errorf("the process ended was %d, want 201", processes.killed[0].ID)
	}
}

// TestKillAllIsOfferedWhenEveryResultIsTheSameProgram covers the quick
// option the reference adds: ending every instance of one program at once.
func TestKillAllIsOfferedWhenEveryResultIsTheSameProgram(t *testing.T) {
	plugin, processes := newTestPlugin(t, sample()...)

	results, err := plugin.Query(context.Background(), core.Query{Search: "chrome"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want the two windows and the kill-all", len(results))
	}
	if results[1].Title != plugin.words.Tf("flowlauncher_plugin_processkiller_kill_all", "chrome") {
		t.Errorf("the second result is %q, want the kill-all option", results[1].Title)
	}
	if results[1].Score != 200 {
		t.Errorf("the kill-all option scores %d, want 200", results[1].Score)
	}

	results[1].Action(core.ActionContext{})
	if len(processes.killed) != 2 {
		t.Errorf("the machine was asked to end %d processes, want both of them", len(processes.killed))
	}
}

func TestKillAllIsNotOfferedForDifferentPrograms(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "e"})
	for _, result := range results {
		if result.Title == plugin.words.Tf("flowlauncher_plugin_processkiller_kill_all", "chrome") {
			t.Error("the kill-all option was offered for different programs")
		}
	}
}

func TestVisibleWindowsAreLifted(t *testing.T) {
	plugin, _ := newTestPlugin(t, sample()...)
	plugin.settings.PutVisibleWindowProcessesTop = true

	results, _ := plugin.Query(context.Background(), core.Query{Search: ""})
	for _, result := range results {
		if result.Title == "background - 300" {
			if result.Score != 0 {
				t.Errorf("the process without a window scores %d, want 0", result.Score)
			}
			continue
		}
		if result.Score != 200 {
			t.Errorf("%q scores %d, want it lifted by 200", result.Title, result.Score)
		}
	}
}

func TestContextMenuKillsEveryInstance(t *testing.T) {
	plugin, processes := newTestPlugin(t, sample()...)

	items := plugin.LoadContextMenus(core.NewResult("GitHub", `C:\Program Files\Chrome\chrome.exe`, "", 0))
	if len(items) != 1 {
		t.Fatalf("the menu has %d items, want one", len(items))
	}
	items[0].Action(core.ActionContext{})
	if len(processes.killed) != 2 {
		t.Errorf("the machine was asked to end %d processes, want both instances", len(processes.killed))
	}
}

func TestProcessTitle(t *testing.T) {
	process := Process{Name: "chrome", ID: 42}
	if got := process.Title(); got != "chrome - 42" {
		t.Errorf("Title() = %q, want %q", got, "chrome - 42")
	}
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}
