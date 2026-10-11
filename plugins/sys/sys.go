package sys

import (
	"context"
	"embed"
	"sort"
	"strings"

	"launch/core"
)

// The system commands' own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about the system commands.
type Settings struct {
	// Commands are the commands, with the name and the words each is
	// found by. They are stored so that the user can rename them and
	// change what they answer to.
	Commands []Command `json:"Commands"`
	// SkipPowerActionConfirmation runs a command that turns the machine
	// off without asking first.
	SkipPowerActionConfirmation bool `json:"SkipPowerActionConfirmation"`
}

// Command is one of the system commands.
type Command struct {
	// Key is which command it is, which does not change.
	Key string `json:"Key"`
	// Keyword is what the user types to find it.
	Keyword string `json:"Keyword"`
	// Name is what it is called, which the user can change.
	Name string `json:"Name"`
	// Description is what it does.
	Description string `json:"Description"`
}

// commandTitles are the words each command is called, as keys of the
// launcher's own language files.
var commandTitles = map[string]string{
	"Shutdown":                           "flowlauncher_plugin_sys_shutdown_computer_cmd",
	"Restart":                            "flowlauncher_plugin_sys_restart_computer_cmd",
	"Restart With Advanced Boot Options": "flowlauncher_plugin_sys_restart_advanced_cmd",
	"Log Off/Sign Out":                   "flowlauncher_plugin_sys_log_off_cmd",
	"Lock":                               "flowlauncher_plugin_sys_lock_cmd",
	"Sleep":                              "flowlauncher_plugin_sys_sleep_cmd",
	"Hibernate":                          "flowlauncher_plugin_sys_hibernate_cmd",
	"Index Option":                       "flowlauncher_plugin_sys_indexoption_cmd",
	"Empty Recycle Bin":                  "flowlauncher_plugin_sys_emptyrecyclebin_cmd",
	"Open Recycle Bin":                   "flowlauncher_plugin_sys_openrecyclebin_cmd",
	"Exit":                               "flowlauncher_plugin_sys_exit_cmd",
	"Save Settings":                      "flowlauncher_plugin_sys_save_all_settings_cmd",
	"Restart Flow Launcher":              "flowlauncher_plugin_sys_restart_cmd",
	"Settings":                           "flowlauncher_plugin_sys_setting_cmd",
	"Reload Plugin Data":                 "flowlauncher_plugin_sys_reload_plugin_data_cmd",
	"Check For Update":                   "flowlauncher_plugin_sys_check_for_update_cmd",
	"Open Log Location":                  "flowlauncher_plugin_sys_open_log_location_cmd",
	"Flow Launcher Tips":                 "flowlauncher_plugin_sys_open_docs_tips_cmd",
	"Flow Launcher UserData Folder":      "flowlauncher_plugin_sys_open_userdata_location_cmd",
	"Toggle Game Mode":                   "flowlauncher_plugin_sys_toggle_game_mode_cmd",
	"Set Flow Launcher Theme":            "flowlauncher_plugin_sys_theme_selector_cmd",
}

// commandOrder is the order the commands are listed in, which is the order
// the reference has them in.
var commandOrder = []string{
	"Shutdown",
	"Restart",
	"Restart With Advanced Boot Options",
	"Log Off/Sign Out",
	"Lock",
	"Sleep",
	"Hibernate",
	"Index Option",
	"Empty Recycle Bin",
	"Open Recycle Bin",
	"Exit",
	"Save Settings",
	"Restart Flow Launcher",
	"Settings",
	"Reload Plugin Data",
	"Check For Update",
	"Open Log Location",
	"Flow Launcher Tips",
	"Flow Launcher UserData Folder",
	"Toggle Game Mode",
	"Set Flow Launcher Theme",
}

// documentation is where the tips are.
const documentation = "https://flowlauncher.com/docs/#/usage-tips"

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	commands := make([]Command, 0, len(commandOrder))
	for _, key := range commandOrder {
		commands = append(commands, Command{Key: key, Keyword: key})
	}
	return Settings{Commands: commands}
}

