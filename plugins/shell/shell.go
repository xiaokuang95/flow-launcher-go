package shell

import (
	"context"
	"embed"
	"path/filepath"
	"sort"
	"strings"

	"launch/core"
)

// The shell's own words, which it ships in the languages the reference
// does.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Shell is the command interpreter a command is run in.
type Shell string

// The shells a command can be run in.
const (
	ShellCmd            Shell = "Cmd"
	ShellPowershell     Shell = "PowerShell"
	ShellPwsh           Shell = "Pwsh"
	ShellRunCommand     Shell = "RunCommand"
	ShellCustomTemplate Shell = "CustomTemplate"
)

// Settings is what the user can change about the shell.
type Settings struct {
	// Shell is the interpreter the command is run in.
	Shell Shell `json:"Shell"`
	// ReplaceWinR makes Win+R open the launcher rather than the Run box.
	ReplaceWinR bool `json:"ReplaceWinR"`
	// CloseShellAfterPress waits for a key before the window closes, so
	// that the output of a command that finished can be read.
	CloseShellAfterPress bool `json:"CloseShellAfterPress"`
	// LeaveShellOpen keeps the shell open after the command, as cmd /k
	// does.
	LeaveShellOpen bool `json:"LeaveShellOpen"`
	// RunAsAdministrator runs every command elevated.
	RunAsAdministrator bool `json:"RunAsAdministrator"`
	// UseWindowsTerminal runs the command in Windows Terminal.
	UseWindowsTerminal bool `json:"UseWindowsTerminal"`
	// ShowOnlyMostUsedCMDs lists only the commands run most often.
	ShowOnlyMostUsedCMDs bool `json:"ShowOnlyMostUsedCMDs"`
	// ShowOnlyMostUsedCMDsNumber is how many of them to list.
	ShowOnlyMostUsedCMDsNumber int `json:"ShowOnlyMostUsedCMDsNumber"`
	// CustomTemplateShellConfig is the interpreter of a shell of the
	// user's own.
	CustomTemplateShellConfig CustomTemplate `json:"CustomTemplateShellConfig"`
	// CommandHistory counts how often each command was run, which the
	// plugin lists.
	CommandHistory map[string]int `json:"CommandHistory"`
}

// CustomTemplate is a shell of the user's own: a program, and how the
// command is written into its arguments.
type CustomTemplate struct {
	ExecutablePath    string `json:"ExecutablePath"`
	ArgumentsTemplate string `json:"ArgumentsTemplate"`
}

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{
		Shell:                      ShellCmd,
		RunAsAdministrator:         true,
		ShowOnlyMostUsedCMDsNumber: 5,
		CustomTemplateShellConfig: CustomTemplate{
			ExecutablePath:    "cmd.exe",
			ArgumentsTemplate: "/k \"{command}\"",
		},
		CommandHistory: map[string]int{},
	}
}

// Runner runs a command line. It is what the plugin does to the machine,
// kept behind a field so that a test can watch what it was asked to run
// rather than running it.
type Runner interface {
	// Run starts a command line in the shell the settings name.
	Run(command Command) error
	// RunAsDifferentUser starts one as another user, which asks for a
	// password.
	RunAsDifferentUser(command Command) error
	// ExpandEnvironmentVariables replaces %VAR% with what the machine has,
	// as the shell would.
	ExpandEnvironmentVariables(value string) string
}

// Command is a command line ready to be run.
type Command struct {
	// FileName is the program to run.
	FileName string
	// Arguments are what it is run with.
	Arguments []string
	// WorkingDirectory is where it starts.
	WorkingDirectory string
	// Verb is "runas" to run it elevated, empty otherwise.
	Verb string
	// UseShellExecute starts it through the shell, which is what makes a
	// document open in its own program and an elevation prompt appear.
	UseShellExecute bool
}

// Plugin is the shell plugin: it runs what the user types after `>`.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	settings Settings
	words    *core.PluginTranslator
	runner   Runner
}

// New makes the shell plugin.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "D409510CD0D2481F853690A07E6DC426",
			Name:           "Shell",
			Description:    "Provide executing commands from Flow Launcher",
			Author:         "qianlifeng",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords: []string{">"},
			IcoPath:        "Images/shell.png",
		}},
		runner: systemRunner{},
	}
}

