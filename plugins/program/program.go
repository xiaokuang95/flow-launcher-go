package program

import (
	"context"
	"embed"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"launch/core"
)

// The pictures this plugin shows its results with, which the launcher
// reads from the plugin's own directory.
//
//go:embed Images/*.png
var images embed.FS

// The program plugin's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about the search for programs.
type Settings struct {
	// LastIndexTime is when the programs were last looked up.
	LastIndexTime time.Time `json:"LastIndexTime"`
	// ProgramSources are the directories the user added to look for
	// programs in.
	ProgramSources []ProgramSource `json:"ProgramSources"`
	// DisabledProgramSources are the programs the user hid.
	DisabledProgramSources []ProgramSource `json:"DisabledProgramSources"`

	// CustomSuffixes and CustomProtocols are the file types and URL
	// schemes the user added.
	CustomSuffixes  []string `json:"CustomSuffixes"`
	CustomProtocols []string `json:"CustomProtocols"`

	// BuiltinSuffixesStatus is which of the file types the launcher looks
	// for are in use.
	BuiltinSuffixesStatus map[string]bool `json:"BuiltinSuffixesStatus"`
	// BuiltinProtocolsStatus is which of the URL schemes are in use.
	BuiltinProtocolsStatus map[string]bool `json:"BuiltinProtocolsStatus"`

	// UseCustomSuffixes and UseCustomProtocols add the user's own to the
	// built-in ones rather than replacing them.
	UseCustomSuffixes  bool `json:"UseCustomSuffixes"`
	UseCustomProtocols bool `json:"UseCustomProtocols"`

	// EnableDescription shows what a program says it does beside its name.
	EnableDescription bool `json:"EnableDescription"`
	// HideAppsPath hides where a program lives from its subtitle.
	HideAppsPath bool `json:"HideAppsPath"`
	// HideUninstallers leaves uninstallers out of the results.
	HideUninstallers bool `json:"HideUninstallers"`
	// HideDuplicatedWindowsApp leaves out the programs of an installed
	// Windows app that the app itself also lists.
	HideDuplicatedWindowsApp bool `json:"HideDuplicatedWindowsApp"`

	// EnableRegistrySource looks for programs in the registry's App Paths.
	EnableRegistrySource bool `json:"EnableRegistrySource"`
	// EnableStartMenuSource looks for programs in the Start Menu.
	EnableStartMenuSource bool `json:"EnableStartMenuSource"`
	// EnablePathSource looks for programs on the PATH.
	EnablePathSource bool `json:"EnablePathSource"`
	// EnableUwpSource looks for the Windows apps installed from the store.
	EnableUwpSource bool `json:"EnableUwpSource"`
}

// ProgramSource is a directory programs are looked for in, or one program
// the user hid.
type ProgramSource struct {
	// Location is the directory, or the program's own path.
	Location string `json:"Location"`
	// Name is what the source is called in the settings.
	Name string `json:"Name"`
	// Enabled is false for a source the user turned off.
	Enabled bool `json:"Enabled"`
	// UniqueIdentifier identifies the program a disabled source names.
	UniqueIdentifier string `json:"UniqueIdentifier"`
}

// The file types and URL schemes a program is looked for by, as the
// reference's settings have them.
var (
	builtinSuffixes  = []string{"exe", "appref-ms", "lnk"}
	builtinProtocols = map[string]string{
		"steam": "steam://run/",
		"epic":  "com.epicgames.launcher://apps/",
		"http":  "http://",
	}
)

// DefaultSettings are the settings a user who changed nothing gets, which
// are the reference's own: the Start Menu and the registry are looked in,
// the PATH is not, and a result is the program's name alone.
func DefaultSettings() Settings {
	return Settings{
		BuiltinSuffixesStatus:  map[string]bool{"exe": true, "appref-ms": true, "lnk": true},
		BuiltinProtocolsStatus: map[string]bool{"steam": true, "epic": true, "http": false},
		EnableRegistrySource:   true,
		EnableStartMenuSource:  true,
		EnablePathSource:       false,
		EnableUwpSource:        true,
		EnableDescription:      false,
		HideAppsPath:           true,
	}
}