// Machine is what the commands do to the machine. It is kept behind a
// field so that a test watches what a command asked for rather than
// turning the machine off.
type Machine interface {
	// ShutDown turns the machine off.
	ShutDown() error
	// Restart starts it again.
	Restart() error
	// RestartToFirmwareSettings starts it again at its own setup screen.
	RestartToFirmwareSettings() error
	// LogOff ends the session.
	LogOff() error
	// Lock locks the screen.
	Lock() error
	// Sleep puts the machine to sleep.
	Sleep() error
	// Hibernate writes the session to disk and turns the machine off.
	Hibernate() error
	// OpenIndexingOptions shows what the machine is indexing.
	OpenIndexingOptions() error
	// EmptyRecycleBin throws away what is in the recycle bin.
	EmptyRecycleBin() error
}

// Plugin is the system commands: turning the machine off and on, locking
// it, and the launcher's own settings.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	settings Settings
	words    *core.PluginTranslator
	machine  Machine
}

// New makes the system commands.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "CEA08895D2544B019B2E9C5009600DF4",
			Name:           "System Commands",
			Description:    "Provide System related commands. e.g. shutdown,lock, setting etc.",
			Author:         "qianlifeng",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords: []string{"*"},
			IcoPath:        "Images/lock.png",
		}},
		machine: systemMachine{},
	}
}

// Init reads the settings and gives every command its words.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")

	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "System Commands", "Settings.json")
	if len(p.settings.Commands) == 0 {
		p.settings = DefaultSettings()
	}
	p.nameCommands()
	return nil
}

// nameCommands gives every command the name and the words it is found by,
// unless the user has changed them.
func (p *Plugin) nameCommands() {
	for i := range p.settings.Commands {
		command := &p.settings.Commands[i]
		if command.Name == "" {
			command.Name = core.T("", commandTitles[command.Key])
		}
		if command.Description == "" {
			command.Description = core.T("", descriptionKey(command.Key))
		}
		if command.Keyword == "" {
			command.Keyword = command.Key
		}
	}
}

// descriptionKey is the key of the words that say what a command does,
// which the reference makes by dropping the ending of the title's key.
func descriptionKey(key string) string {
	title := commandTitles[key]
	return strings.TrimSuffix(title, "_cmd")
}

// Query answers what the user typed with the commands whose name, words or
// keyword match it.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	search := strings.TrimSpace(query.Search)
	matcher := p.api.Matcher()

	type scored struct {
		result core.Result
		score  int
	}
	var matched []scored

	for i := range p.settings.Commands {
		command := p.settings.Commands[i]
		result := p.resultFor(command, query)

		if search == "" {
			matched = append(matched, scored{result: result})
			continue
		}

		// A command is found by its name, by what it does, or by the word
		// the user gave it.
		titleMatch := matcher.FuzzyMatch(search, result.Title)
		subtitleMatch := matcher.FuzzyMatch(search, result.SubTitle)
		keywordMatch := matcher.FuzzyMatch(search, command.Keyword)

		score := titleMatch.Score()
		if subtitleMatch.Score() > score {
			score = subtitleMatch.Score()
		}
		if keywordMatch.Score() > score {
			score = keywordMatch.Score()
		}
		if score <= 0 {
			continue
		}

		result.Score = score
		// Only a name that matched is highlighted.
		if score == titleMatch.Score() {
			result.TitleHighlightData = titleMatch.MatchData
		} else {
			result.TitleHighlightData = nil
		}
		matched = append(matched, scored{result: result, score: score})
	}

	sort.SliceStable(matched, func(i, j int) bool { return matched[i].score > matched[j].score })

	results := make([]core.Result, 0, len(matched))
	for _, entry := range matched {
		results = append(results, entry.result)
	}
	return results, nil
}

