package websearch

import (
	"context"
	"embed"
	"net/url"
	"sort"
	"strings"

	"launch/core"
)

// The web search's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about the searches.
type Settings struct {
	// Sources are the searches: what each is called, what it answers to,
	// and where it looks.
	Sources []SearchSource `json:"SearchSources"`
	// SelectedSuggestion is the site whose suggestions are offered as the
	// user types.
	SelectedSuggestion string `json:"SelectedSuggestion"`
	// EnableSuggestion offers what the site suggests as the user types.
	EnableSuggestion bool `json:"EnableSuggestion"`
	// MaxSuggestions is how many suggestions to offer.
	MaxSuggestions int `json:"MaxSuggestions"`
}

// SearchSource is one place a search can be made.
type SearchSource struct {
	// Title is what the search is called.
	Title string `json:"Title"`
	// ActionKeyword is what the user types to choose it, or "*" for the
	// one that answers every query.
	ActionKeyword string `json:"ActionKeyword"`
	// Icon is the picture of the site.
	Icon string `json:"Icon"`
	// CustomIcon says the picture is one the user gave.
	CustomIcon bool `json:"CustomIcon"`
	// URL is where the search is made, with {q} where the words go.
	URL string `json:"Url"`
	// IsPrivateMode opens the search in a private window.
	IsPrivateMode bool `json:"IsPrivateMode"`
	// Enabled says whether the search is offered.
	Enabled bool `json:"Enabled"`
}

// defaultSources are the searches a user who changed nothing gets, which
// are the reference's own.
var defaultSources = []SearchSource{
	{Title: "Google", ActionKeyword: "*", Icon: "google.png", URL: "https://www.google.com/search?q={q}", Enabled: true},
	{Title: "YouTube", ActionKeyword: "youtube", Icon: "youtube.png", URL: "http://www.youtube.com/results?search_query={q}", Enabled: true},
	{Title: "Netflix", ActionKeyword: "netflix", Icon: "netflix.png", URL: "https://www.netflix.com/search?q={q}", Enabled: true},
	{Title: "Google Translate", ActionKeyword: "translate", Icon: "google_translate.png", URL: "http://translate.google.com/#auto|en|{q}", Enabled: true},
	{Title: "Gmail", ActionKeyword: "gmail", Icon: "gmail.png", URL: "https://mail.google.com/mail/ca/u/0/#apps/{q}", Enabled: true},
	{Title: "Google Drive", ActionKeyword: "drive", Icon: "drive.png", URL: "http://drive.google.com/?hl=en&tab=bo#search/{q}", Enabled: true},
	{Title: "YouTube Music", ActionKeyword: "ytmusic", Icon: "youtube_music.png", URL: "https://music.youtube.com/search?q={q}", Enabled: true},
	{Title: "Wikipedia", ActionKeyword: "wiki", Icon: "wikipedia.png", URL: "http://en.wikipedia.org/wiki/{q}", Enabled: true},
	{Title: "Facebook", ActionKeyword: "facebook", Icon: "facebook.png", URL: "http://www.facebook.com/search/?q={q}", Enabled: true},
	{Title: "Twitter", ActionKeyword: "twitter", Icon: "twitter.png", URL: "http://twitter.com/search?q={q}", Enabled: true},
	{Title: "Google Maps", ActionKeyword: "maps", Icon: "google_maps.png", URL: "http://maps.google.com/maps?q={q}", Enabled: true},
	{Title: "DuckDuckGo", ActionKeyword: "duckduckgo", Icon: "duckduckgo.png", URL: "https://duckduckgo.com/?q={q}", Enabled: true},
	{Title: "GitHub", ActionKeyword: "github", Icon: "github.png", URL: "https://github.com/search?q={q}", Enabled: true},
	{Title: "GitHub Gist", ActionKeyword: "gist", Icon: "gist.png", URL: "https://gist.github.com/search?q={q}", Enabled: true},
	{Title: "WolframAlpha", ActionKeyword: "wolframalpha", Icon: "wolframalpha.png", URL: "http://www.wolframalpha.com/input/?i={q}", Enabled: true},
	{Title: "Stack Overflow", ActionKeyword: "stackoverflow", Icon: "stackoverflow.png", URL: "http://stackoverflow.com/search?q={q}", Enabled: true},
	{Title: "I'm Feeling Lucky", ActionKeyword: "lucky", Icon: "google.png", URL: "http://google.com/search?q={q}&btnI=I", Enabled: true},
	{Title: "Google Images", ActionKeyword: "image", Icon: "google.png", URL: "https://www.google.com/search?q={q}&tbm=isch", Enabled: true},
	{Title: "Bing", ActionKeyword: "bing", Icon: "bing.png", URL: "https://www.bing.com/search?q={q}", Enabled: true},
	{Title: "Yahoo", ActionKeyword: "yahoo", Icon: "yahoo.png", URL: "https://search.yahoo.com/search?p={q}", Enabled: true},
}

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{
		Sources:            append([]SearchSource(nil), defaultSources...),
		SelectedSuggestion: "Google",
		MaxSuggestions:     5,
	}
}

// The scores a search is offered with: one that answers every query comes
// below one the user asked for, and a suggestion below both.
const (
	scoreStandard    = 50
	scoreSuggestions = 48
)

// wildcard is the keyword of a search that answers every query.
const wildcard = "*"

// Suggester offers what a site suggests as the user types. It is kept
// behind a field so that a test reads a list of its own rather than asking
// the site.
type Suggester interface {
	// Suggestions returns what the site suggests for what was typed.
	Suggestions(ctx context.Context, query string) ([]string, error)
}

