package calculator

import (
	"strings"

	"launch/core"
)

// Settings is what the user can change about the calculator.
type Settings struct {
	// DecimalSeparator is the mark between a number's whole part and its
	// fraction. "system" follows the launcher's language.
	DecimalSeparator string `json:"DecimalSeparator"`
	// MaxDecimalPlaces is how far the answer is rounded.
	MaxDecimalPlaces int `json:"MaxDecimalPlaces"`
	// ShowErrorMessage shows "expression not complete" while the user is
	// still typing, rather than nothing.
	ShowErrorMessage bool `json:"ShowErrorMessage"`
	// UseThousandsSeparator groups the digits of a long number.
	UseThousandsSeparator bool `json:"UseThousandsSeparator"`
}

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{
		DecimalSeparator:      "system",
		MaxDecimalPlaces:      10,
		ShowErrorMessage:      false,
		UseThousandsSeparator: true,
	}
}

// The marks a number can be written with.
const (
	separatorSystem = "system"
	separatorDot    = "."
	separatorComma  = ","
)

// numberFormat is how a language writes numbers, which the reference reads
// from the desktop's culture.
type numberFormat struct {
	decimal string
	group   string
}

// formatsByLanguage are the number formats of the languages the launcher
// speaks, as .NET's cultures define them.
var formatsByLanguage = map[string]numberFormat{
	"en":         {".", ","},
	"zh-cn":      {".", ","},
	"zh-tw":      {".", ","},
	"ja":         {".", ","},
	"ko":         {".", ","},
	"he":         {".", ","},
	"es-419":     {".", ","},
	"uk-UA":      {",", "\u00a0"},
	"ru":         {",", "\u00a0"},
	"fr":         {",", "\u202f"},
	"nl":         {",", "."},
	"pl":         {",", "\u00a0"},
	"da":         {",", "."},
	"de":         {",", "."},
	"sr":         {",", "."},
	"sr-Cyrl-RS": {",", "."},
	"pt-pt":      {",", "\u00a0"},
	"pt-br":      {",", "."},
	"es":         {",", "."},
	"it":         {",", "."},
	"nb-NO":      {",", "\u00a0"},
	"sk":         {",", "\u00a0"},
	"tr":         {",", "."},
	"cs":         {",", "\u00a0"},
	"ar":         {"\u066b", "\u066c"},
	"vi-vn":      {",", "."},
}

// formatFor is how a language writes numbers, defaulting to the English
// way.
func formatFor(language string) numberFormat {
	if format, ok := formatsByLanguage[strings.ToLower(language)]; ok {
		return format
	}
	return formatsByLanguage["en"]
}

// separators are the marks this run of the calculator writes numbers with,
// which the settings and the launcher's language decide between them.
func (s Settings) separators(language string) (decimal, group string) {
	system := formatFor(language)

	switch s.DecimalSeparator {
	case separatorDot:
		group = system.group
		if group == separatorDot {
			group = separatorComma
		}
		return separatorDot, group
	case separatorComma:
		group = system.group
		if group == separatorComma {
			group = separatorDot
		}
		return separatorComma, group
	default:
		return system.decimal, system.group
	}
}

// loadSettings reads the calculator's settings, or makes new ones.
func loadSettings(api *core.PublicAPI) Settings {
	settings := core.LoadSettingJsonStorage[Settings](api, "Calculator", "Settings.json")
	if settings.DecimalSeparator == "" {
		settings = DefaultSettings()
	}
	return settings
}

// saveSettings writes them out.
func saveSettings(api *core.PublicAPI, settings Settings) {
	_ = core.SaveSettingJsonStorage(api, "Calculator", "Settings.json", settings)
}
