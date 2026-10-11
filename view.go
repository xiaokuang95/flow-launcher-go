package main

import (
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// The window's own metrics, which the reference keeps in its settings: the
// height of the query box, and of one result.
const (
	queryBoxHeight = 42
	resultHeight   = 58
	previewWidth   = 320
)

// previewOn reports whether the preview panel shows for the result the user
// is on.
func (a *app) previewOn() bool {
	result, ok := a.selectedResult()
	if !ok {
		return false
	}
	switch result.PreviewVisibility {
	case core.PreviewNever:
		return false
	case core.PreviewAlways:
		return true
	}
	return a.preview || a.settings.AlwaysPreview
}

// queryBox is the row the user types in, with the icon of the plugin the
// query goes to.
func (a *app) queryBox(c *ui.Context) {
	t := c.Theme()

	row := ui.Row(c).Height(float32(a.settings.WindowHeightSize)).Padding(0, 12).Gap(10).AlignItems(ui.Center)
	row.Children(func() {
		if a.pluginIcon != "" {
			if bitmap := a.bitmapFor(a.pluginIcon); bitmap != nil {
				ui.Image(c, bitmap).Size(24, 24)
			}
		} else {
			ui.Icon(c, searchIcon()).FontSize(18).TextColor(t.TextMuted)
		}

		// The query box is the window's own text field rather than a
		// control of the desktop's: no border, no background, and the
		// launcher's keys instead of a field's.
		input := ui.TextInputBase(c, &a.query).
			Bind(&a.input).
			Placeholder(a.placeholder()).
			Label(core.T("", "queryTextBoxPlaceholder")).
			FontSize(float32(a.queryFontSize())).
			Grow(1)

		// The query box takes every key, so that the launcher's own
		// bindings work while it has the focus, and passes on the ones it
		// does not use so that typing still edits the text.
		input.HandleInput(a.keyInput)

		if input.Submitted() {
			switch {
			case len(a.contextMenu) > 0:
				// Enter chooses the item of the menu that is open.
				a.runContextItem(a.contextIndex)
			case c.Modifiers()&ui.Shift != 0:
				a.openContextMenu()
			default:
				a.runResult(a.selection, core.DefaultSpecialKeyState)
			}
		}
		if input.Changed() {
			a.search(a.query)
		}
	})
}

// placeholder is what the empty query box says.
func (a *app) placeholder() string {
	if a.settings.PlaceholderText != "" {
		return a.settings.PlaceholderText
	}
	return core.T("", "queryTextBoxPlaceholder")
}

// queryFontSize is the size the query box is drawn at.
func (a *app) queryFontSize() int {
	if a.settings.QueryFontSize > 0 {
		return a.settings.QueryFontSize
	}
	return 24
}

// resultFontSize is the size a result's title is drawn at.
func (a *app) resultFontSize() int {
	if a.settings.ResultFontSize > 0 {
		return a.settings.ResultFontSize
	}
	return 16
}

// keyInput handles the keys the launcher binds while the query box has the
// focus, and lets everything else through so that typing works.
func (a *app) keyInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown {
		return false
	}
	return a.key(ev.Mods, ev.Key)
}

