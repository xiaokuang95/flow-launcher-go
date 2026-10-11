# Writing a plugin

A plugin is a Go package under `plugins/`, and it answers a query with
results. Everything a plugin can do is in package `core`.

## The shape of one

```go
package example

import (
	"context"

	"launch/core"
)

// Plugin answers a query with an example.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	settings Settings
}

// New makes the plugin.
func New() *Plugin {
	return &Plugin{BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
		ID:             "0ECADE17459B49F587BF81DC3A125110",
		Name:           "Example",
		Description:    "What it does, in the words the reference uses.",
		Author:         "qianlifeng",
		Version:        "1.0.0",
		Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
		ActionKeywords: []string{"*"},
		IcoPath:        "Images/example.png",
	}}}
}

// Init is called once, before the first query.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.settings = loadSettings(ctx.API)
	return nil
}

// Query answers a query.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	...
}
```

`core.BasePlugin` provides `Metadata` and an `Init` that does nothing, so a
plugin is its metadata, its `Init` and its `Query`.

## The metadata

The id, name, description, author, version and action keywords are the
reference's own, from the plugin's `plugin.json` under
`C:/tmp/flow-ref/Plugins/<plugin>/plugin.json`. Keep them: they are what
the settings window shows and what the launcher stores the user's settings
under.

`ActionKeywords` decides when the plugin is asked:

- `"*"` — every query.
- `">"` — only when the query starts with `>`, which the launcher takes off
  before the plugin sees it.
- several — the plugin answers to any of them, as the web search plugin
  does with `sc`, `wiki` and the rest.

## The query

`core.Query` is what the plugin is asked:

| field | what it is |
|---|---|
| `OriginalQuery` | exactly what the user typed |
| `TrimmedQuery` | the same without surrounding whitespace, with the custom shortcuts expanded |
| `Search` | the query without the action keyword: what to match against |
| `SearchTerms` | `Search` split at its spaces |
| `ActionKeyword` | the keyword that chose the plugin, empty for a global one |
| `IsHomeQuery` | the user typed nothing |
| `IsReQuery` | the user pressed Ctrl+R, so cached results must not be served |

`FirstSearch()`, `SecondSearch()`, `ThirdSearch()` and `SecondToEndSearch()`
read the terms.

## The results

`core.Result` is one row. The fields a plugin usually sets:

| field | what it is |
|---|---|
| `Title` | the first line, required |
| `SubTitle` | the second line; Ctrl+C copies it when there is no `SetCopyText` |
| `IcoPath` | the icon: a path inside the plugin's directory, an absolute path, or a URL |
| `Score` | the priority among the other plugins' results |
| `Action` | what running the result does; it reports whether the launcher hides |
| `TitleHighlightData` | the rune indexes of `Title` the query matched, which are shown in the accent colour |
| `ActionKeywordAssigned` | for a plugin with several keywords, which one found it |
| `AutoCompleteText` | what Tab leaves in the query box |
| `ContextData` | anything the plugin wants back in its context menu |
| `ProgressBar` | a progress bar over the result, from 0 to 100 |
| `Preview` | what the preview panel shows |
| `AddSelectedCount` | false for a plugin that keeps its own order |
| `RecordKey` | identifies the result when its title changes between queries |

`core.NewResult(title, subTitle, icoPath, score)` makes one with the
defaults filled in.

### Matching

`core.StringMatcher` is the reference's own matcher, and a plugin uses it to
score and to fill `TitleHighlightData`:

```go
matcher := core.StringMatcher{
	UserSettingSearchPrecision: core.SearchPrecisionRegular,
	IgnoreAccents:              p.api.Matcher().IgnoreAccents,
}
match := matcher.FuzzyMatch(query.Search, title)
if !match.IsSearchPrecisionScoreMet() {
	return nil, nil
}
result := core.NewResult(title, subTitle, icoPath, match.Score())
result.TitleHighlightData = match.MatchData
```

`p.api.Matcher()` is the matcher the launcher is configured with, which
carries the user's accent setting.

### The action

```go
result.Action = func(action core.ActionContext) bool {
	if action.SpecialKeyState.CtrlPressed {
		p.api.OpenDirectory(directory, file) // Ctrl+Enter
		return true
	}
	p.api.ShellRun(exe, "")
	return true
}
```

The action runs on a goroutine of its own, so it may block. Returning true
hides the launcher; returning false keeps it in front. `Action` must never
run something destructive while a test runs: give the plugin a field for
what it does to the machine, so that a test can put a fake there.

## What else a plugin can implement

