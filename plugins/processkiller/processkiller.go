package processkiller

import (
	"context"
	"embed"
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

// The process killer's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about the process killer.
type Settings struct {
	// ShowWindowTitle shows a process's window title rather than its name
	// and id.
	ShowWindowTitle bool `json:"ShowWindowTitle"`
	// PutVisibleWindowProcessesTop lifts the processes that have a window
	// the user can see, which are the ones they usually mean.
	PutVisibleWindowProcessesTop bool `json:"PutVisibleWindowProcessesTop"`
}

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{ShowWindowTitle: true}
}

// Process is one running program.
type Process struct {
	// Name is the program's name, without its extension.
	Name string
	// ID is the system's number for it.
	ID int
	// Path is the file it was started from, empty when the system will not
	// say.
	Path string
	// WindowTitle is the title of its window, empty when it has none.
	WindowTitle string
}

// Title is how a process is named in a result, as the reference names it.
func (p Process) Title() string {
	return p.Name + " - " + itoa(p.ID)
}

// processCacheAge is how long the list of running processes is used before
// the machine is asked again. Reading it means a snapshot of every process
// and a window title for each, which is too slow to do for every keystroke.
const processCacheAge = 2 * time.Second

// Processes is what the plugin reads from the machine and does to it. It is
// kept behind a field so that a test can watch what the plugin asked for
// rather than reading and killing the machine's own processes.
type Processes interface {
	// Running returns the processes a user could mean: every one that is
	// not a part of the system and is not the launcher itself.
	Running() ([]Process, error)
	// Kill ends a process.
	Kill(process Process) error
}

// Plugin is the process killer: it lists the running programs and ends the
// one the user picks.
type Plugin struct {
	core.BasePlugin

	api       *core.PublicAPI
	settings  Settings
	words     *core.PluginTranslator
	processes Processes

	// cache is the list of processes read last, so that a keystroke does
	// not ask the machine for all of them again.
	cacheMu  sync.Mutex
	cache    []Process
	cachedAt time.Time
}

// New makes the process killer.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "b64d0a79-329a-48b0-b53f-d658318a1bf6",
			Name:           "Process Killer",
			Description:    "Kill running processes from Flow",
			Author:         "Flow-Launcher",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher.Plugin.ProcessKiller",
			ActionKeywords: []string{"kill"},
			IcoPath:        "Images/app.png",
		}},
		processes: systemProcesses{},
	}
}

// Init reads the process killer's settings.
// installImages writes the pictures this plugin ships where the launcher
// reads them from, which is the plugin's own directory.
func (p *Plugin) installImages(ctx *core.PluginInitContext) {
	if err := core.InstallImages(ctx.CurrentPluginMetadata.PluginDirectory, images, "Images/*.png"); err != nil {
		ctx.API.LogError("processkiller", "writing the plugin's pictures failed: "+err.Error())
	}
}

// Init reads the plugin's settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.installImages(ctx)
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")
	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "ProcessKiller", "Settings.json")
	return nil
}

// Settings hands the plugin's settings over, which the launcher shows and
// changes in a window of its own.
func (p *Plugin) Settings() any { return &p.settings }

// Save writes the settings out.
func (p *Plugin) Save() {
	_ = core.SaveSettingJsonStorage(p.api, "ProcessKiller", "Settings.json", p.settings)
}