// key handles one key press, and reports whether the launcher took it.
func (a *app) key(mods ui.Modifiers, key ui.Key) bool {
	// While the menu of a result is open, the keys move through it.
	if len(a.contextMenu) > 0 {
		switch {
		case mods == 0 && key == ui.KeyDown:
			a.moveMenuSelection(1)
		case mods == 0 && key == ui.KeyUp:
			a.moveMenuSelection(-1)
		case mods == 0 && key == ui.KeyEscape:
			a.closeContextMenu()
		case mods == 0 && (key == ui.KeyLeft || key == ui.KeyRight):
			a.closeContextMenu()
		case mods == 0 && (key == ui.KeyPageDown || key == ui.KeyPageUp):
			step := a.pageSize()
			if key == ui.KeyPageUp {
				step = -step
			}
			a.moveMenuSelection(step)
		case mods == ui.Cmd && key == ui.KeyO:
			a.closeContextMenu()
		default:
			// Everything else is left to the query box, so that typing
			// carries on narrowing what is underneath.
			return false
		}
		return true
	}

	switch {
	// Choosing and running results.
	case mods == 0 && key == ui.KeyDown:
		a.moveSelection(1)
	case mods == 0 && key == ui.KeyUp:
		a.moveSelection(-1)
	case mods == 0 && key == ui.KeyTab:
		a.moveSelection(1)
	case mods == ui.Shift && key == ui.KeyTab:
		a.moveSelection(-1)
	case mods == 0 && key == ui.KeyPageDown:
		a.moveSelection(a.pageSize())
	case mods == 0 && key == ui.KeyPageUp:
		a.moveSelection(-a.pageSize())
	case mods == 0 && key == ui.KeyHome, mods == ui.Alt && key == ui.KeyHome:
		a.selectIndex(0)
	case mods == 0 && key == ui.KeyEnd, mods == ui.Alt && key == ui.KeyEnd:
		a.selectIndex(len(a.results) - 1)

	// Running a result with a modifier held.
	case mods == ui.Cmd && key == ui.KeyEnter:
		a.runVariant(core.SpecialKeyState{CtrlPressed: true})
	case mods == ui.Shift && key == ui.KeyEnter, mods == ui.Cmd && key == ui.KeyO:
		a.openContextMenu()
	case mods == ui.Alt && key == ui.KeyEnter:
		a.runVariant(core.SpecialKeyState{AltPressed: true})
	case mods == ui.Cmd|ui.Shift && key == ui.KeyEnter:
		a.runVariant(core.SpecialKeyState{CtrlPressed: true, ShiftPressed: true})
	case mods == ui.Cmd && key >= ui.Key1 && key <= ui.Key9:
		a.runResult(int(key-ui.Key1), core.DefaultSpecialKeyState)
	case mods == ui.Cmd && key == ui.Key0:
		a.runResult(9, core.DefaultSpecialKeyState)

	// The query box and the results.
	case mods == ui.Cmd && key == ui.KeyC:
		a.copySelected(false)
	case mods == ui.Cmd|ui.Shift && key == ui.KeyC:
		a.copySelected(true)
	case mods == ui.Cmd && key == ui.KeyTab:
		a.autocomplete(false)
	case mods == ui.Cmd|ui.Shift && key == ui.KeyTab:
		a.autocomplete(true)
	case mods == ui.Cmd && key == ui.KeyR:
		a.requery()
	case mods == 0 && key == ui.KeyF5:
		a.reloadPluginData()
	case mods == ui.Alt && key == ui.KeyUp:
		a.cycleHistory(-1)
	case mods == ui.Alt && key == ui.KeyDown:
		a.cycleHistory(1)
	case mods == ui.Cmd && key == ui.KeyH:
		a.toggleHistory()

	// The window.
	case mods == 0 && key == ui.KeyEscape:
		a.escape()
	case mods == 0 && key == ui.KeyF1:
		a.preview = !a.preview
	case mods == ui.Cmd && key == ui.KeyI:
		openSettingsWindow(a)
	case mods == ui.Cmd && key == ui.KeyF12:
		a.setGameMode(!a.gameMode())
	case mods == ui.Cmd && (key == ui.KeyEqual || key == ui.KeyMinus):
		a.stepResultsCount(key == ui.KeyEqual)
	case mods == ui.Cmd && (key == ui.KeyBracketLeft || key == ui.KeyBracketRight):
		a.stepWindowWidth(key == ui.KeyBracketRight)

	default:
		return false
	}
	return true
}

// moveMenuSelection walks the items of the menu of a result.
func (a *app) moveMenuSelection(delta int) {
	if len(a.contextMenu) == 0 {
		return
	}
	a.contextIndex = (a.contextIndex + delta + len(a.contextMenu)) % len(a.contextMenu)
}

// moveSelection walks the results, wrapping around as the reference does.
func (a *app) moveSelection(delta int) {
	if len(a.results) == 0 {
		return
	}
	a.selection = (a.selection + delta + len(a.results)) % len(a.results)
}

// selectIndex chooses a result by its place.
func (a *app) selectIndex(index int) {
	if index < 0 || index >= len(a.results) {
		return
	}
	a.selection = index
}