// Suffixes are the file types the plugin looks for.
func (s Settings) Suffixes() []string {
	var suffixes []string
	for _, suffix := range builtinSuffixes {
		if s.BuiltinSuffixesStatus[suffix] {
			suffixes = append(suffixes, suffix)
		}
	}
	for _, enabled := range s.BuiltinProtocolsStatus {
		if enabled {
			suffixes = append(suffixes, "url")
			break
		}
	}
	if s.UseCustomProtocols && len(s.CustomProtocols) > 0 {
		suffixes = append(suffixes, "url")
	}
	if s.UseCustomSuffixes {
		suffixes = append(suffixes, s.CustomSuffixes...)
	}
	return distinctLower(suffixes)
}

// Protocols are the URL schemes a shortcut may point at for the plugin to
// count it as a program.
func (s Settings) Protocols() []string {
	var protocols []string
	for name, enabled := range s.BuiltinProtocolsStatus {
		if !enabled {
			continue
		}
		if prefix, ok := builtinProtocols[name]; ok {
			protocols = append(protocols, prefix)
		}
	}
	if s.UseCustomProtocols {
		protocols = append(protocols, s.CustomProtocols...)
	}
	return protocols
}

// Program is one program the plugin found.
type Program struct {
	// Name is what the program is called.
	Name string `json:"Name"`
	// Description is what the program says it does.
	Description string `json:"Description"`
	// LocalizedName is the name the shell gives the shortcut, when it
	// differs from the file's own name.
	LocalizedName string `json:"LocalizedName"`
	// IcoPath is the program's icon.
	IcoPath string `json:"IcoPath"`
	// FullPath is the file that was found: a shortcut, an executable or an
	// internet shortcut.
	FullPath string `json:"FullPath"`
	// LnkResolvedPath is what a shortcut points at, or the URL of an
	// internet shortcut.
	LnkResolvedPath string `json:"LnkResolvedPath"`
	// ParentDirectory is where the program lives.
	ParentDirectory string `json:"ParentDirectory"`
	// ExecutableName is the name of the program a shortcut runs.
	ExecutableName string `json:"ExecutableName"`
	// Args are the arguments a shortcut runs its program with.
	Args string `json:"Args"`
	// UniqueIdentifier tells two programs apart.
	UniqueIdentifier string `json:"UniqueIdentifier"`
	// Enabled is false for a program the user hid.
	Enabled bool `json:"Enabled"`

	// AppUserModelID is what the shell runs a Windows app installed from
	// the store by, and is empty for a program that is a file.
	AppUserModelID string `json:"AppUserModelID"`
	// CanRunElevated is true for a store app that asked to be able to run
	// as an administrator.
	CanRunElevated bool `json:"CanRunElevated"`
}

// DisplayName is what the program is called in a result.
func (p Program) DisplayName() string {
	if p.LocalizedName != "" && p.LocalizedName != p.Name {
		return p.LocalizedName
	}
	return p.Name
}

// ProgramFile is the program a result runs: what a shortcut points at when
// it points at something, and the file itself otherwise. Whether the target
// is there was settled when the programs were looked up.
func (p Program) ProgramFile() string {
	if p.LnkResolvedPath != "" {
		return p.LnkResolvedPath
	}
	return p.FullPath
}

