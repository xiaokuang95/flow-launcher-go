package core

import (
	"context"
	"image"
	"path"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// MaxScore is the score of a result a user pinned to the top.
const MaxScore = int(^uint32(0) >> 1)

// SpecialKeyState holds the modifiers held when an action runs.
type SpecialKeyState struct {
	CtrlPressed  bool
	ShiftPressed bool
	AltPressed   bool
	WinPressed   bool
}

// DefaultSpecialKeyState is the state with no modifier held.
var DefaultSpecialKeyState = SpecialKeyState{}

// ActionContext is what an action is told about how it was asked for.
type ActionContext struct {
	SpecialKeyState SpecialKeyState
}

// Result is one row a plugin answered a query with.
type Result struct {
	// Title is the row's first line, and what the user sees first.
	Title string
	// SubTitle is the second line, and the text Ctrl+C copies when the
	// plugin gave no CopyText.
	SubTitle string
	// ActionKeywordAssigned is the keyword that triggered the result, empty
	// for a plugin that answers every query.
	ActionKeywordAssigned string
	// AutoCompleteText is what Tab leaves in the query box, in place of the
	// title. It must carry the keyword itself when it needs one.
	AutoCompleteText string
	// QuerySuggestionText is what Tab completes with when AutoCompleteText
	// is empty; the keyword is prepended for it.
	QuerySuggestionText string
	// TitleToolTip and SubTitleToolTip show on hover.
	TitleToolTip    string
	SubTitleToolTip string

	copyText string

	// IcoPath is the icon: a path relative to the plugin's directory, an
	// absolute path, or an http, https or data URL.
	IcoPath string
	// BadgeIcoPath is the badge's image; empty shows the plugin's icon.
	BadgeIcoPath string
	// ShowBadge draws the badge over the icon.
	ShowBadge bool
	// RoundedIcon rounds the icon's corners.
	RoundedIcon bool

	// Icon loads the icon itself, for one drawn rather than read from a
	// file, and takes precedence over IcoPath.
	Icon func() image.Image
	// BadgeIcon loads the badge itself.
	BadgeIcon func() image.Image
	// Glyph draws the icon as a character of a font, as the reference's
	// glyph icons do.
	Glyph *GlyphInfo

	// Action runs when the user picks the result. It reports whether the
	// launcher should hide: true hides it, false keeps it in front.
	Action func(ActionContext) bool
	// AsyncAction is Action for work that waits, and takes precedence.
	AsyncAction func(context.Context, ActionContext) bool

	// Score is the priority the plugin gives the result.
	Score int
	// TitleHighlightData holds the rune indexes of Title the query matched.
	TitleHighlightData []int
	// OriginQuery is the query that produced the result.
	OriginQuery *Query

	// PluginDirectory is the directory the plugin was loaded from, which
	// IcoPath is resolved against.
	PluginDirectory string
	// PluginID is the id of the plugin that produced the result.
	PluginID string

	// ContextData is anything the plugin wants back in its context menu.
	ContextData any

	// ProgressBar draws a progress bar over the result when set, from 0 to
	// 100, in ProgressBarColor.
	ProgressBar      *int
	ProgressBarColor string

	// Preview is what the preview panel shows for this result.
	Preview PreviewInfo
	// PreviewVisibility overrides the user's setting for the panel.
	PreviewVisibility PreviewVisibility
	// PreviewPanel builds a preview of the plugin's own, in place of the
	// one Preview describes.
	PreviewPanel func(c *ui.Context) ui.Element

	// AddSelectedCount lets the score grow with how often the user picked
	// the result. A plugin that keeps its own order sets it false.
	AddSelectedCount bool
	// RecordKey identifies the result for pinning and selection counts
	// when its title and subtitle change between queries.
	RecordKey string
}

// GlyphInfo is an icon drawn as a character of a font.
type GlyphInfo struct {
	FontFamily string
	Glyph      string
}

// PreviewInfo is what the preview panel shows.
type PreviewInfo struct {
	// PreviewImagePath is the image shown; empty falls back to the icon.
	PreviewImagePath string
	// IsMedia makes the image take the panel's full width.
	IsMedia bool
	// Description is the text shown under the image; empty falls back to
	// the subtitle.
	Description string
	// PreviewDelegate loads the image itself.
	PreviewDelegate func() image.Image
	// FilePath is the file the result is about, for an external preview.
	FilePath string
	// ContentType says how Description is rendered.
	ContentType PreviewContentType
}

// DefaultPreviewInfo is the preview of a result that set none.
func DefaultPreviewInfo() PreviewInfo {
	return PreviewInfo{ContentType: PreviewContentTypeImageWithText}
}

// PreviewContentType is how a preview's description is rendered.
type PreviewContentType int

const (
	// PreviewContentTypeImageWithText shows the image, then the
	// description as plain text.
	PreviewContentTypeImageWithText PreviewContentType = iota
	// PreviewContentTypeMarkdown shows a scrolling panel with the
	// description as Markdown.
	PreviewContentTypeMarkdown
)

// String is the name the protocol uses.
func (p PreviewContentType) String() string {
	if p == PreviewContentTypeMarkdown {
		return "markdown"
	}
	return "imageWithText"
}

// PreviewVisibility is whether the preview panel shows for a result.
type PreviewVisibility int

const (
	// PreviewOptional shows the panel only when the user asks for it.
	PreviewOptional PreviewVisibility = iota
	// PreviewNever hides it even when the user always previews.
	PreviewNever
	// PreviewAlways shows it even when the user never previews.
	PreviewAlways
)

// String is the name the protocol uses.
func (p PreviewVisibility) String() string {
	switch p {
	case PreviewNever:
		return "never"
	case PreviewAlways:
		return "always"
	default:
		return "optional"
	}
}

// NewResult makes a result with the title, subtitle and icon path.
func NewResult(title, subTitle, icoPath string, score int) Result {
	return Result{
		Title:            title,
		SubTitle:         subTitle,
		IcoPath:          icoPath,
		Score:            score,
		AddSelectedCount: true,
		Preview:          DefaultPreviewInfo(),
		ProgressBarColor: "#26a0da",
	}
}

// CopyText is what Ctrl+C copies: the plugin's own text, or the subtitle.
func (r Result) CopyText() string {
	if r.copyText != "" {
		return r.copyText
	}
	return r.SubTitle
}

// SetCopyText sets the text Ctrl+C copies.
func (r *Result) SetCopyText(text string) { r.copyText = text }

// IcoPathAbsolute is IcoPath resolved against the plugin's directory, which
// is what the view loads. A URL or an absolute path is left alone.
func (r Result) IcoPathAbsolute() string { return resolvePluginPath(r.PluginDirectory, r.IcoPath) }

// BadgeIcoPathAbsolute is BadgeIcoPath resolved the same way.
func (r Result) BadgeIcoPathAbsolute() string {
	return resolvePluginPath(r.PluginDirectory, r.BadgeIcoPath)
}

// resolvePluginPath joins a path to the plugin's directory unless it is
// already absolute or is a URL.
func resolvePluginPath(dir, value string) string {
	if value == "" || dir == "" {
		return value
	}
	if filepath.IsAbs(value) || hasURLScheme(value) {
		return value
	}
	return filepath.Join(dir, value)
}

// hasURLScheme reports whether the path is an http, https or data URL.
func hasURLScheme(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "data:image")
}

// Progress draws a progress bar over the result.
func Progress(value int) *int { return &value }

// Execute runs the result's action, preferring the asynchronous one, and
// reports whether the launcher should hide.
func (r Result) Execute(ctx context.Context, action ActionContext) bool {
	if r.AsyncAction != nil {
		return r.AsyncAction(ctx, action)
	}
	if r.Action != nil {
		return r.Action(action)
	}
	return false
}

// Clone copies a result, so that the launcher can change the score of a
// result a plugin still holds without changing the plugin's own value.
func (r Result) Clone() Result {
	clone := r
	if r.TitleHighlightData != nil {
		clone.TitleHighlightData = append([]int(nil), r.TitleHighlightData...)
	}
	if r.ProgressBar != nil {
		value := *r.ProgressBar
		clone.ProgressBar = &value
	}
	if r.Glyph != nil {
		glyph := *r.Glyph
		clone.Glyph = &glyph
	}
	return clone
}

// FileName returns the last element of a path, for a subtitle or a tooltip.
func FileName(p string) string { return path.Base(filepath.ToSlash(p)) }
