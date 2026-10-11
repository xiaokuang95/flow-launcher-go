package core

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// ReferenceTheme is one of the reference's themes: what it is called, and
// the colours it draws the window with.
type ReferenceTheme struct {
	FileName string
	Name     string
	IsDark   bool
	HasBlur  bool
	Colors   ThemeColors
}

// AvailableThemes are the themes the launcher can wear, with the ones the
// launcher draws itself first.
func AvailableThemes() []ThemeData {
	themes := []ThemeData{
		{FileNameWithoutExtension: DefaultThemeName, Name: DefaultThemeName},
	}

	for _, theme := range ReferenceThemes {
		isDark := theme.IsDark
		hasBlur := theme.HasBlur
		themes = append(themes, ThemeData{
			FileNameWithoutExtension: theme.FileName,
			Name:                     theme.Name,
			IsDark:                   &isDark,
			HasBlur:                  &hasBlur,
		})
	}
	return themes
}

// FindTheme is the theme with a file name, if it is one of the reference's.
func FindTheme(fileName string) (ReferenceTheme, bool) {
	for _, theme := range ReferenceThemes {
		if strings.EqualFold(theme.FileName, fileName) {
			return theme, true
		}
	}
	return ReferenceTheme{}, false
}

// PaintTheme paints one of the reference's themes onto the theme the
// widgets draw with: the colours the theme names are used, and the rest of
// the theme it was drawn from is kept, so that the parts the reference's
// themes say nothing about still look right.
//
// A theme the launcher cannot read leaves the theme alone, which is what
// the launcher's own "System" theme does.
func PaintTheme(theme *ui.Theme, reference ReferenceTheme) {
	applyColors(theme, reference.Colors, reference.IsDark)
}

// applyColors paints a theme's colours onto the theme the widgets draw
// with.
func applyColors(theme *ui.Theme, colors ThemeColors, isDark bool) {
	if colors.Background != "" {
		theme.Background = colorOf(colors.Background, theme.Background)
		theme.Surface = theme.Background
	}
	if colors.Border != "" {
		theme.Border = colorOf(colors.Border, theme.Border)
	}
	if colors.Title != "" {
		theme.Text = colorOf(colors.Title, theme.Text)
	}
	if colors.Subtitle != "" {
		theme.TextMuted = colorOf(colors.Subtitle, theme.TextMuted)
	}
	if colors.Highlight != "" {
		theme.Accent = colorOf(colors.Highlight, theme.Accent)
	} else if colors.TitleSelected != "" {
		theme.Accent = colorOf(colors.TitleSelected, theme.Accent)
	}
	if colors.SelectedBackground != "" {
		theme.Selection = colorOf(colors.SelectedBackground, theme.Selection)
		theme.SurfaceHover = theme.Selection
	}

	// A theme's colours are for a window with a background; one that is
	// drawn dark needs its text to stand out against it.
	if isDark {
		theme.Inverse = theme.Text
		theme.InverseText = theme.Background
	}
}

// colorOf reads a colour, falling back to what the theme already has when
// the reference writes one this launcher cannot read.
func colorOf(value string, fallback ui.Color) ui.Color {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !strings.HasPrefix(value, "#") {
		return fallback
	}
	// Go reads #RRGGBB and #RRGGBBAA; the reference writes both that and
	// #AARRGGBB, which the extractor turns round.
	if len(value) != 4 && len(value) != 7 && len(value) != 9 {
		return fallback
	}
	return ui.Hex(value)
}

// ThemeNames are the names of the themes the launcher can wear, sorted.
func ThemeNames() []string {
	names := make([]string, 0, len(ReferenceThemes)+1)
	names = append(names, DefaultThemeName)
	for _, theme := range ReferenceThemes {
		names = append(names, theme.Name)
	}
	sort.Strings(names)
	return names
}