// Init reads the shell's settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")

	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "Shell", "Settings.json")
	if p.settings.Shell == "" {
		p.settings = DefaultSettings()
	}
	if p.settings.ShowOnlyMostUsedCMDsNumber == 0 {
		p.settings.ShowOnlyMostUsedCMDsNumber = 5
	}
	if p.settings.CommandHistory == nil {
		p.settings.CommandHistory = map[string]int{}
	}
	return nil
}

// Query answers what the user typed after the keyword: the command itself,
// what they ran before, and the paths of the directory they are typing in.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	command := query.Search
	if strings.TrimSpace(command) == "" {
		return p.resultsFromHistory(), nil
	}

	results := []core.Result{p.currentCommand(command)}
	results = append(results, p.historyCommands(command)...)
	results = append(results, p.pathCompletions(command, results)...)
	return results, nil
}

// currentCommand is the command as the user typed it, which runs when they
// press Enter.
func (p *Plugin) currentCommand(command string) core.Result {
	result := core.NewResult(command, p.words.T("flowlauncher_plugin_cmd_execute_through_shell"), "Images/shell.png", 5000)
	result.SetCopyText(command)
	result.Action = func(action core.ActionContext) bool {
		p.run(command, runAsAdministrator(action.SpecialKeyState))
		return true
	}
	return result
}

// historyCommands are the commands the user ran before that contain what
// they are typing, most used first.
func (p *Plugin) historyCommands(command string) []core.Result {
	type entry struct {
		command string
		count   int
	}

	var entries []entry
	for previous, count := range p.settings.CommandHistory {
		if previous != command && strings.Contains(previous, command) {
			entries = append(entries, entry{previous, count})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].command < entries[j].command
	})

	if p.settings.ShowOnlyMostUsedCMDs && len(entries) > p.settings.ShowOnlyMostUsedCMDsNumber {
		entries = entries[:p.settings.ShowOnlyMostUsedCMDsNumber]
	}

	results := make([]core.Result, 0, len(entries))
	for _, entry := range entries {
		command := entry.command
		result := core.NewResult(command,
			p.words.Tf("flowlauncher_plugin_cmd_cmd_has_been_executed_times", itoa(entry.count)),
			"Images/shell.png", 0)
		result.SetCopyText(command)
		result.Action = func(action core.ActionContext) bool {
			p.run(command, runAsAdministrator(action.SpecialKeyState))
			return true
		}
		results = append(results, result)
	}
	return results
}

// resultsFromHistory lists the commands run most often, which is what the
// plugin shows before anything is typed after the keyword.
func (p *Plugin) resultsFromHistory() []core.Result {
	type entry struct {
		command string
		count   int
	}

	var entries []entry
	for command, count := range p.settings.CommandHistory {
		entries = append(entries, entry{command, count})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].command < entries[j].command
	})

	if p.settings.ShowOnlyMostUsedCMDs && len(entries) > p.settings.ShowOnlyMostUsedCMDsNumber {
		entries = entries[:p.settings.ShowOnlyMostUsedCMDsNumber]
	}

	results := make([]core.Result, 0, len(entries))
	for _, entry := range entries {
		command := entry.command
		result := core.NewResult(command,
			p.words.Tf("flowlauncher_plugin_cmd_cmd_has_been_executed_times", itoa(entry.count)),
			"Images/shell.png", 0)
		result.SetCopyText(command)
		result.Action = func(action core.ActionContext) bool {
			p.run(command, runAsAdministrator(action.SpecialKeyState))
			return true
		}
		results = append(results, result)
	}
	return results
}

