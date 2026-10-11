package main

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// runVariant runs the result the user is on with a modifier held, which
// plugins read to do something else, as the reference does.
func (a *app) runVariant(state core.SpecialKeyState) {
	a.runResult(a.selection, state)
}

// copySelected copies the text of the result, or the path of the file it is
// about when the user asks for that.
func (a *app) copySelected(path bool) {
	result, ok := a.selectedResult()
	if !ok {
		return
	}

	text := result.CopyText()
	if path && result.Preview.FilePath != "" {
		text = result.Preview.FilePath
	}
	if text == "" {
		return
	}
	a.copyToClipboard(text, false, true)
}

// copyToClipboard puts text on the clipboard, or the file at a path: a
// path that is there is copied as the file itself, so that pasting it into
// a file manager copies the file rather than its name.
func (a *app) copyToClipboard(text string, directCopy, notify bool) {
	if text == "" {
		return
	}

	var data transfer.Data
	if core.FileExists(text) || core.DirectoryExists(text) {
		absolute := text
		if resolved, err := filepath.Abs(text); err == nil {
			absolute = resolved
		}
		files, err := transfer.FileData(absolute)
		if err != nil {
			data = transfer.TextData(text)
		} else {
			data = files
		}
	} else {
		data = transfer.TextData(text)
	}

	if err := mygo.Clipboard.Write(data); err != nil {
		a.api.LogError("launcher", "writing the clipboard failed: "+err.Error())
		return
	}
	if notify {
		a.notify(core.T("", "copy"), text)
	}
}

// autocomplete leaves the completion of the result in the query box, as
// Ctrl+Tab does.
func (a *app) autocomplete(useSubTitle bool) {
	result, ok := a.selectedResult()
	if !ok {
		return
	}

	text := result.Title
	if result.AutoCompleteText != "" {
		text = result.AutoCompleteText
	} else if result.QuerySuggestionText != "" {
		text = result.QuerySuggestionText
		if keyword := result.ActionKeywordAssigned; keyword != "" {
			text = keyword + " " + text
		}
	}
	if useSubTitle {
		text = result.SubTitle
	}
	if text == "" {
		return
	}

	a.query = text
	a.searched = ""
	a.search(text)
}

// cycleHistory walks the queries the user ran before, as Alt+Up and Alt+Down
// do.
func (a *app) cycleHistory(delta int) {
	items := a.history.LastOpenedHistoryItems
	if len(items) == 0 {
		return
	}

	if a.historyIndex == -1 {
		a.historyDraft = a.query
		if delta < 0 {
			a.historyIndex = len(items)
		} else {
			a.historyIndex = -1
		}
	}

	a.historyIndex += delta
	if a.historyIndex < 0 {
		a.historyIndex = -1
		a.query = a.historyDraft
		a.searched = ""
		a.search(a.query)
		return
	}
	if a.historyIndex >= len(items) {
		a.historyIndex = len(items) - 1
	}

	a.query = items[a.historyIndex].Query
	a.searched = ""
	a.search(a.query)
}

// toggleHistory shows what the user opened before, as Ctrl+H does.
func (a *app) toggleHistory() {
	a.query = ""
	a.searched = ""
	a.settings.ShowHistoryResultsForHomePage = true
	a.search("")
}

// escape clears the query box, closes the menu, or hides the launcher.
func (a *app) escape() {
	switch {
	case len(a.contextMenu) > 0:
		a.closeContextMenu()
	case a.query != "":
		a.query = ""
		a.searched = ""
		a.search("")
	default:
		a.hide()
	}
}

// stepResultsCount shows more or fewer results, as Ctrl+= and Ctrl+- do.
func (a *app) stepResultsCount(more bool) {
	if more {
		if a.settings.MaxResultsToShow < core.MaxResultsToShowLimit {
			a.settings.MaxResultsToShow++
		}
	} else if a.settings.MaxResultsToShow > core.MinResultsToShow {
		a.settings.MaxResultsToShow--
	}
	a.saveSettings()
	a.requery()
}

// stepWindowWidth widens or narrows the window, as Ctrl+] and Ctrl+[ do.
func (a *app) stepWindowWidth(wider bool) {
	if wider {
		a.settings.WindowSize += 40
	} else if a.settings.WindowSize > 320 {
		a.settings.WindowSize -= 40
	}
	a.lastWidth, a.lastHeight = 0, 0
	a.saveSettings()
}

// openContextMenu shows the menu of the result the user is on.
func (a *app) openContextMenu() {
	result, ok := a.selectedResult()
	if !ok {
		return
	}

	items := []core.Result{}
	if a.topMost.IsTopMost(result) {
		items = append(items, a.pinnedItem(result, false))
	} else {
		items = append(items, a.pinnedItem(result, true))
	}
	items = append(items, a.pluginSettingsItem(result))
	items = append(items, a.pluginInfoItem(result))

	items = append(items, a.manager.ContextMenusForPlugin(result)...)

	a.contextMenu = items
	a.contextIndex = 0
	a.input.Focus()
	a.update(func() {})
}

