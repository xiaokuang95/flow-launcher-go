package pluginindicator

import (
	"context"
	"embed"
	"sort"

	"launch/core"
)

// The plugin indicator's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Plugin is the plugin indicator: it lists what each plugin answers to, so
// that the user can learn the keywords without reading the settings.
type Plugin struct {
	core.BasePlugin

	api   *core.PublicAPI
	words *core.PluginTranslator
}

// New makes the plugin indicator.
func New() *Plugin {
	return &Plugin{BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
		ID:             "6A122269676E40EB86EB543B945932B9",
		Name:           "Plugin Indicator",
		Description:    "Provides plugin action keyword suggestions",
		Author:         "qianlifeng",
		Version:        "1.0.0",
		Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
		ActionKeywords: []string{"?"},
		IcoPath:        "Images/work.png",
	}}}
}

// Init is called once, before the first query.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")
	return nil
}

// Query lists the action keywords that match what the user typed, or every
// one of them when they have typed nothing.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	return p.results(query.Search), nil
}

// HomeQuery lists every action keyword on the home page, which is what the
// reference does with its home query.
func (p *Plugin) HomeQuery(ctx context.Context) ([]core.Result, error) {
	return p.results(""), nil
}

// results lists the keywords of every plugin that answers to one, with the
// plugin each belongs to.
func (p *Plugin) results(search string) []core.Result {
	matcher := p.api.Matcher()

	type entry struct {
		keyword string
		plugin  core.PluginMetadata
		score   int
	}
	var entries []entry

	seen := map[string]bool{}
	for _, pair := range p.api.GetAllPlugins() {
		plugin := pair.Metadata
		if plugin.Disabled {
			continue
		}
		for _, keyword := range plugin.ActionKeywords {
			// A plugin that answers every query has no keyword to show.
			if keyword == core.GlobalPluginWildcardSign || keyword == "" {
				continue
			}
			// Two plugins may share a keyword, and both are shown; the
			// same keyword twice for one plugin is not.
			key := keyword + "\x00" + plugin.ID
			if seen[key] {
				continue
			}
			seen[key] = true

			// The keyword is what the user is looking for, so a keyword
			// that matches is a better answer than a plugin whose name
			// matches.
			keywordMatch := matcher.FuzzyMatch(search, keyword)
			score := 0
			matched := search == ""
			if keywordMatch.IsSearchPrecisionScoreMet() {
				score = keywordMatch.Score()
				matched = true
			} else if nameMatch := matcher.FuzzyMatch(search, plugin.Name); nameMatch.IsSearchPrecisionScoreMet() {
				score = nameMatch.Score()
				matched = true
			}
			if !matched {
				continue
			}

			entries = append(entries, entry{keyword: keyword, plugin: plugin, score: score})
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		return entries[i].keyword < entries[j].keyword
	})

	results := make([]core.Result, 0, len(entries))
	for _, entry := range entries {
		keyword := entry.keyword
		plugin := entry.plugin

		result := core.NewResult(keyword,
			p.words.Tf("flowlauncher_plugin_pluginindicator_result_subtitle", plugin.Name),
			plugin.IcoPath, entry.score)
		result.PluginDirectory = plugin.PluginDirectory
		result.ContextData = plugin
		result.AutoCompleteText = keyword + core.TermSeparator
		result.Action = func(core.ActionContext) bool {
			// Choosing a keyword puts it in the query box, so that the
			// user can carry on typing what they want to search for.
			p.api.ChangeQuery(keyword+core.TermSeparator, false)
			return false
		}
		results = append(results, result)
	}
	return results
}

// LoadContextMenus offers the settings page of the plugin a keyword belongs
// to.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	plugin, ok := selectedResult.ContextData.(core.PluginMetadata)
	if !ok {
		return nil
	}

	item := core.NewResult(
		p.words.Tf("flowlauncher_plugin_pluginindicator_plugin_settings_title", plugin.Name),
		p.words.Tf("flowlauncher_plugin_pluginindicator_plugin_settings_subtitle", plugin.Name),
		plugin.IcoPath, 0)
	item.PluginDirectory = plugin.PluginDirectory
	item.Action = func(core.ActionContext) bool {
		return p.api.OpenPluginSettingsWindow(plugin.ID)
	}
	return []core.Result{item}
}