// Plugin is the web search: it offers to look what the user typed up on
// the web.
type Plugin struct {
	core.BasePlugin

	api       *core.PublicAPI
	settings  Settings
	words     *core.PluginTranslator
	suggester Suggester
}

// New makes the web search.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "565B73353DBF4806919830B9202EE3BF",
			Name:           "Web Search",
			Description:    "Search the web for the query",
			Author:         "qianlifeng",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords: keywordsOf(defaultSources),
			IcoPath:        "Images/web_search.png",
		}},
		suggester: &siteSuggester{},
	}
}

// keywordsOf is every keyword the searches answer to, which is what the
// launcher files the plugin under.
func keywordsOf(sources []SearchSource) []string {
	seen := map[string]bool{}
	var keywords []string
	for _, source := range sources {
		if source.ActionKeyword == "" || seen[source.ActionKeyword] {
			continue
		}
		seen[source.ActionKeyword] = true
		keywords = append(keywords, source.ActionKeyword)
	}
	return keywords
}

// Init reads the web search's settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")
	p.suggester = &siteSuggester{api: ctx.API, site: ""}

	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "WebSearch", "Settings.json")
	if len(p.settings.Sources) == 0 {
		p.settings = DefaultSettings()
	}
	if p.settings.MaxSuggestions <= 0 {
		p.settings.MaxSuggestions = 5
	}
	if site := p.settings.SelectedSuggestion; site != "" {
		p.suggester = &siteSuggester{api: ctx.API, site: site}
	}
	return nil
}

// Query offers to look what the user typed up on each search that answers
// to the keyword they used, and what the site suggests.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	// A path is not something to look up.
	if looksLikePath(query.Search) {
		return nil, nil
	}

	var results []core.Result
	for _, source := range p.settings.Sources {
		if !source.Enabled {
			continue
		}
		if source.ActionKeyword != query.ActionKeyword && source.ActionKeyword != wildcard {
			continue
		}

		// A search that answers every query is given the whole query; one
		// the user asked for is given what they typed after its keyword.
		keyword := query.Search
		if source.ActionKeyword == wildcard {
			keyword = query.TrimmedQuery
		}

		title := keyword
		subtitle := p.words.T("flowlauncher_plugin_websearch_search") + " " + source.Title
		score := scoreStandard
		if source.ActionKeyword != wildcard {
			score = scoreStandard + 1
		}

		// A search with nothing to look up is offered by name, so that the
		// user learns it is there.
		if strings.TrimSpace(keyword) == "" {
			result := core.NewResult(subtitle, "", p.iconFor(source), score)
			results = append(results, result)
			continue
		}

		address := searchAddress(source, keyword)
		result := core.NewResult(title, subtitle, p.iconFor(source), score)
		result.ContextData = address
		if source.ActionKeyword != wildcard {
			result.ActionKeywordAssigned = source.ActionKeyword
		}
		result.Action = func(core.ActionContext) bool {
			p.api.OpenWebUrl(address, privateOrNil(source.IsPrivateMode))
			return true
		}
		results = append(results, result)

		results = append(results, p.suggestions(ctx, keyword, subtitle, source)...)
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results, nil
}

// suggestions are what the site suggests for what was typed, offered below
// the search itself.
func (p *Plugin) suggestions(ctx context.Context, keyword, subtitle string, source SearchSource) []core.Result {
	if !p.settings.EnableSuggestion || strings.TrimSpace(keyword) == "" {
		return nil
	}

	found, err := p.suggester.Suggestions(ctx, keyword)
	if err != nil || len(found) == 0 {
		return nil
	}
	if len(found) > p.settings.MaxSuggestions {
		found = found[:p.settings.MaxSuggestions]
	}

	score := scoreSuggestions
	if source.ActionKeyword != wildcard {
		score = scoreSuggestions + 1
	}

	results := make([]core.Result, 0, len(found))
	for _, suggestion := range found {
		address := searchAddress(source, suggestion)
		result := core.NewResult(suggestion, subtitle, p.iconFor(source), score)
		result.ContextData = address
		if source.ActionKeyword != wildcard {
			result.ActionKeywordAssigned = source.ActionKeyword
		}
		result.Action = func(core.ActionContext) bool {
			p.api.OpenWebUrl(address, privateOrNil(source.IsPrivateMode))
			return true
		}
		results = append(results, result)
	}
	return results
}

// LoadContextMenus offers to copy where a search would go.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	address, ok := selectedResult.ContextData.(string)
	if !ok || address == "" {
		return nil
	}

	item := core.NewResult(
		p.words.T("flowlauncher_plugin_websearch_copyurl_title"),
		p.words.T("flowlauncher_plugin_websearch_copyurl_subtitle"),
		"Images/copylink.png", 0)
	item.Action = func(core.ActionContext) bool {
		p.api.CopyToClipboard(address, false, true)
		return true
	}
	return []core.Result{item}
}

// iconFor is where a search's picture is.
func (p *Plugin) iconFor(source SearchSource) string {
	if source.Icon == "" {
		return "Images/web_search.png"
	}
	return "Images/" + source.Icon
}

// searchAddress is where a search looks, with the words escaped and put
// where the search says they go.
func searchAddress(source SearchSource, keyword string) string {
	return strings.ReplaceAll(source.URL, "{q}", url.QueryEscape(keyword))
}

// privateOrNil says whether to open a private window, which the launcher
// takes as nothing when it should not.
func privateOrNil(private bool) *bool {
	if !private {
		return nil
	}
	return &private
}

// looksLikePath reports whether what was typed is a path rather than
// something to look up.
func looksLikePath(search string) bool {
	return strings.ContainsAny(search, `/\`) || strings.Contains(search, ":\\")
}
