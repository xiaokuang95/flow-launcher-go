package core

import "time"

// ThemeData is a theme the launcher can wear.
type ThemeData struct {
	// FileNameWithoutExtension is the theme's file name.
	FileNameWithoutExtension string
	// Name is what the settings window shows.
	Name string
	// IsDark says whether the theme is dark; nil leaves it to the theme's
	// own colours.
	IsDark *bool
	// HasBlur says whether the theme blurs what is behind the window.
	HasBlur *bool
}

// String is the theme's name.
func (t ThemeData) String() string { return t.Name }

// UserPlugin is a plugin of the community's, as the plugin store lists it.
type UserPlugin struct {
	ID            string `json:"ID"`
	Name          string `json:"Name"`
	Description   string `json:"Description"`
	Author        string `json:"Author"`
	Version       string `json:"Version"`
	Language      string `json:"Language"`
	Website       string `json:"Website"`
	UrlDownload   string `json:"UrlDownload"`
	UrlSourceCode string `json:"UrlSourceCode"`

	// LocalInstallPath is where a plugin installed from a file was put.
	LocalInstallPath string `json:"LocalInstallPath"`
	// IcoPath is the plugin's icon.
	IcoPath string `json:"IcoPath"`

	// LatestReleaseDate is when the plugin last changed, and DateAdded when
	// the user installed it.
	LatestReleaseDate *time.Time `json:"LatestReleaseDate"`
	DateAdded         *time.Time `json:"DateAdded"`

	// MinimumAppVersion is the launcher version the plugin needs.
	MinimumAppVersion string `json:"MinimumAppVersion"`
}

// IsFromLocalInstallPath reports whether the plugin was installed from a
// file rather than from the store.
func (p UserPlugin) IsFromLocalInstallPath() bool { return p.LocalInstallPath != "" }

// AllowedLanguage says which languages a plugin may be written in.
type AllowedLanguage struct{}

// The languages the launcher can load plugins of.
const (
	LanguageCSharp       = "csharp"
	LanguageFSharp       = "fsharp"
	LanguagePython       = "python"
	LanguagePythonV2     = "python_v2"
	LanguageExecutable   = "executable"
	LanguageExecutableV2 = "executable_v2"
	LanguageJavaScript   = "javascript"
	LanguageTypeScript   = "typescript"
	LanguageTypeScriptV2 = "typescript_v2"
)

// IsDotNet reports whether a language is one the launcher would need a
// .NET host to run.
func IsDotNet(language string) bool {
	return language == LanguageCSharp || language == LanguageFSharp
}