// Query lists the running processes that match what the user typed, and the
// processes whose windows they can see when nothing is typed.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	running, err := p.runningNow()
	if err != nil {
		return nil, err
	}
	if len(running) == 0 {
		return nil, nil
	}

	searchTerm := strings.TrimSpace(query.Search)
	matcher := p.api.Matcher()

	type scored struct {
		process  Process
		score    int
		title    string
		highlite []int
	}
	var matched []scored

	for _, process := range running {
		title := process.Title()
		if p.settings.ShowWindowTitle && process.WindowTitle != "" {
			title = process.WindowTitle
		}

		if searchTerm == "" {
			score := 0
			if p.settings.PutVisibleWindowProcessesTop && process.WindowTitle != "" {
				score = 200
			}
			matched = append(matched, scored{process: process, score: score, title: title})
			continue
		}

		// A process is found by its window title or by its name and id,
		// whichever matches better.
		nameMatch := matcher.FuzzyMatch(searchTerm, process.Title())
		score := nameMatch.Score()
		var highlite []int

		if process.WindowTitle != "" {
			windowMatch := matcher.FuzzyMatch(searchTerm, process.WindowTitle)
			if windowMatch.Score() > score {
				score = windowMatch.Score()
				highlite = windowMatch.MatchData
			} else {
				highlite = nameMatch.MatchData
			}
		} else {
			highlite = nameMatch.MatchData
		}

		if score <= 0 {
			continue
		}
		if p.settings.PutVisibleWindowProcessesTop && process.WindowTitle != "" {
			score += 200
		}
		matched = append(matched, scored{process: process, score: score, title: title, highlite: highlite})
	}

	// The results are listed by title, which is what the reference does.
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].title < matched[j].title })

	results := make([]core.Result, 0, len(matched))
	for _, entry := range matched {
		process := entry.process
		result := core.NewResult(entry.title, process.Path, process.Path, entry.score)
		result.TitleHighlightData = entry.highlite
		result.TitleToolTip = process.Title()
		result.ContextData = process.Name
		result.AutoCompleteText = p.Meta.ActionKeywords[0] + core.TermSeparator + process.Name
		result.Action = func(core.ActionContext) bool {
			if err := p.processes.Kill(process); err != nil {
				p.api.LogError("ProcessKiller", "ending a process failed: "+err.Error())
			}
			p.forgetProcesses()
			// The list of processes has changed, so the query is run again.
			p.api.ReQuery(true)
			return true
		}
		results = append(results, result)
	}

	// When every result is the same program, the user probably wants to end
	// all of it, so that is offered just below the first of them.
	if searchTerm != "" && len(results) > 1 && allSamePath(results) {
		all := make([]Process, 0, len(matched))
		for _, entry := range matched {
			all = append(all, entry.process)
		}
		killAll := core.NewResult(
			p.words.Tf("flowlauncher_plugin_processkiller_kill_all", nameOf(results[0].ContextData)),
			p.words.Tf("flowlauncher_plugin_processkiller_kill_all_count", itoa(len(all))),
			results[0].IcoPath, 200)
		killAll.Action = func(core.ActionContext) bool {
			for _, process := range all {
				if err := p.processes.Kill(process); err != nil {
					p.api.LogError("ProcessKiller", "ending a process failed: "+err.Error())
				}
			}
			p.forgetProcesses()
			p.api.ReQuery(true)
			return true
		}
		results = append(results[:1], append([]core.Result{killAll}, results[1:]...)...)
	}

	return results, nil
}

// runningNow is the processes that are running, read from the machine when
// what was read last is old enough.
func (p *Plugin) runningNow() ([]Process, error) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	if p.cache != nil && time.Since(p.cachedAt) < processCacheAge {
		return p.cache, nil
	}

	running, err := p.processes.Running()
	if err != nil {
		return nil, err
	}
	p.cache, p.cachedAt = running, time.Now()
	return running, nil
}

// LoadContextMenus offers to end every instance of the same program.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	path := selectedResult.SubTitle
	if path == "" {
		return nil
	}

	running, err := p.processes.Running()
	if err != nil {
		return nil
	}

	var similar []Process
	for _, process := range running {
		if process.Path == path {
			similar = append(similar, process)
		}
	}
	if len(similar) == 0 {
		return nil
	}

	item := core.NewResult(p.words.T("flowlauncher_plugin_processkiller_kill_instances"), path, path, 0)
	item.Action = func(core.ActionContext) bool {
		for _, process := range similar {
			if err := p.processes.Kill(process); err != nil {
				p.api.LogError("ProcessKiller", "ending a process failed: "+err.Error())
			}
		}
		// The list has changed, so what was read last is left out.
		p.forgetProcesses()
		return true
	}
	return []core.Result{item}
}

// forgetProcesses leaves out what was read last, after something ended.
func (p *Plugin) forgetProcesses() {
	p.cacheMu.Lock()
	p.cache, p.cachedAt = nil, time.Time{}
	p.cacheMu.Unlock()
}

// allSamePath reports whether every result is the same program.
func allSamePath(results []core.Result) bool {
	if len(results) == 0 {
		return false
	}
	first := results[0].SubTitle
	if first == "" {
		return false
	}
	for _, result := range results {
		if result.SubTitle != first {
			return false
		}
	}
	return true
}

// nameOf reads the process name a result carries.
func nameOf(contextData any) string {
	if name, ok := contextData.(string); ok {
		return name
	}
	return ""
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
