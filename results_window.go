package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// A plugin's results window is a window of its own, showing what the plugin
// holds: the clipboard history at one keystroke, say. The launcher's own
// window keeps its search box and its results; a plugin that offers
// something at a keystroke shows it here instead of taking the search box
// over.

// resultsWindowHeight and resultsWindowWidth are how big a plugin's window
// is, which is enough for a list of results and a query box.
const (
	resultsWindowWidth  = 460
	resultsWindowHeight = 420
)

// resultsWindow is one plugin's window, with the list it shows.
type resultsWindow struct {
	app *app
	win *mygo.Window

	window core.ResultsWindow

	// query is what the user typed, and results what it found.
	query   string
	results []core.Result
	list    ui.ListState
	input   ui.Handle
	// selection is the result the user is on.
	selection int
}

// resultsWindows are the windows that are open, one per plugin.
var resultsWindows = map[string]*resultsWindow{}

// showResultsWindow shows a plugin's results in a window of its own, or
// brings the one it has forward.
func (a *app) showResultsWindow(window core.ResultsWindow) {
	if window.PluginID == "" || window.Search == nil {
		return
	}

	existing := resultsWindows[window.PluginID]
	if existing != nil {
		existing.window = window
		existing.search("")
		existing.win.Show()
		existing.win.Focus()
		existing.focusQuery()
		return
	}

	a.rememberForeground()
	shown := &resultsWindow{app: a, window: window}
	shown.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     window.Title,
		Width:     resultsWindowWidth,
		Height:    resultsWindowHeight,
		MinWidth:  320,
		MinHeight: 240,
		StateKey:  "plugin-results-" + window.PluginID,
		Content:   ui.View(shown.view),
	})
	shown.win.OnBlur(func() {
		if a.settings.HideWhenDeactivated {
			shown.close()
		}
	})
	resultsWindows[window.PluginID] = shown
	shown.search("")
	shown.win.Show()
	shown.win.Focus()
	shown.focusQuery()
}

// closeResultsWindow closes the window of a plugin's results.
func (a *app) closeResultsWindow(pluginID string) {
	if window := resultsWindows[pluginID]; window != nil {
		window.close()
	}
}

// closeResultsWindows closes every plugin's window, which the launcher does
// when it hides itself.
func (a *app) closeResultsWindows() {
	for _, window := range resultsWindows {
		window.close()
	}
}

// close takes the window down.
func (w *resultsWindow) close() {
	if w.win != nil {
		w.win.Hide()
	}
}

// focusQuery puts the caret in the query box.
func (w *resultsWindow) focusQuery() { w.input.Focus() }

// search answers what was typed with the plugin's results.
func (w *resultsWindow) search(query string) {
	w.query = query
	w.results = w.window.Search(query)
	w.selection = 0
}

// view builds the window: the query box, then the results.
func (w *resultsWindow) view(c *ui.Context) {
	w.app.applyTheme(c)
	t := c.Theme()

	ui.Column(c).Fill().Clip().Background(t.Background).Children(func() {
		ui.Row(c).Height(queryBoxHeight).Padding(12, 10).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, searchIcon()).FontSize(18).TextColor(t.TextMuted)
			input := ui.TextInputBase(c, &w.query).
				Bind(&w.input).
				Placeholder(w.window.Placeholder).
				Label(w.window.Title).
				FontSize(float32(w.app.queryFontSize())).
				Grow(1)
			input.HandleInput(w.keyInput)

			if input.Changed() {
				w.search(w.query)
			}
			if input.Submitted() {
				w.choose(w.selection)
			}
		})

		w.list.Selected = &w.selection
		w.list.Label = func(row int) string {
			if row < len(w.results) {
				return w.results[row].Title
			}
			return ""
		}

		if len(w.results) == 0 {
			ui.Text(c, core.T("", "searchplugin_Noresult_Title")).
				TextColor(t.TextMuted).Padding(12, 16)
			return
		}

		// What the chosen result holds is shown beside the list, when the
		// plugin gave something to look at.
		if w.showsPreview() {
			width := float32(resultsWindowWidth) - previewPaneWidth
			ui.Split(c, &width, func() { w.resultsView(c) }, func() { w.previewView(c) }).Grow(1)
			return
		}
		w.resultsView(c)
	})
}