| interface | what it adds |
|---|---|
| `ContextMenuPlugin` | items on the menu a result shows |
| `HomePlugin` | results for the home query, the one with nothing typed |
| `PluginI18n` | the plugin's name and description in the user's language |
| `ReloadablePlugin` | reading its data again, as F5 asks |
| `SettingProviderPlugin` | the settings the settings window shows |
| `ExternalPreviewPlugin` | a preview in a window of its own |

## The API

`core.PublicAPI` is what a plugin calls back into. The parts a plugin
usually wants:

- **The window**: `ChangeQuery`, `ReQuery`, `ShowMainWindow`,
  `HideMainWindow`, `FocusQueryTextBox`, `StartLoadingBar`, `StopLoadingBar`
- **The machine**: `OpenDirectory`, `OpenUrl`, `OpenWebUrl`, `OpenAppUri`,
  `ShellRun`, `CopyToClipboard`
- **The settings**: `OpenSettingDialog`, `OpenPluginSettingsWindow`,
  `SaveAppAllSettings`, `SavePluginSettings`
- **The plugins**: `GetAllPlugins`, `GetAllInitializedPlugins`,
  `AddActionKeyword`, `RemoveActionKeyword`, `ActionKeywordAssigned`
- **The network**: `HttpGetStringAsync`, `HttpGetStreamAsync`,
  `HttpDownloadAsync`
- **The log**: `LogDebug`, `LogInfo`, `LogWarn`, `LogError`, `LogException`
- **Messages**: `ShowMsg`, `ShowMsgError`, `ShowMsgWithButton`, `ShowMsgBox`
- **The theme**: `IsApplicationDarkTheme`, `GetAvailableThemes`,
  `GetCurrentTheme`, `SetCurrentTheme`
- **Fuzzy search**: `FuzzySearch(query, text)`

## The plugin's own settings and cache

```go
type Settings struct {
	Enabled bool `json:"Enabled"`
}

settings := core.LoadSettingJsonStorage[Settings](api, "Example", "Settings.json")
core.SaveSettingJsonStorage(api, "Example", "Settings.json", settings)
```

The file goes to `<dataDir>/Settings/Plugins/<plugin name>/`. A cache goes
to `<dataDir>/Cache/Plugins/<plugin name>/` with `LoadCacheBinaryStorage`
and `SaveCacheBinaryStorage`.

## The words on the screen

There are two kinds of word, and they come from two places.

**The launcher's own words** are the settings window, the tray menu and the
messages the launcher itself shows. They are keys of the launcher's
`core/languages/*.xaml`, which the reference keeps in
`Flow.Launcher/Languages/`:

```go
core.T("", "searchplugin_Noresult_Title")     // the current language
core.Tf("", "pluginFailedToRespond", name)    // with its placeholders
```

`go run ./tools/i18nkeys en <pattern>` prints those keys and their English
text, which is how to find the one to use. A key that does not exist is
shown as itself, so a wrong key is visible on the screen rather than
missing.

**A plugin's own words** are the ones only it says, and the reference keeps
them in the plugin's own `Languages/<code>.xaml`, beside its code. Copy
those files into the plugin package and read them the same way:

```go
//go:embed Languages/*.xaml
var languages embed.FS

p.words = core.NewPluginTranslator(languages, "Languages")
...
subtitle := p.words.T("flowlauncher_plugin_calculator_copy_number_to_clipboard")
```

Their keys are namespaced by the plugin (`flowlauncher_plugin_<name>_...`),
so they never collide with the launcher's. `plugins/calculator/` is the
worked example.

A plugin with nothing of its own to say needs neither.

## Tests

A plugin is tested without the machine under it:

```go
func TestQuery(t *testing.T) {
	plugin := New()
	plugin.Init(&core.PluginInitContext{API: core.NewPublicAPI(nil, t.TempDir(), "test")})

	results, err := plugin.Query(context.Background(), core.Query{Search: "example"})
	...
}
```

- `core.NewPublicAPI(nil, t.TempDir(), "test")` gives an API whose hooks do
  nothing, so a test never opens a window or touches the clipboard.
- Give the plugin a field for whatever it does to the machine, fill it with
  the real thing in `New`, and put a fake there in the test. A test must
  never lock the screen, shut the machine down, kill a process or open a
  window.
- Where the reference has tests for the plugin, port them: they are the
  spec. They are under `C:/tmp/flow-ref/Flow.Launcher.Test/Plugins/`.
- Where the reference's own implementation can be run, compare against it:
  `tools/oracle` runs the reference's C# over cases and answers with what it
  said. `core/oracle_test.go` and `plugins/calculator/calculator_test.go`
  show how.