// pageSize is how many results a page moves by.
func (a *app) pageSize() int { return a.settings.MaxResultsToShow }

// contextMenuView is the menu of the result the user asked for, shown in
// place of the results.
func (a *app) contextMenuView(c *ui.Context) {
	t := c.Theme()

	// The menu keeps its own list, so that moving through it does not move
	// the choice among the results.
	a.menuList.Selected = &a.contextIndex
	a.menuList.Key = func(row int) any { return row }
	a.menuList.Label = func(row int) string {
		if row < len(a.contextMenu) {
			return a.contextMenu[row].Title
		}
		return ""
	}

	height := float32(a.settings.MaxResultsToShow) * resultHeight
	ui.List(c, &a.menuList, len(a.contextMenu)).MaxHeight(height).Grow(1).Rows(func(row ui.ListRow) {
		item := a.contextMenu[row.Index]
		selected := row.Selected() && row.ListFocused()

		element := ui.Row(row.Context).Height(resultHeight).Padding(0, 12).Gap(12).AlignItems(ui.Center)
		if selected {
			element.Background(t.Selection)
		}
		element.Children(func() {
			a.resultIcon(row.Context, item)
			ui.Text(row.Context, item.Title).FontSize(float32(a.resultFontSize())).SingleLine().Grow(1)
		})
		element.OnClick(func() {
			a.contextIndex = row.Index
			a.runContextItem(row.Index)
		})
	})
}

// runContextItem does what an item of the menu says.
func (a *app) runContextItem(index int) {
	if index < 0 || index >= len(a.contextMenu) {
		return
	}
	item := a.contextMenu[index]
	a.closeContextMenu()
	a.runAction(item, core.DefaultSpecialKeyState)
}

// resultsView is the list of results.
func (a *app) resultsView(c *ui.Context) {
	t := c.Theme()

	a.list.Selected = &a.selection
	a.list.Key = func(row int) any { return row }
	a.list.Label = func(row int) string {
		if row < len(a.results) {
			return a.results[row].Title
		}
		return ""
	}

	height := float32(a.settings.MaxResultsToShow) * resultHeight
	list := ui.List(c, &a.list, len(a.results)).MaxHeight(height).Grow(1)

	if len(a.results) == 0 {
		list.Children(func() {
			ui.Text(c, core.T("", "searchplugin_Noresult_Title")).TextColor(t.TextMuted).Padding(12, 16)
		})
		return
	}

	list.Rows(func(row ui.ListRow) {
		a.resultRow(row)
	})
}

// resultRow draws one result: its icon, its title with the matched
// characters highlighted, and its subtitle.
func (a *app) resultRow(row ui.ListRow) {
	c := row.Context
	t := c.Theme()
	result := a.results[row.Index]

	selected := row.Selected() && row.ListFocused()
	element := ui.Row(c).Height(resultHeight).Padding(0, 12).Gap(12).AlignItems(ui.Center)
	if selected {
		element.Background(t.Selection)
	}

	element.Children(func() {
		a.resultIcon(c, result)

		ui.Column(c).Grow(1).Gap(2).Children(func() {
			a.resultTitle(c, result)
			if result.SubTitle != "" {
				subtitle := ui.Text(c, result.SubTitle).
					FontSize(12).
					SingleLine().
					TextColor(t.TextMuted)
				if result.SubTitleToolTip != "" {
					subtitle.Tooltip(result.SubTitleToolTip)
				}
			}
			if result.ProgressBar != nil {
				ui.Progress(c, float64(*result.ProgressBar)/100).Height(3)
			}
		})

		if result.ShowBadge && result.BadgeIcoPath != "" {
			if bitmap := a.bitmapFor(result.BadgeIcoPath); bitmap != nil {
				ui.Image(c, bitmap).Size(16, 16)
			}
		}
	})

	element.OnClick(func() {
		a.selection = row.Index
		a.runResult(row.Index, core.DefaultSpecialKeyState)
	})
	element.ContextMenu(func(m *ui.Menu) { a.resultContextMenu(m, result) })
}