// resultFor is one command as a result.
func (p *Plugin) resultFor(command Command, query core.Query) core.Result {
	result := core.NewResult(command.Name, command.Description, "Images/app.png", 0)
	result.Glyph = &core.GlyphInfo{FontFamily: "Segoe Fluent Icons", Glyph: commandGlyphs[command.Key]}
	result.IcoPath = commandIcons[command.Key]
	result.AutoCompleteText = command.Name
	result.Action = func(core.ActionContext) bool {
		return p.run(command, query)
	}

	// The two commands that show a folder say where it is, so that it can
	// be copied.
	switch command.Key {
	case "Open Log Location":
		result.SetCopyText(p.api.LogDirectory())
		result.AutoCompleteText = p.api.LogDirectory()
	case "Flow Launcher Tips":
		result.SetCopyText(documentation)
		result.AutoCompleteText = documentation
	case "Flow Launcher UserData Folder":
		result.SetCopyText(p.api.GetDataDirectory())
		result.AutoCompleteText = p.api.GetDataDirectory()
	}
	return result
}

// run does what a command says.
func (p *Plugin) run(command Command, query core.Query) bool {
	switch command.Key {
	case "Shutdown":
		if p.confirm("flowlauncher_plugin_sys_dlgtext_shutdown_computer", "flowlauncher_plugin_sys_shutdown_computer") {
			p.api.SaveAppAllSettings()
			p.report(p.machine.ShutDown())
		}
		return true

	case "Restart":
		if p.confirm("flowlauncher_plugin_sys_dlgtext_restart_computer", "flowlauncher_plugin_sys_restart_computer") {
			p.api.SaveAppAllSettings()
			p.report(p.machine.Restart())
		}
		return true

	case "Restart With Advanced Boot Options":
		if p.confirm("flowlauncher_plugin_sys_dlgtext_restart_computer_advanced", "flowlauncher_plugin_sys_restart_computer") {
			p.api.SaveAppAllSettings()
			p.report(p.machine.RestartToFirmwareSettings())
		}
		return true

	case "Log Off/Sign Out":
		if p.confirm("flowlauncher_plugin_sys_dlgtext_logoff_computer", "flowlauncher_plugin_sys_log_off") {
			p.report(p.machine.LogOff())
		}
		return true

	case "Lock":
		p.report(p.machine.Lock())
		return true

	case "Sleep":
		p.report(p.machine.Sleep())
		return true

	case "Hibernate":
		p.report(p.machine.Hibernate())
		return true

	case "Index Option":
		p.report(p.machine.OpenIndexingOptions())
		return true

	case "Empty Recycle Bin":
		answer := p.api.ShowMsgBox(
			core.T("", "flowlauncher_plugin_sys_dlgtext_empty_recycle_bin"),
			core.T("", "flowlauncher_plugin_sys_emptyrecyclebin"),
			core.MessageBoxYesNo, core.MessageBoxIconWarning)
		if answer == core.MessageBoxResultYes {
			p.report(p.machine.EmptyRecycleBin())
		}
		return true

	case "Open Recycle Bin":
		p.api.OpenDirectory("shell:RecycleBinFolder", "")
		return true

	case "Exit":
		p.api.HideMainWindow()
		p.api.SaveAppAllSettings()
		p.api.Exit()
		return true

	case "Save Settings":
		p.api.SaveAppAllSettings()
		p.api.ShowMsg(core.T("", "flowlauncher_plugin_sys_dlgtitle_success"),
			core.T("", "flowlauncher_plugin_sys_dlgtext_all_settings_saved"), "")
		return true

	case "Restart Flow Launcher":
		p.api.SaveAppAllSettings()
		p.api.RestartApp()
		return true

	case "Settings":
		p.api.HideMainWindow()
		p.api.OpenSettingDialog()
		return false

	case "Reload Plugin Data":
		p.api.HideMainWindow()
		if err := p.api.ReloadAllPluginData(); err != nil {
			p.api.ShowMsgError(core.T("", "flowlauncher_plugin_sys_dlgtitle_error"), err.Error())
		} else {
			p.api.ShowMsg(core.T("", "flowlauncher_plugin_sys_dlgtitle_success"),
				core.T("", "flowlauncher_plugin_sys_dlgtext_all_applicableplugins_reloaded"), "")
		}
		return false

	case "Check For Update":
		p.api.HideMainWindow()
		p.api.CheckForNewUpdate()
		return false

	case "Open Log Location":
		p.api.OpenDirectory(p.api.LogDirectory(), "")
		return true

	case "Flow Launcher Tips":
		p.api.OpenUrl(documentation, nil)
		return true

	case "Flow Launcher UserData Folder":
		p.api.OpenDirectory(p.api.GetDataDirectory(), "")
		return true

	case "Toggle Game Mode":
		p.api.ToggleGameMode()
		return true

	case "Set Flow Launcher Theme":
		// The theme selector is a keyword of its own, so the query box is
		// given it.
		if query.ActionKeyword == "" {
			p.api.ChangeQuery(themeKeyword+core.TermSeparator, false)
		} else {
			p.api.ChangeQuery(query.ActionKeyword+core.TermSeparator+themeKeyword+core.TermSeparator, false)
		}
		return false
	}
	return true
}

