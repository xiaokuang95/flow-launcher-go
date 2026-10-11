package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The JSON-RPC plugin host: how a plugin written in another language is
// asked what it found.
//
// A plugin of this kind is a program of its own. The launcher runs it once
// for each request, hands it the request as the last argument, and reads
// what it answers on its standard output. The request and the answer are
// JSON, in the shape the reference uses:
//
//	{"id":1,"method":"query","parameters":["what was typed"],"settings":{...}}
//	{"id":1,"result":[{"title":"...","subTitle":"...","jsonRPCAction":{...}}]}
//
// A plugin that cannot be reached, or that answers with something else,
// is left out of the results rather than stopping the launcher.

// JSONRPCRequest is what a plugin is asked.
type JSONRPCRequest struct {
	ID         int            `json:"id"`
	Method     string         `json:"method"`
	Parameters []any          `json:"parameters"`
	Settings   map[string]any `json:"settings,omitempty"`
	Error      *JSONRPCError  `json:"error,omitempty"`
}

// JSONRPCError is what a plugin answers when it could not do what it was
// asked.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

// JSONRPCResponse is what a plugin answers with, whether it was asked for
// results or for something to be done.
type JSONRPCResponse struct {
	ID             int             `json:"id"`
	Result         []JSONRPCResult `json:"result"`
	SettingsChange map[string]any  `json:"settingsChange"`
	DebugMessage   string          `json:"debugMessage"`
	Error          *JSONRPCError   `json:"error"`
}

// JSONRPCResult is one result a plugin answered with, which is the
// launcher's own result plus the action to take when it is chosen.
type JSONRPCResult struct {
	Title        string `json:"title"`
	SubTitle     string `json:"subTitle"`
	IcoPath      string `json:"icoPath"`
	BadgeIcoPath string `json:"badgeIcoPath"`
	Glyph        *struct {
		Glyph      string `json:"glyph"`
		FontFamily string `json:"fontFamily"`
	} `json:"glyph"`
	RoundedIcon           bool   `json:"roundedIcon"`
	ShowBadge             bool   `json:"showBadge"`
	TitleHighlightData    []int  `json:"titleHighlightData"`
	CopyText              string `json:"copyText"`
	AutoCompleteText      string `json:"autoCompleteText"`
	QuerySuggestionText   string `json:"querySuggestionText"`
	TitleToolTip          string `json:"titleToolTip"`
	SubTitleToolTip       string `json:"subTitleToolTip"`
	Score                 int    `json:"score"`
	ProgressBar           *int   `json:"progressBar"`
	ProgressBarColor      string `json:"progressBarColor"`
	ActionKeywordAssigned string `json:"actionKeywordAssigned"`
	RecordKey             string `json:"recordKey"`
	AddSelectedCount      *bool  `json:"addSelectedCount"`
	ContextData           any    `json:"contextData"`

	// Action is what the plugin is asked to do when the result is chosen.
	Action *JSONRPCAction `json:"jsonRPCAction"`
	// SettingsChange is what the plugin wants remembered after the result
	// is chosen.
	SettingsChange map[string]any `json:"settingsChange"`
}

// JSONRPCAction is what a plugin is asked to do when a result is chosen.
type JSONRPCAction struct {
	ID         int    `json:"id"`
	Method     string `json:"method"`
	Parameters []any  `json:"parameters"`
	// DontHideAfterAction keeps the launcher up after the action.
	DontHideAfterAction bool `json:"dontHideAfterAction"`
}

// JSONRPCPlugin is a plugin that runs as a program of its own.
type JSONRPCPlugin struct {
	// Meta is what the plugin says about itself, read from its plugin.json.
	Meta PluginMetadata
	// Directory is where the plugin lives.
	Directory string
	// Runner runs the plugin and returns what it answered.
	Runner Runner

	mu     sync.Mutex
	nextID int
	// settings are what the launcher remembers for the plugin, which it is
	// given with every request.
	settings map[string]any
}

