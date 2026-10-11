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
// dark says which of the theme's two backgrounds to use, which is what the
// reference picks from the user's colour scheme: a theme such as Blur White
// or Circle System draws on either, and the desktop decides which.
//
// A theme the launcher cannot read leaves the theme alone, which is what
// the launcher's own "System" theme does.
func PaintTheme(theme *ui.Theme, reference ReferenceTheme, dark bool) {
	applyColors(theme, reference.Colors, dark)
}

// applyColors paints a theme's colours onto the theme the widgets draw
// with.
func applyColors(theme *ui.Theme, colors ThemeColors, dark bool) {
	// The window's own background: what the theme names for this desktop,
	// and what it draws its border with when it names neither.
	background := colors.Background
	if dark && colors.BackgroundDark != "" {
		background = colors.BackgroundDark
	}
	if !dark && colors.BackgroundLight != "" {
		background = colors.BackgroundLight
	}
	if background != "" {
		theme.Background = colorOf(background, theme.Background)
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

	// A theme that names no text colour is drawn on the desktop's own,
	// which is what the theme it was drawn from has; one that names a
	// background and no text would leave the text unreadable, so the text
	// follows the background it is drawn on.
	if dark {
		theme.Inverse = theme.Text
		theme.InverseText = theme.Background
	}
	if luminance(theme.Background) < 0.5 {
		// A dark background needs light text unless the theme said which.
		if colors.Title == "" {
			theme.Text = ui.RGB(255, 255, 255)
		}
		if colors.Subtitle == "" {
			theme.TextMuted = ui.RGB(190, 190, 190)
		}
		theme.Inverse = theme.Background
		theme.InverseText = theme.Text
	}
}

// luminance is how light a colour is, from 0 for black to 1 for white,
// with the weights a person's eye gives the three parts of it.
func luminance(c ui.Color) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
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
