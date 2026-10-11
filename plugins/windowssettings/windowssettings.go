package windowssettings

import (
	"context"
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"launch/core"
)

// The settings pages of Windows, which the reference keeps as data: what
// each page is called, the part of the settings it is in, and what opens
// it.
//
//go:embed WindowsSettings.json
var settingsData []byte

// The icons the pages are drawn with: the settings of an app, and the
// older control panels.
const (
	windowsSettingsIcon = "Images/WindowsSettings.png"
	controlPanelIcon    = "Images/ControlPanel.png"
)

// The kinds of page, which say where a page is opened from.
const (
	typeAppSettingsApp = "AppSettingsApp"
	typeControlPanel   = "ControlPanel"
	typeTaskLink       = "TaskLink"
)

// Setting is one page of the Windows settings.
type Setting struct {
	// Name is what the page is called, as a key of the reference's own
	// words.
	Name string `json:"Name"`
	// Area is the part of the settings it is in, as a key of the same.
	Area string `json:"Area"`
	// Type is where the page is opened from.
	Type string `json:"Type"`
	// AltNames are the other things the page is called.
	AltNames []string `json:"AltNames"`
	// Command is what opens the page.
	Command string `json:"Command"`
	// IconGlyph is the character the page is drawn with.
	IconGlyph *Glyph `json:"IconGlyph"`
	// Keywords are the words the page answers to.
	Keywords [][]string `json:"Keywords"`
	// Note is what the reference says about a page that needs care.
	Note string `json:"Note"`
}

// Glyph is a character of a font, which is how the reference draws an
// icon.
type Glyph struct {
	Glyph      string `json:"Glyph"`
	FontFamily string `json:"FontFamily"`
}

// Plugin is the Windows settings: it finds a page of the settings by what
// it is called.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	words    *translator
	settings []Setting
	opener   Opener
}

// Opener opens a page of the settings. It is kept behind a field so that a
// test watches what was asked for rather than opening the settings.
type Opener interface {
	// Open opens what a settings page's command names.
	Open(command string) error
}

// New makes the Windows settings.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "50452934-1d5c-4510-b9a8-4b3e7e2c0e36",
			Name:           "Windows Settings",
			Description:    "Search and open Windows settings",
			Author:         "Yusyuriv",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords: []string{"s"},
			IcoPath:        windowsSettingsIcon,
		}},
		opener: systemOpener{},
	}
}

// Init reads the settings pages.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = newTranslator()

	if err := json.Unmarshal(settingsData, &p.settings); err != nil {
		return err
	}
	return nil
}

// Query answers what the user typed with the settings pages whose name,
// part of the settings or other name matches it.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	search := strings.TrimSpace(query.Search)
	matcher := p.api.Matcher()

	if search == "" {
		results := make([]core.Result, 0, len(p.settings))
		for _, setting := range p.settings {
			results = append(results, p.resultFor(setting, 100, nil))
		}
		return results, nil
	}

	// A part of the settings that matched is worth less than a page's own
	// name, so that a page called by what it does comes first.
	const nonNameMatchScoreAdj = 10

	var results []core.Result
	for _, setting := range p.settings {
		name := p.nameOf(setting)

		if nameMatch := matcher.FuzzyMatch(search, name); nameMatch.IsSearchPrecisionScoreMet() {
			results = append(results, p.resultFor(setting, nameMatch.Score(), nameMatch.MatchData))
			continue
		}

		if areaMatch := matcher.FuzzyMatch(search, p.areaOf(setting)); areaMatch.IsSearchPrecisionScoreMet() {
			results = append(results, p.resultFor(setting, areaMatch.Score()-nonNameMatchScoreAdj, nil))
			continue
		}

		// A page is also found by the other things it is called.
		matched := false
		for _, altName := range setting.AltNames {
			if altMatch := matcher.FuzzyMatch(search, altName); altMatch.IsSearchPrecisionScoreMet() {
				results = append(results, p.resultFor(setting, altMatch.Score()-nonNameMatchScoreAdj, nil))
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		// And by the words it answers to, which all have to be there.
		if len(setting.Keywords) > 0 && query.SearchTerms != nil {
			if allTermsInKeywords(query.SearchTerms, setting.Keywords) {
				results = append(results, p.resultFor(setting, nonNameMatchScoreAdj, nil))
			}
		}
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results, nil
}

// allTermsInKeywords reports whether every word the user typed is one of
// the words a page answers to.
func allTermsInKeywords(terms []string, keywords [][]string) bool {
	for _, term := range terms {
		found := false
		for _, group := range keywords {
			for _, keyword := range group {
				if strings.EqualFold(keyword, term) {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// resultFor is one settings page as a result.
func (p *Plugin) resultFor(setting Setting, score int, highlight []int) core.Result {
	// A page that is a shortcut to something else is worth less, so that
	// the page itself comes first.
	if setting.Type == typeTaskLink {
		score -= 50
	}

	icon := controlPanelIcon
	if setting.Type == typeAppSettingsApp {
		icon = windowsSettingsIcon
	}

	result := core.NewResult(p.nameOf(setting), p.subtitleOf(setting), icon, score)
	result.TitleHighlightData = highlight
	result.ContextData = setting
	result.SetCopyText(setting.Command)
	if setting.IconGlyph != nil {
		result.Glyph = &core.GlyphInfo{
			FontFamily: strings.TrimPrefix(setting.IconGlyph.FontFamily, "/Resources/#"),
			Glyph:      setting.IconGlyph.Glyph,
		}
	}
	result.Action = func(core.ActionContext) bool {
		if err := p.opener.Open(setting.Command); err != nil {
			p.api.ShowMsgError(p.Meta.Name, err.Error())
		}
		return true
	}
	return result
}

// nameOf is what a page is called in the launcher's language.
func (p *Plugin) nameOf(setting Setting) string { return p.words.t(setting.Name) }

// areaOf is the part of the settings a page is in.
func (p *Plugin) areaOf(setting Setting) string { return p.words.t("Area" + setting.Area) }

// subtitleOf is where a page is opened from, and what part of the settings
// it is in.
func (p *Plugin) subtitleOf(setting Setting) string {
	kind := p.words.t(typeControlPanel)
	if setting.Type == typeAppSettingsApp {
		kind = p.words.t(typeAppSettingsApp)
	}
	return kind + " > " + p.areaOf(setting)
}

// LoadContextMenus offers the command a page is opened with, so that it can
// be used somewhere else.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	setting, ok := selectedResult.ContextData.(Setting)
	if !ok {
		return nil
	}

	item := core.NewResult(p.words.t("CopyCommand"), setting.Command, "Images/copy.png", 0)
	item.Action = func(core.ActionContext) bool {
		p.api.CopyToClipboard(setting.Command, false, true)
		return true
	}
	return []core.Result{item}
}