// pinnedItem is the menu item that pins a result to the top of its query,
// or unpins it.
func (a *app) pinnedItem(result core.Result, pin bool) core.Result {
	item := core.NewResult(core.T("", "setAsTopMostInThisQuery"), "", "", 0)
	if !pin {
		item.Title = core.T("", "cancelTopMostInThisQuery")
	}
	item.Action = func(core.ActionContext) bool {
		if pin {
			a.topMost.AddOrUpdate(result)
		} else {
			a.topMost.Remove(result)
		}
		_ = a.topMost.Save()
		a.requery()
		return false
	}
	return item
}

// pluginSettingsItem opens the settings page of the plugin that answered.
func (a *app) pluginSettingsItem(result core.Result) core.Result {
	name := result.PluginID
	if pair, ok := a.manager.GetPluginForID(result.PluginID); ok {
		name = pair.Metadata.Name
	}
	item := core.NewResult(core.Tf("", "pluginSettingsWindowTitle", name), "", "", 0)
	item.Action = func(core.ActionContext) bool {
		a.api.OpenPluginSettingsWindow(result.PluginID)
		return false
	}
	return item
}

// pluginInfoItem opens the plugin's page in the plugin store.
func (a *app) pluginInfoItem(result core.Result) core.Result {
	item := core.NewResult(core.T("", "pluginInfo"), "", "", 0)
	item.Action = func(core.ActionContext) bool {
		openURL("https://www.flowlauncher.com/plugins/"+result.PluginID, nil)
		return true
	}
	return item
}

// closeContextMenu puts the results back.
func (a *app) closeContextMenu() {
	if len(a.contextMenu) == 0 {
		return
	}
	a.contextMenu = nil
	a.update(func() {})
}

// resultContextMenu builds the menu a result shows on a right click.
func (a *app) resultContextMenu(menu *ui.Menu, result core.Result) {
	items := a.manager.ContextMenusForPlugin(result)
	if len(items) == 0 {
		menu.Item(core.T("", "searchplugin_Noresult_Title")).Disabled(true)
		return
	}

	for i := range items {
		item := items[i]
		if menu.Item(item.Title).Chosen() {
			a.runAction(item, core.DefaultSpecialKeyState)
		}
	}
}

// openDirectory shows a directory, with a file inside it selected.
func openDirectory(directoryPath, fileNameOrFilePath string) {
	target := directoryPath
	if fileNameOrFilePath != "" {
		target = filepath.Join(directoryPath, fileNameOrFilePath)
	}
	openWithShell(target)
}

// openURL opens a URL in the user's browser.
func openURL(rawURL string, inPrivate *bool) {
	if rawURL == "" {
		return
	}
	if !strings.Contains(rawURL, "://") && !strings.HasPrefix(rawURL, "mailto:") {
		rawURL = "http://" + rawURL
	}
	openWithShell(rawURL)
}

// shellRun runs a command line, as the shell plugin does.
func shellRun(cmd, filename string) {
	runShell(cmd, filename)
}

// notify shows a notification.
func (a *app) notify(title, subTitle string) {
	notification := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: subTitle})
	_ = notification.Show()
}

// showMessageBox asks the user a question.
func (a *app) showMessageBox(text, caption string, buttons core.MessageBoxButtons, icon core.MessageBoxIcon) core.MessageBoxResult {
	kind := mygo.MessageInfo
	switch icon {
	case core.MessageBoxIconWarning:
		kind = mygo.MessageWarning
	case core.MessageBoxIconError:
		kind = mygo.MessageError
	case core.MessageBoxIconQuestion:
		kind = mygo.MessageQuestion
	}

	labels := []string{"OK"}
	switch buttons {
	case core.MessageBoxOKCancel:
		labels = []string{"OK", "Cancel"}
	case core.MessageBoxYesNo:
		labels = []string{"Yes", "No"}
	}

	answer, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:  a.win,
		Title:   caption,
		Message: text,
		Type:    kind,
		Buttons: labels,
	})
	if err != nil {
		return core.MessageBoxResultOK
	}

	switch answer.Button {
	case 1:
		return core.MessageBoxResultCancel
	default:
		return core.MessageBoxResultOK
	}
}

// gameMode reports whether the launcher is holding itself back.
func (a *app) gameMode() bool { return a.settings.GameMode }

// setGameMode holds the launcher back while the user plays: it stops
// answering the hotkey, so that a game keeps the screen to itself.
func (a *app) setGameMode(value bool) {
	if a.settings.GameMode == value {
		return
	}
	a.settings.GameMode = value
	a.saveSettings()
	a.updateTray()
	if a.win != nil {
		if value {
			a.win.SetAlwaysOnTop(false)
		} else {
			a.win.SetAlwaysOnTop(a.settings.ShowAtTopmost)
		}
	}
}

// isDarkTheme reports whether the launcher is drawn dark, which plugins ask
// about.
func isDarkTheme() bool { return mygo.Theme.IsDark() }