// resultIcon draws the icon of a result, which a plugin may give as a path,
// as a picture it drew, or as a character of a font.
func (a *app) resultIcon(c *ui.Context, result core.Result) {
	if result.Icon != nil {
		if bitmap := a.imageBitmap(result.Icon()); bitmap != nil {
			ui.Image(c, bitmap).Size(24, 24)
			return
		}
	}
	if result.Glyph != nil {
		ui.Text(c, result.Glyph.Glyph).Font(result.Glyph.FontFamily).FontSize(20).Width(24).TextAlign(ui.Center)
		return
	}
	if bitmap := a.bitmapFor(result.IcoPathAbsolute()); bitmap != nil {
		image := ui.Image(c, bitmap).Size(24, 24)
		if result.RoundedIcon {
			image.Radius(4)
		}
		return
	}
	// A result whose icon is missing keeps its place, so that the titles of
	// the rows line up.
	ui.Box(c).Size(24, 24)
}

// resultTitle draws the title with the characters the query matched in the
// accent color.
func (a *app) resultTitle(c *ui.Context, result core.Result) {
	t := c.Theme()
	style := func(element ui.Element) ui.Element {
		return element.FontSize(float32(a.resultFontSize())).SingleLine()
	}

	if len(result.TitleHighlightData) == 0 {
		title := style(ui.Text(c, result.Title))
		if result.TitleToolTip != "" {
			title.Tooltip(result.TitleToolTip)
		}
		return
	}

	title := style(ui.RichText(c, highlightSpans(result.Title, result.TitleHighlightData, t.Accent)...))
	if result.TitleToolTip != "" {
		title.Tooltip(result.TitleToolTip)
	}
}

// highlightSpans cuts a title into the runs that matched and the runs that
// did not.
func highlightSpans(title string, indexes []int, accent ui.Color) []ui.Span {
	runes := []rune(title)
	highlighted := make(map[int]bool, len(indexes))
	for _, index := range indexes {
		highlighted[index] = true
	}

	var spans []ui.Span
	var current []rune
	on := false
	flush := func() {
		if len(current) == 0 {
			return
		}
		span := ui.Span{Text: string(current)}
		if on {
			span.Color = accent
			span.Weight = 700
		}
		spans = append(spans, span)
		current = nil
	}

	for i, r := range runes {
		matched := highlighted[i]
		if i > 0 && matched != on {
			flush()
		}
		on = matched
		current = append(current, r)
	}
	flush()
	return spans
}

// previewView is the panel beside the results.
func (a *app) previewView(c *ui.Context) {
	t := c.Theme()
	result, ok := a.selectedResult()
	if !ok {
		return
	}

	panel := ui.Column(c).Fill().Padding(12).Gap(8).Background(t.Surface)

	preview := result.Preview
	if result.PreviewPanel != nil {
		panel.Children(func() { result.PreviewPanel(c) })
		return
	}

	panel.Children(func() {
		if preview.PreviewImagePath != "" || preview.PreviewDelegate != nil {
			var bitmap *ui.Bitmap
			if preview.PreviewDelegate != nil {
				bitmap = a.imageBitmap(preview.PreviewDelegate())
			} else {
				bitmap = a.bitmapFor(preview.PreviewImagePath)
			}
			if bitmap != nil {
				image := ui.Image(c, bitmap)
				if preview.IsMedia {
					image.FillWidth().Fit(ui.Contain)
				} else {
					image.Size(96, 96)
				}
			}
		}

		description := preview.Description
		if description == "" {
			description = result.SubTitle
		}
		if description != "" {
			ui.Text(c, description).Selectable().FontSize(12).TextColor(t.TextMuted)
		}
	})
}

// resizeWindow makes the window as tall as the results need.
func (a *app) resizeWindow(c *ui.Context) {
	if a.win == nil {
		return
	}

	width := int(a.settings.WindowSize)
	if width < 320 {
		width = 580
	}

	height := int(a.settings.WindowHeightSize)
	if len(a.results) > 0 {
		shown := len(a.results)
		if shown > a.settings.MaxResultsToShow {
			shown = a.settings.MaxResultsToShow
		}
		height += shown * resultHeight
	}

	if width == a.lastWidth && height == a.lastHeight {
		return
	}
	a.lastWidth, a.lastHeight = width, height
	a.win.SetSize(width, height)
}