// confirm asks the user before turning the machine off, unless they have
// said not to be asked.
func (p *Plugin) confirm(questionKey, titleKey string) bool {
	if p.settings.SkipPowerActionConfirmation {
		return true
	}
	answer := p.api.ShowMsgBox(
		core.T("", questionKey),
		core.T("", titleKey),
		core.MessageBoxYesNo, core.MessageBoxIconWarning)
	return answer == core.MessageBoxResultYes
}

// report says that a command the machine refused did not happen.
func (p *Plugin) report(err error) {
	if err == nil {
		return
	}
	p.api.ShowMsgError(core.T("", "flowlauncher_plugin_sys_dlgtitle_error"), err.Error())
}

// themeKeyword opens the theme selector, which the settings window has.
const themeKeyword = "theme"

// commandGlyphs are the characters the commands are drawn with, as the
// reference draws them.
var commandGlyphs = map[string]string{
	"Shutdown":                           "\ue7e8",
	"Restart":                            "\ue777",
	"Restart With Advanced Boot Options": "\uecc5",
	"Log Off/Sign Out":                   "\ue77b",
	"Lock":                               "\ue72e",
	"Sleep":                              "\uec46",
	"Hibernate":                          "\ue8be",
	"Index Option":                       "\ue773",
	"Empty Recycle Bin":                  "\uea99",
	"Open Recycle Bin":                   "\ue74d",
	"Exit":                               "\ue89f",
	"Save Settings":                      "\uea35",
	"Restart Flow Launcher":              "\ue72c",
	"Settings":                           "\uf210",
	"Reload Plugin Data":                 "\ue72c",
	"Check For Update":                   "\uede4",
	"Open Log Location":                  "\uf12b",
	"Flow Launcher Tips":                 "\ue897",
	"Flow Launcher UserData Folder":      "\uf12b",
	"Toggle Game Mode":                   "\ue7fc",
	"Set Flow Launcher Theme":            "\ue790",
}

// commandIcons are the pictures the commands are drawn with.
var commandIcons = map[string]string{
	"Shutdown":                           "Images/shutdown.png",
	"Restart":                            "Images/restart.png",
	"Restart With Advanced Boot Options": "Images/restart_advanced.png",
	"Log Off/Sign Out":                   "Images/logoff.png",
	"Lock":                               "Images/lock.png",
	"Sleep":                              "Images/sleep.png",
	"Hibernate":                          "Images/hibernate.png",
	"Index Option":                       "Images/indexoption.png",
	"Empty Recycle Bin":                  "Images/recyclebin.png",
	"Open Recycle Bin":                   "Images/openrecyclebin.png",
	"Exit":                               "Images/app.png",
	"Save Settings":                      "Images/app.png",
	"Restart Flow Launcher":              "Images/app.png",
	"Settings":                           "Images/app.png",
	"Reload Plugin Data":                 "Images/app.png",
	"Check For Update":                   "Images/checkupdate.png",
	"Open Log Location":                  "Images/app.png",
	"Flow Launcher Tips":                 "Images/app.png",
	"Flow Launcher UserData Folder":      "Images/app.png",
	"Toggle Game Mode":                   "Images/app.png",
	"Set Flow Launcher Theme":            "Images/app.png",
}