// pathCompletions are the files and directories of the path being typed,
// which is how the shell plugin completes a path.
func (p *Plugin) pathCompletions(command string, existing []core.Result) []core.Result {
	expanded := p.runner.ExpandEnvironmentVariables(command)

	directory := ""
	prefix := ""
	switch {
	case strings.HasSuffix(expanded, "/") || strings.HasSuffix(expanded, "\\"):
		directory = expanded
		prefix = command
	default:
		parent := filepath.Dir(expanded)
		if parent == "." || parent == "" {
			return nil
		}
		if !core.DirectoryExists(parent) {
			return nil
		}
		directory = parent
		// The part of the path the user has typed, which the completion
		// carries on from.
		prefix = command[:len(command)-len(filepath.Base(expanded))]
	}

	entries, err := readDirectory(directory)
	if err != nil {
		return nil
	}

	var results []core.Result
	for _, entry := range entries {
		title := prefix + entry
		if !strings.HasPrefix(strings.ToLower(title), strings.ToLower(command)) {
			continue
		}
		if alreadyListed(existing, title) {
			continue
		}
		completion := title
		result := core.NewResult(title, "", "Images/shell.png", 0)
		result.SetCopyText(completion)
		result.Action = func(action core.ActionContext) bool {
			p.run(completion, runAsAdministrator(action.SpecialKeyState))
			return true
		}
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Title < results[j].Title })
	return results
}

func alreadyListed(results []core.Result, title string) bool {
	for _, result := range results {
		if strings.EqualFold(result.Title, title) {
			return true
		}
	}
	return false
}

// run starts a command line and remembers that it was run.
func (p *Plugin) run(command string, elevated bool) {
	prepared, err := p.prepare(command, elevated)
	if err != nil {
		p.api.ShowMsgError(p.words.T("flowlauncher_plugin_cmd_plugin_name"), err.Error())
		return
	}

	p.rememberCommand(command)
	if err := p.runner.Run(prepared); err != nil {
		p.api.ShowMsgError(p.words.T("flowlauncher_plugin_cmd_plugin_name"),
			p.words.Tf("flowlauncher_plugin_cmd_error_running_command", err.Error()))
	}
}

// rememberCommand counts the command, which is what the history lists.
func (p *Plugin) rememberCommand(command string) {
	if p.settings.CommandHistory == nil {
		p.settings.CommandHistory = map[string]int{}
	}
	p.settings.CommandHistory[command]++
	_ = core.SaveSettingJsonStorage(p.api, "Shell", "Settings.json", p.settings)
}

// Prepare builds the command line for a shell, which is what the reference's
// CreateProcessStartInfo does.
func (p *Plugin) prepare(command string, elevated bool) (Command, error) {
	return Prepare(command, p.settings, elevated, p.words.T("flowlauncher_plugin_cmd_press_any_key_to_close"))
}

// LoadContextMenus adds the items the shell plugin offers on a result: run
// it as another user, as an administrator, or copy it.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	command := selectedResult.Title

	differentUser := core.NewResult(p.words.T("flowlauncher_plugin_cmd_run_as_different_user"), "", "Images/user.png", 0)
	differentUser.Glyph = &core.GlyphInfo{FontFamily: "/Resources/#Segoe Fluent Icons", Glyph: "\ue7ee"}
	differentUser.Action = func(core.ActionContext) bool {
		prepared, err := p.prepare(command, false)
		if err != nil {
			return true
		}
		p.rememberCommand(command)
		_ = p.runner.RunAsDifferentUser(prepared)
		return true
	}

	administrator := core.NewResult(p.words.T("flowlauncher_plugin_cmd_run_as_administrator"), "", "Images/admin.png", 0)
	administrator.Glyph = &core.GlyphInfo{FontFamily: "/Resources/#Segoe Fluent Icons", Glyph: "\ue7ef"}
	administrator.Action = func(core.ActionContext) bool {
		p.run(command, true)
		return true
	}

	copyItem := core.NewResult(p.words.T("flowlauncher_plugin_cmd_copy"), "", "Images/copy.png", 0)
	copyItem.Glyph = &core.GlyphInfo{FontFamily: "/Resources/#Segoe Fluent Icons", Glyph: "\ue8c8"}
	copyItem.Action = func(core.ActionContext) bool {
		p.api.CopyToClipboard(command, false, true)
		return true
	}

	return []core.Result{differentUser, administrator, copyItem}
}

// runAsAdministrator reports whether the keys held ask for an elevated run,
// which is Ctrl+Shift with nothing else.
func runAsAdministrator(state core.SpecialKeyState) bool {
	return state.CtrlPressed && state.ShiftPressed && !state.AltPressed && !state.WinPressed
}

// itoa is strconv.Itoa, kept out of the imports for one use.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	if negative {
		return "-" + digits
	}
	return digits
}