// Runner runs a plugin's program and returns what it wrote on its
// standard output.
type Runner interface {
	// Run runs the plugin with a request and returns its answer.
	Run(ctx context.Context, directory string, request []byte) ([]byte, error)
}

// NewJSONRPCPlugin makes a plugin over a program of its own.
func NewJSONRPCPlugin(metadata PluginMetadata, directory string, runner Runner) *JSONRPCPlugin {
	return &JSONRPCPlugin{
		Meta:      metadata,
		Directory: directory,
		Runner:    runner,
		settings:  map[string]any{},
	}
}

// Metadata is what the launcher knows about the plugin.
func (p *JSONRPCPlugin) Metadata() PluginMetadata { return p.Meta }

// Init reads what the launcher remembers for the plugin.
func (p *JSONRPCPlugin) Init(ctx *PluginInitContext) error {
	path := filepath.Join(ctx.CurrentPluginMetadata.PluginSettingsDirectoryPath, "Settings.json")
	if content := mustReadFile(path); len(content) > 0 {
		var settings map[string]any
		if err := json.Unmarshal(content, &settings); err == nil {
			p.mu.Lock()
			p.settings = settings
			p.mu.Unlock()
		}
	}
	return nil
}

// Query asks the plugin what it found.
func (p *JSONRPCPlugin) Query(ctx context.Context, query Query) ([]Result, error) {
	response, err := p.request(ctx, "query", []any{query.Search})
	if err != nil {
		return nil, err
	}
	return p.resultsFrom(response), nil
}

// LoadContextMenus asks the plugin for the menu of a result.
func (p *JSONRPCPlugin) LoadContextMenus(selectedResult Result) []Result {
	response, err := p.request(context.Background(), "context_menu", []any{selectedResult.ContextData})
	if err != nil {
		return nil
	}
	return p.resultsFrom(response)
}