// RunTarget is what the shell is asked to run: a Windows app installed
// from the store is not a file, so it is run by the application user model
// id the shell knows it by.
func (p Program) RunTarget() string {
	if p.AppUserModelID != "" {
		return `shell:AppsFolder\` + p.AppUserModelID
	}
	return p.ProgramFile()
}

// Finder looks for the programs on the machine. It is what the plugin reads
// from the machine, kept behind a field so that a test can give it a list of
// its own rather than reading the machine's own Start Menu.
type Finder interface {
	// Find returns the programs the settings ask for.
	Find(settings Settings) ([]Program, error)
	// Start runs a program.
	Start(program Program, arguments string, runAsAdmin bool) error
	// StartAsDifferentUser runs one as another user.
	StartAsDifferentUser(program Program, arguments string) error
}

// Plugin is the program plugin: it finds the programs installed on the
// machine and runs them.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	settings Settings
	words    *core.PluginTranslator
	finder   Finder

	programs []Program
	// indexed is when the programs were last looked up, which the cache is
	// kept by.
	indexed time.Time
}

// New makes the program plugin.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "791FC278BA414111B8D1886DFE447410",
			Name:           "Program",
			Description:    "Search programs in Flow.Launcher",
			Author:         "qianlifeng",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords: []string{"*"},
			IcoPath:        "Images/program.png",
		}},
		finder: systemFinder{},
	}
}

// Init reads the settings and looks the programs up, from the cache when it
// is fresh enough.
// installImages writes the pictures this plugin ships where the launcher
// reads them from, which is the plugin's own directory.
func (p *Plugin) installImages(ctx *core.PluginInitContext) {
	if err := core.InstallImages(ctx.CurrentPluginMetadata.PluginDirectory, images, "Images/*.png"); err != nil {
		ctx.API.LogError("program", "writing the plugin's pictures failed: "+err.Error())
	}
}

// Init reads the plugin's settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.installImages(ctx)
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")

	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "Program", "Settings.json")
	if p.settings.BuiltinSuffixesStatus == nil {
		p.settings = DefaultSettings()
	}

	p.programs = core.LoadSettingJsonStorage[[]Program](ctx.API, "Program", "Programs.json")
	p.indexed = p.settings.LastIndexTime

	// The programs are looked up again when the cache is empty or old,
	// which the reference does after thirty hours.
	if len(p.programs) == 0 || time.Since(p.indexed) > 30*time.Hour {
		p.reindex()
	}
	return nil
}

// reindex looks the programs up again and writes the cache.
func (p *Plugin) reindex() {
	programs, err := p.finder.Find(p.settings)
	if err != nil {
		p.api.LogError("Program", "looking for programs failed: "+err.Error())
		return
	}

	sort.SliceStable(programs, func(i, j int) bool { return programs[i].Name < programs[j].Name })
	p.programs = programs
	p.indexed = time.Now()
	p.settings.LastIndexTime = p.indexed

	_ = core.SaveSettingJsonStorage(p.api, "Program", "Programs.json", programs)
	_ = core.SaveSettingJsonStorage(p.api, "Program", "Settings.json", p.settings)
}

// ReloadData looks the programs up again, as F5 asks.
func (p *Plugin) ReloadData(ctx context.Context) error {
	p.reindex()
	return nil
}

// Query answers what the user typed with the programs whose name, path or
// description matches it.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	search := query.Search
	matcher := p.api.Matcher()

	// Every character of the query has to appear in a program's name, in
	// order, for the match to have any chance; checking that first is far
	// cheaper than scoring every program, and rejects most of them. The
	// check compares letters as they are written, so it is only sound
	// while the matcher does the same: with accents folded away, a query
	// can match a letter a program spells with a diacritic.
	needle := []rune(strings.ToLower(search))
	prefilter := search != "" && !matcher.IgnoreAccents

	var results []core.Result
	for i := range p.programs {
		program := p.programs[i]
		if !program.Enabled || p.disabled(program) {
			continue
		}
		if p.settings.HideUninstallers && isUninstaller(program) {
			continue
		}
		if prefilter && !p.mightMatch(needle, program, matcher.Alphabet) {
			continue
		}

		result, ok := p.resultFor(program, search, matcher)
		if !ok {
			continue
		}
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results, nil
}

// mightMatch is the cheap pre-check above: every character of the query has
// to appear, in order, in one of the fields resultFor looks at, so a program
// that fails this cannot match at all.
func (p *Plugin) mightMatch(needle []rune, program Program, alphabet core.Alphabet) bool {
	if mightMatchOne(needle, alphabet, program.DisplayName()) ||
		mightMatchOne(needle, alphabet, program.Name) ||
		mightMatchOne(needle, alphabet, program.ExecutableName) {
		return true
	}
	return p.settings.EnableDescription && mightMatchOne(needle, alphabet, program.Description)
}

// mightMatchOne is the same check for one field. A name written in Chinese
// is matched by its pinyin, which the characters themselves do not hold, so
// what the matcher will compare it with is checked as well.
func mightMatchOne(needle []rune, alphabet core.Alphabet, text string) bool {
	if isSubsequence(needle, text) {
		return true
	}
	if alphabet == nil || !core.HasChinese(text) {
		return false
	}
	translated, _ := alphabet.Translate(text)
	return isSubsequence(needle, translated)
}

// resultFor is one program as a result, and whether it matched at all.
func (p *Plugin) resultFor(program Program, search string, matcher core.StringMatcher) (core.Result, bool) {
	name := program.DisplayName()

	// The title carries the description too, when the user asks for it, so
	// that a program is found by what it does.
	title := name
	var match core.MatchResult
	if p.settings.EnableDescription && program.Description != "" && program.Description != name {
		title = name + ": " + program.Description

		nameMatch := matcher.FuzzyMatch(search, name)
		descriptionMatch := matcher.FuzzyMatch(search, program.Description)
		if descriptionMatch.Score() > nameMatch.Score() {
			// The highlight is moved along by the name and the ": ".
			offset := len([]rune(name)) + 2
			for i := range descriptionMatch.MatchData {
				descriptionMatch.MatchData[i] += offset
			}
			match = descriptionMatch
		} else {
			match = nameMatch
		}
	} else {
		match = matcher.FuzzyMatch(search, name)
	}

	// A program is also found by the name of the program a shortcut runs,
	// which is what the user may remember.
	if !match.IsSearchPrecisionScoreMet() && search != "" {
		candidates := []string{}
		if program.ExecutableName != "" {
			candidates = append(candidates, program.ExecutableName)
		}
		if program.LocalizedName != "" && program.LocalizedName != program.Name {
			candidates = append(candidates, program.Name)
		}
		// The candidates are tried one by one; a program that matches none
		// of them is not a result at all.
		matched := false
		best := core.MatchResult{}
		for _, candidate := range candidates {
			candidateMatch := matcher.FuzzyMatch(search, candidate)
			if !candidateMatch.IsSearchPrecisionScoreMet() {
				continue
			}
			if !matched || candidateMatch.Score() > best.Score() {
				best, matched = candidateMatch, true
			}
		}
		if !matched {
			return core.Result{}, false
		}
		// Nothing in the title is highlighted in this case.
		best.MatchData = nil
		match = best
	}

	if search != "" && !match.IsSearchPrecisionScoreMet() {
		return core.Result{}, false
	}

	subtitle := ""
	if !p.settings.HideAppsPath {
		subtitle = program.FullPath
		if strings.EqualFold(filepath.Ext(program.FullPath), ".url") {
			subtitle = program.LnkResolvedPath
		}
	}

	result := core.NewResult(title, subtitle, program.IcoPath, match.Score())
	result.TitleHighlightData = match.MatchData
	result.TitleToolTip = title + "\n" + program.ProgramFile()
	result.AutoCompleteText = name
	result.ContextData = program

	arguments := program.Args
	result.Action = func(action core.ActionContext) bool {
		switch {
		case action.SpecialKeyState.CtrlPressed && !action.SpecialKeyState.ShiftPressed:
			// Ctrl+Enter opens where the program lives. A store app is not
			// a file to pick out, so the folder itself is opened.
			if program.AppUserModelID != "" {
				p.api.OpenDirectory(program.FullPath, "")
				return true
			}
			p.api.OpenDirectory(program.ParentDirectory, program.FullPath)
		case action.SpecialKeyState.CtrlPressed && action.SpecialKeyState.ShiftPressed:
			p.runElevated(program, arguments)
		default:
			p.run(program, arguments, false)
		}
		return true
	}
	return result, true
}

// runElevated runs a program as an administrator, which a store app only
// allows when the package said it may.
func (p *Plugin) runElevated(program Program, arguments string) {
	if program.AppUserModelID != "" && !program.CanRunElevated {
		p.api.ShowMsgError(
			p.words.T("flowlauncher_plugin_program_disable_dlgtitle_error"),
			p.words.T("flowlauncher_plugin_program_run_as_administrator_not_supported_message"))
		return
	}
	p.run(program, arguments, true)
}

// run starts a program.
func (p *Plugin) run(program Program, arguments string, runAsAdmin bool) {
	if err := p.finder.Start(program, arguments, runAsAdmin); err != nil {
		p.api.ShowMsgError(p.words.T("flowlauncher_plugin_program_plugin_name"),
			p.words.Tf("flowlauncher_plugin_program_run_failed", err.Error()))
	}
}

// disabled reports whether the user hid a program.
func (p *Plugin) disabled(program Program) bool {
	for _, source := range p.settings.DisabledProgramSources {
		if source.UniqueIdentifier == program.UniqueIdentifier {
			return true
		}
	}
	return false
}

// LoadContextMenus offers running the program another way, and where it
// lives.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	program, ok := selectedResult.ContextData.(Program)
	if !ok {
		return nil
	}
	words := p.words

	// A store app is not a file, so it is not run as another user and has
	// no program of its own to open: it is offered where it is installed,
	// and running it elevated when its package allows that.
	if program.AppUserModelID != "" {
		return p.uwpContextMenus(program)
	}

	differentUser := core.NewResult(words.T("flowlauncher_plugin_program_run_as_different_user"), "", "Images/user.png", 0)
	differentUser.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: "\ue7ee"}
	differentUser.Action = func(core.ActionContext) bool {
		_ = p.finder.StartAsDifferentUser(program, program.Args)
		return true
	}

	administrator := core.NewResult(words.T("flowlauncher_plugin_program_run_as_administrator"), "", "Images/cmd.png", 0)
	administrator.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: "\ue7ef"}
	administrator.Action = func(core.ActionContext) bool {
		p.run(program, program.Args, true)
		return true
	}

	folder := core.NewResult(words.T("flowlauncher_plugin_program_open_containing_folder"), "", "Images/folder.png", 0)
	folder.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: "\ue838"}
	folder.Action = func(core.ActionContext) bool {
		p.api.OpenDirectory(program.ParentDirectory, program.FullPath)
		return true
	}

	items := []core.Result{differentUser, administrator, folder}

	// A shortcut's own program lives somewhere else again.
	if strings.EqualFold(filepath.Ext(program.FullPath), ".lnk") && program.LnkResolvedPath != "" {
		target := core.NewResult(words.T("flowlauncher_plugin_program_open_target_folder"), "", "Images/folder.png", 0)
		target.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: "\ue8de"}
		target.Action = func(core.ActionContext) bool {
			p.api.OpenDirectory(filepath.Dir(program.LnkResolvedPath), program.LnkResolvedPath)
			return true
		}
		items = append(items, target)
	}

	// And it can be hidden from the results altogether.
	hide := core.NewResult(words.T("flowlauncher_plugin_program_disable_program"), "", "Images/disable.png", 0)
	hide.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: "\uece4"}
	hide.Action = func(core.ActionContext) bool {
		p.hide(program)
		return false
	}
	items = append(items, hide)

	return items
}

// uwpContextMenus are what can be done with a Windows app installed from
// the store, which the reference offers the same way: where it is
// installed, and running it as an administrator when its package allows
// that.
func (p *Plugin) uwpContextMenus(program Program) []core.Result {
	folder := core.NewResult(p.words.T("flowlauncher_plugin_program_open_containing_folder"), "", "Images/folder.png", 0)
	folder.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: ""}
	folder.Action = func(core.ActionContext) bool {
		p.api.OpenDirectory(program.FullPath, "")
		return true
	}

	items := []core.Result{folder}
	if program.CanRunElevated {
		administrator := core.NewResult(p.words.T("flowlauncher_plugin_program_run_as_administrator"), "", "Images/cmd.png", 0)
		administrator.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: ""}
		administrator.Action = func(core.ActionContext) bool {
			p.runElevated(program, program.Args)
			return true
		}
		items = append(items, administrator)
	}
	return items
}

// hide leaves a program out of the results from now on.
func (p *Plugin) hide(program Program) {
	for _, source := range p.settings.DisabledProgramSources {
		if source.UniqueIdentifier == program.UniqueIdentifier {
			return
		}
	}

	p.settings.DisabledProgramSources = append(p.settings.DisabledProgramSources, ProgramSource{
		Location:         program.FullPath,
		Name:             program.Name,
		Enabled:          false,
		UniqueIdentifier: program.UniqueIdentifier,
	})
	_ = core.SaveSettingJsonStorage(p.api, "Program", "Settings.json", p.settings)
	p.api.ReQuery(true)
}

// uninstallerNames are the names an uninstaller is given, and the words one
// may start with in the languages the launcher speaks.
var uninstallerNames = []string{"uninst.exe", "unins000.exe", "uninst000.exe", "uninstall.exe"}

var uninstallerPrefixes = []string{
	"uninstall", "卸载", "卸載", "видалити", "удалить", "désinstaller", "アンインストール",
	"deïnstalleren", "odinstaluj", "afinstallere", "deinstallieren", "삭제", "деинсталирај",
	"desinstalar", "disinstallare", "avinstallere", "odinštalovať", "kaldır", "odinstalovat",
	"إلغاء التثبيت", "gỡ bỏ", "הסרה",
}

// isUninstaller reports whether a program is one that removes another.
func isUninstaller(program Program) bool {
	name := strings.ToLower(filepath.Base(program.ProgramFile()))
	for _, uninstaller := range uninstallerNames {
		if name == uninstaller {
			return true
		}
	}
	for _, prefix := range uninstallerPrefixes {
		if strings.HasPrefix(name, strings.ToLower(prefix)) && strings.HasSuffix(name, ".exe") {
			return true
		}
	}

	// The shortcut itself may be named for the uninstaller.
	if program.LnkResolvedPath != "" {
		linkName := strings.ToLower(filepath.Base(program.FullPath))
		for _, prefix := range uninstallerPrefixes {
			if strings.HasPrefix(linkName, strings.ToLower(prefix)) && strings.HasSuffix(linkName, ".lnk") {
				return true
			}
		}
	}
	return false
}

// isSubsequence reports whether every character of the needle appears in
// the text, in order, whatever their case. A fuzzy match cannot succeed
// without it.
func isSubsequence(needle []rune, text string) bool {
	if len(needle) == 0 {
		return true
	}

	at := 0
	for _, r := range text {
		if lowerRune(r) == needle[at] {
			at++
			if at == len(needle) {
				return true
			}
		}
	}
	return false
}

// lowerRune is the lower case of a rune, for the cheap check above.
func lowerRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// distinctLower returns the values without their duplicates, comparing
// without case.
func distinctLower(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		lower := strings.ToLower(value)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		out = append(out, lower)
	}
	return out
}

// Settings hands the plugin's settings over, which the launcher shows and
// changes in a window of its own.
func (p *Plugin) Settings() any { return &p.settings }

// Save writes the settings out.
func (p *Plugin) Save() {
	_ = core.SaveSettingJsonStorage(p.api, "Program", "Settings.json", p.settings)
}

// ReloadData looks the programs up again, as F5 asks.
