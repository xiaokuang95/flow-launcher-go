package calculator

import (
	"context"
	"embed"
	"regexp"
	"strings"

	"launch/core"
)

// The pictures this plugin shows its results with, which the launcher
// reads from the plugin's own directory.
//
//go:embed Images/*.png
var images embed.FS

// The calculator's own words, which it ships in the languages the
// reference does.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Plugin is the calculator: it answers an arithmetic expression with its
// value.
type Plugin struct {
	core.BasePlugin

	settings Settings
	api      *core.PublicAPI
	words    *core.PluginTranslator
}

// New makes the calculator.
func New() *Plugin {
	return &Plugin{BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
		ID:             "CEA0FDFC6D3B4085823D60DC76F28855",
		Name:           "Calculator",
		Description:    "Perform mathematical calculations, including hex values and advanced functions such as 'min(1,2,3)', 'sqrt(123)' and 'cos(123)'.",
		Author:         "cxfksword, dcog989",
		Version:        "1.0.0",
		Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
		ActionKeywords: []string{"*"},
		IcoPath:        "Images/calculator.png",
	}}}
}

// installImages writes the pictures this plugin ships where the launcher
// reads them from, which is the plugin's own directory.
func (p *Plugin) installImages(ctx *core.PluginInitContext) {
	if err := core.InstallImages(ctx.CurrentPluginMetadata.PluginDirectory, images, "Images/*.png"); err != nil {
		ctx.API.LogError("calculator", "writing the plugin's pictures failed: "+err.Error())
	}
}

// Init reads the plugin's settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.installImages(ctx)
	p.api = ctx.API
	p.settings = loadSettings(ctx.API)
	p.words = core.NewPluginTranslator(languages, "Languages")
	return nil
}

// Query answers an arithmetic expression with its value.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	search := strings.TrimSpace(query.Search)
	if search == "" {
		return nil, nil
	}

	value, err := Evaluate(search)
	if err != nil {
		return p.errorResult(), nil
	}

	decimal, group := p.settings.separators(core.CurrentLanguage())
	formatted, err := FormatNumber(value, p.settings.MaxDecimalPlaces, decimal, group, p.settings.UseThousandsSeparator)
	if err != nil {
		return p.errorResult(), nil
	}
	if formatted == "" {
		return nil, nil
	}

	result := core.NewResult(formatted, p.words.T("flowlauncher_plugin_calculator_copy_number_to_clipboard"), "Images/calculator.png", 300)
	result.SetCopyText(formatted)
	result.Action = func(core.ActionContext) bool {
		p.api.CopyToClipboard(formatted, false, true)
		return true
	}
	return []core.Result{result}, nil
}

// errorResult is what the calculator answers when it cannot work an
// expression out, which the user can turn off while they type.
func (p *Plugin) errorResult() []core.Result {
	if !p.settings.ShowErrorMessage {
		return nil
	}
	return []core.Result{
		core.NewResult(p.words.T("flowlauncher_plugin_calculator_expression_not_complete"), "", "Images/calculator.png", 0),
	}
}

// Expression-like input is what the calculator looks at before working it
// out, so that it does not answer every query with "not a number".
var numberLike = regexp.MustCompile(`^[\s\d\.\+\-\*/%^()!,eExX]+$`)

// LooksLikeExpression reports whether the text could be an expression, so
// that the calculator stays out of the way of a search for a word.
func LooksLikeExpression(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if !numberLike.MatchString(text) {
		return false
	}
	// A run of digits on its own is a search, not an expression.
	hasOperator := strings.ContainsAny(text, "+-*/%^!")
	return hasOperator
}