// request asks the plugin something and reads what it answered.
func (p *JSONRPCPlugin) request(ctx context.Context, method string, parameters []any) (JSONRPCResponse, error) {
	p.mu.Lock()
	p.nextID++
	request := JSONRPCRequest{
		ID:         p.nextID,
		Method:     method,
		Parameters: parameters,
		Settings:   p.settings,
	}
	p.mu.Unlock()

	serialized, err := json.Marshal(request)
	if err != nil {
		return JSONRPCResponse{}, err
	}

	answer, err := p.Runner.Run(ctx, p.Directory, serialized)
	if err != nil {
		return JSONRPCResponse{}, err
	}
	if len(answer) == 0 {
		return JSONRPCResponse{}, errEmptyAnswer
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(answer, &response); err != nil {
		return JSONRPCResponse{}, fmt.Errorf("the plugin answered something else: %w", err)
	}
	if response.Error != nil {
		return response, fmt.Errorf("the plugin answered with an error: %s", response.Error.Message)
	}

	// What the plugin wants remembered is remembered for the next request.
	if len(response.SettingsChange) > 0 {
		p.mu.Lock()
		for key, value := range response.SettingsChange {
			p.settings[key] = value
		}
		p.mu.Unlock()
	}
	if response.DebugMessage != "" {
		Logf("%s: %s", p.Meta.Name, response.DebugMessage)
	}
	return response, nil
}

// resultsFrom turns what the plugin answered into results.
func (p *JSONRPCPlugin) resultsFrom(response JSONRPCResponse) []Result {
	results := make([]Result, 0, len(response.Result))
	for i := range response.Result {
		results = append(results, p.resultFrom(response.Result[i]))
	}
	return results
}

// resultFrom turns one result of the plugin's into the launcher's own.
func (p *JSONRPCPlugin) resultFrom(answered JSONRPCResult) Result {
	result := NewResult(answered.Title, answered.SubTitle, answered.IcoPath, answered.Score)
	result.PluginDirectory = p.Directory
	result.PluginID = p.Meta.ID
	result.BadgeIcoPath = answered.BadgeIcoPath
	result.ShowBadge = answered.ShowBadge
	result.RoundedIcon = answered.RoundedIcon
	result.TitleHighlightData = answered.TitleHighlightData
	result.AutoCompleteText = answered.AutoCompleteText
	result.QuerySuggestionText = answered.QuerySuggestionText
	result.TitleToolTip = answered.TitleToolTip
	result.SubTitleToolTip = answered.SubTitleToolTip
	result.ActionKeywordAssigned = answered.ActionKeywordAssigned
	result.RecordKey = answered.RecordKey
	result.ContextData = answered.ContextData
	result.ProgressBar = answered.ProgressBar
	if answered.ProgressBarColor != "" {
		result.ProgressBarColor = answered.ProgressBarColor
	}
	if answered.CopyText != "" {
		result.SetCopyText(answered.CopyText)
	}
	if answered.Glyph != nil {
		result.Glyph = &GlyphInfo{
			FontFamily: strings.TrimPrefix(answered.Glyph.FontFamily, "/Resources/#"),
			Glyph:      answered.Glyph.Glyph,
		}
	}
	if answered.AddSelectedCount != nil {
		result.AddSelectedCount = *answered.AddSelectedCount
	}

	// The action is the plugin's own: the launcher asks it to do what the
	// result says, and leaves the launcher up if it says to.
	action := answered.Action
	settingsChange := answered.SettingsChange
	result.AsyncAction = func(ctx context.Context, _ ActionContext) bool {
		if action == nil {
			return true
		}
		if action.Method == "" {
			return !action.DontHideAfterAction
		}

		// A method of the launcher's own is done here rather than by the
		// plugin, which is how a plugin opens a URL or puts text on the
		// clipboard.
		if strings.HasPrefix(action.Method, flowLauncherPrefix) {
			p.callLauncher(action.Method[len(flowLauncherPrefix):], action.Parameters)
			return !action.DontHideAfterAction
		}

		response, err := p.requestWithID(ctx, action.ID, action.Method, action.Parameters)
		if err != nil || len(response.Result) == 0 {
			return !action.DontHideAfterAction
		}
		// The plugin may answer with something else to do.
		for _, next := range response.Result {
			if next.Action == nil {
				continue
			}
			if strings.HasPrefix(next.Action.Method, flowLauncherPrefix) {
				p.callLauncher(next.Action.Method[len(flowLauncherPrefix):], next.Action.Parameters)
			}
		}
		if len(settingsChange) > 0 {
			p.mu.Lock()
			for key, value := range settingsChange {
				p.settings[key] = value
			}
			p.mu.Unlock()
		}
		return !action.DontHideAfterAction
	}
	return result
}

// requestWithID asks the plugin to do something, with the number the plugin
// gave the action.
func (p *JSONRPCPlugin) requestWithID(ctx context.Context, id int, method string, parameters []any) (JSONRPCResponse, error) {
	request := JSONRPCRequest{ID: id, Method: method, Parameters: parameters}

	p.mu.Lock()
	request.Settings = p.settings
	p.mu.Unlock()

	serialized, err := json.Marshal(request)
	if err != nil {
		return JSONRPCResponse{}, err
	}
	answer, err := p.Runner.Run(ctx, p.Directory, serialized)
	if err != nil || len(answer) == 0 {
		return JSONRPCResponse{}, err
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(answer, &response); err != nil {
		return JSONRPCResponse{}, err
	}
	return response, nil
}

// flowLauncherPrefix marks a method the launcher itself does.
const flowLauncherPrefix = "Flow.Launcher."

// callLauncher does what a plugin asked the launcher to do, which is how a
// plugin reaches the window, the clipboard and the rest.
func (p *JSONRPCPlugin) callLauncher(method string, parameters []any) {
	// The API a plugin may call is small and named the same as the
	// reference's own, so each is looked up by name.
	switch method {
	case "OpenDirectory":
		if len(parameters) >= 1 {
			Launcher().OpenDirectory(text(parameters[0]), text(optional(parameters, 1)))
		}
	case "OpenUrl", "OpenWebUrl":
		if len(parameters) >= 1 {
			Launcher().OpenUrl(text(parameters[0]), nil)
		}
	case "OpenAppUri":
		if len(parameters) >= 1 {
			Launcher().OpenAppUri(text(parameters[0]))
		}
	case "ShellRun":
		if len(parameters) >= 1 {
			Launcher().ShellRun(text(parameters[0]), text(optional(parameters, 1)))
		}
	case "CopyToClipboard":
		if len(parameters) >= 1 {
			Launcher().CopyToClipboard(text(parameters[0]), false, true)
		}
	case "ChangeQuery":
		if len(parameters) >= 1 {
			Launcher().ChangeQuery(text(parameters[0]), boolOf(optional(parameters, 1)))
		}
	case "ReQuery":
		Launcher().ReQuery(true)
	case "BackToQueryResults":
		Launcher().BackToQueryResults()
	case "ShowMainWindow":
		Launcher().ShowMainWindow()
	case "HideMainWindow":
		Launcher().HideMainWindow()
	case "FocusQueryTextBox":
		Launcher().FocusQueryTextBox()
	case "StartLoadingBar":
		Launcher().StartLoadingBar()
	case "StopLoadingBar":
		Launcher().StopLoadingBar()
	case "OpenSettingDialog":
		Launcher().OpenSettingDialog()
	case "OpenPluginSettingsWindow":
		if len(parameters) >= 1 {
			Launcher().OpenPluginSettingsWindow(text(parameters[0]))
		}
	case "SaveAppAllSettings":
		Launcher().SaveAppAllSettings()
	case "SavePluginSettings":
		Launcher().SavePluginSettings()
	case "RestartApp":
		Launcher().RestartApp()
	case "ToggleGameMode":
		Launcher().ToggleGameMode()
	case "SetGameMode":
		if len(parameters) >= 1 {
			Launcher().SetGameMode(boolOf(parameters[0]))
		}
	case "IsGameModeOn":
		Launcher().IsGameModeOn()
	case "ShowMsg":
		Launcher().ShowMsg(text(parameters[0]), text(optional(parameters, 1)), text(optional(parameters, 2)))
	case "ShowMsgError":
		Launcher().ShowMsgError(text(parameters[0]), text(optional(parameters, 1)))
	case "GetTranslation":
		Launcher().GetTranslation(text(parameters[0]))
	case "GetDataDirectory":
		Launcher().GetDataDirectory()
	default:
		Logf("a plugin asked the launcher for %q, which it does not do", method)
	}
}

// Launcher is the API a plugin of another language calls, which the app
// fills in as it starts.
var launcher *PublicAPI

// SetLauncher gives the host the API that plugins of another language
// call.
func SetLauncher(api *PublicAPI) { launcher = api }

// Launcher is the API a plugin of another language calls.
func Launcher() *PublicAPI { return launcher }

// optional is the parameter at an index, or nothing when there is none.
func optional(parameters []any, index int) any {
	if index < len(parameters) {
		return parameters[index]
	}
	return nil
}

// text reads a parameter as text.
func text(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

// boolOf reads a parameter as a yes or no.
func boolOf(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	case float64:
		return typed != 0
	default:
		return false
	}
}

// errEmptyAnswer is what a plugin that said nothing answers.
var errEmptyAnswer = errors.New("the plugin said nothing")

// --- The runners ---

// ExecutableRunner runs a plugin that is a program of its own, which is
// handed the request as its last argument.
type ExecutableRunner struct {
	// Program is the program to run.
	Program string
	// LeadingArguments come before the request.
	LeadingArguments []string
	// Environment is what the program is run with, on top of the
	// launcher's own.
	Environment []string
	// Timeout bounds how long a request may take.
	Timeout time.Duration
}

// Run runs the plugin with a request and returns what it wrote.
func (r ExecutableRunner) Run(ctx context.Context, directory string, request []byte) ([]byte, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	arguments := append(append([]string{}, r.LeadingArguments...), string(request))
	command := exec.CommandContext(ctx, r.Program, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), r.Environment...)
	// A plugin that writes a window of its own would flash on the screen.
	hideWindow(command)

	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return nil, fmt.Errorf("the plugin failed: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, err
	}
	return output, nil
}

// PythonRunner runs a plugin written in Python, which the launcher starts
// with its own helper so that the plugin's own modules can be imported.
type PythonRunner struct {
	// Python is the interpreter to run.
	Python string
	// File is the plugin's own file.
	File string
	// Environment is what the program is run with.
	Environment []string
	// Timeout bounds how long a request may take.
	Timeout time.Duration
}

// Run runs the plugin with a request and returns what it wrote. The
// request is the last argument, after the interpreter's own, which is
// where a plugin of this kind reads it.
func (r PythonRunner) Run(ctx context.Context, directory string, request []byte) ([]byte, error) {
	var arguments []string
	if strings.HasSuffix(strings.ToLower(r.File), ".py") {
		// The plugin's own modules are made importable before it runs.
		arguments = []string{"-c", pythonPrelude(directory, r.File)}
	} else {
		// A packed plugin is run as it is.
		arguments = []string{"-B", r.File}
	}

	return ExecutableRunner{
		Program:          r.Python,
		LeadingArguments: arguments,
		Environment:      r.Environment,
		Timeout:          r.Timeout,
	}.Run(ctx, directory, request)
}

// pythonPrelude is the little program that makes a plugin's own modules
// importable and then runs it.
func pythonPrelude(directory, file string) string {
	paths := []string{
		directory,
		filepath.Join(directory, "lib"),
		filepath.Join(directory, "lib", "win32"),
		filepath.Join(directory, "lib", "win32", "lib"),
		filepath.Join(directory, "plugin"),
	}

	var prelude strings.Builder
	prelude.WriteString("import sys\n")
	for _, path := range paths {
		fmt.Fprintf(&prelude, "sys.path.append(r'%s')\n", path)
	}
	prelude.WriteString("import runpy\n")
	fmt.Fprintf(&prelude, "runpy.run_path(r'%s', None, '__main__')\n", file)
	return prelude.String()
}

// NodeRunner runs a plugin written in JavaScript or TypeScript.
type NodeRunner struct {
	// Node is the interpreter to run.
	Node string
	// File is the plugin's own file.
	File string
	// Environment is what the program is run with.
	Environment []string
	// Timeout bounds how long a request may take.
	Timeout time.Duration
}

// Run runs the plugin with a request and returns what it wrote.
func (r NodeRunner) Run(ctx context.Context, directory string, request []byte) ([]byte, error) {
	return ExecutableRunner{
		Program:          r.Node,
		LeadingArguments: []string{r.File},
		Environment:      r.Environment,
		Timeout:          r.Timeout,
	}.Run(ctx, directory, request)
}

// RunnerFor makes the runner a plugin is run with, from the language it
// says it is written in and the program the launcher knows of that
// language.
func RunnerFor(language, file, pythonPath, nodePath string, environment []string) (Runner, error) {
	switch language {
	case LanguageExecutable, LanguageExecutableV2:
		return ExecutableRunner{Program: file, Environment: environment}, nil

	case LanguagePython, LanguagePythonV2:
		if pythonPath == "" {
			return nil, fmt.Errorf("the Python interpreter is not set")
		}
		return PythonRunner{Python: pythonPath, File: file, Environment: environment}, nil

	case LanguageJavaScript, LanguageTypeScript, LanguageTypeScriptV2:
		if nodePath == "" {
			return nil, fmt.Errorf("the Node interpreter is not set")
		}
		return NodeRunner{Node: nodePath, File: file, Environment: environment}, nil

	default:
		return nil, fmt.Errorf("plugins written in %s are not supported", language)
	}
}