// resultsView is the list of the plugin's results.
func (w *resultsWindow) resultsView(c *ui.Context) {
	t := c.Theme()

	ui.List(c, &w.list, len(w.results)).Grow(1).Rows(func(row ui.ListRow) {
		if row.Index < 0 || row.Index >= len(w.results) {
			return
		}
		result := w.results[row.Index]

		element := ui.Row(row.Context).Height(resultHeight).Padding(0, 12).Gap(12).AlignItems(ui.Center)
		if row.Index == w.selection {
			element.Background(t.Selection)
		}
		element.Children(func() {
			w.app.resultIcon(row.Context, result)
			ui.Column(row.Context).Grow(1).Gap(2).Children(func() {
				ui.Text(row.Context, result.Title).
					FontSize(float32(w.app.resultFontSize())).
					SingleLine()
				if result.SubTitle != "" {
					ui.Text(row.Context, result.SubTitle).
						FontSize(float32(w.app.resultSubFontSize())).
						SingleLine().
						TextColor(t.TextMuted)
				}
			})
		})
		element.OnClick(func() {
			w.selection = row.Index
			w.choose(row.Index)
		})
	})
}

// previewPaneWidth is how much of the window a picture takes.
const previewPaneWidth float32 = 160

// showsPreview reports whether the result the user is on has something to
// look at.
func (w *resultsWindow) showsPreview() bool {
	if w.selection < 0 || w.selection >= len(w.results) {
		return false
	}
	preview := w.results[w.selection].Preview
	return preview.PreviewImagePath != "" || preview.FilePath != ""
}

// previewView shows what the result the user is on holds, when the plugin
// gave something to look at: a picture copied to the clipboard is shown as
// itself rather than as the word "Image".
func (w *resultsWindow) previewView(c *ui.Context) {
	if w.selection < 0 || w.selection >= len(w.results) {
		return
	}
	preview := w.results[w.selection].Preview
	if preview.PreviewImagePath == "" && preview.FilePath == "" {
		return
	}

	path := preview.PreviewImagePath
	if path == "" {
		path = preview.FilePath
	}
	bitmap := w.app.bitmapFor(path)
	if bitmap == nil {
		return
	}

	ui.Column(c).Width(previewPaneWidth).Padding(12).Gap(8).Background(c.Theme().Surface).Children(func() {
		ui.Image(c, bitmap).FillWidth().Fit(ui.Contain)
	})
}

// keyInput handles the keys the window answers to, and lets everything else
// through so that typing works.
func (w *resultsWindow) keyInput(event ui.InputEvent) bool {
	if event.Kind != ui.InputKeyDown {
		return false
	}

	switch {
	case event.Mods == 0 && event.Key == ui.KeyDown:
		w.moveSelection(1)
	case event.Mods == 0 && event.Key == ui.KeyUp:
		w.moveSelection(-1)
	case event.Mods == 0 && event.Key == ui.KeyPageDown:
		w.moveSelection(5)
	case event.Mods == 0 && event.Key == ui.KeyPageUp:
		w.moveSelection(-5)
	case event.Mods == 0 && event.Key == ui.KeyDelete && w.window.Delete != nil:
		// Delete takes the result away, which a clipboard history needs:
		// what was copied by mistake is in the way of what was meant.
		w.deleteSelection()
	case event.Mods == ui.Cmd && event.Key == ui.KeyDelete && w.window.Delete != nil:
		// Ctrl+Delete empties the list, which the plugin decides on.
		w.deleteAll()
	case event.Mods == 0 && event.Key == ui.KeyEscape:
		w.close()
	case event.Mods == ui.Cmd && event.Key == ui.KeyW:
		w.close()
	default:
		// Everything else is the query box's, so that typing narrows the
		// list.
		return false
	}
	return true
}

// deleteSelection takes the result the user is on away, and shows what is
// left.
func (w *resultsWindow) deleteSelection() {
	if w.selection < 0 || w.selection >= len(w.results) {
		return
	}
	if w.window.Delete == nil || !w.window.Delete(w.results[w.selection]) {
		return
	}

	// The list is asked again, and the choice stays where it was, which is
	// the next result.
	was := w.selection
	w.search(w.query)
	w.selection = was
	if w.selection >= len(w.results) && w.selection > 0 {
		w.selection = len(w.results) - 1
	}
	w.list.ScrollIntoView(w.selection)
}

// deleteAll takes every result away, which the plugin decides what that
// means.
func (w *resultsWindow) deleteAll() {
	if w.window.Delete == nil {
		return
	}
	for _, result := range w.results {
		if !w.window.Delete(result) {
			return
		}
	}
	w.search(w.query)
}

// moveSelection walks the results, keeping to the ends.
func (w *resultsWindow) moveSelection(delta int) {
	if len(w.results) == 0 {
		return
	}
	next := w.selection + delta
	if next < 0 {
		next = 0
	}
	if next >= len(w.results) {
		next = len(w.results) - 1
	}
	w.selection = next
	w.list.ScrollIntoView(next)
}

// choose runs the result the user chose, and closes the window unless the
// plugin asks to keep it.
func (w *resultsWindow) choose(index int) {
	if index < 0 || index >= len(w.results) {
		return
	}

	result := w.results[index]
	keep := false
	if w.window.Choose != nil {
		keep = w.window.Choose(result)
	}
	if !keep {
		w.close()
	}
}
