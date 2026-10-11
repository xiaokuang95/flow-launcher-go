package explorer

import (
	"os"
	"path/filepath"
	"strings"

	"launch/core"
)

// The icons the explorer gives its results, as the reference names them.
const (
	folderIcon      = "Images/folder.png"
	fileIcon        = "Images/file.png"
	quickAccessIcon = "Images/quickaccess.png"
	indexIcon       = "Images/index.png"
	explorerIcon    = "Images/explorer.png"
)

// resultFor turns what a search found into a result.
func (p *Plugin) resultFor(query core.Query, found SearchResult) core.Result {
	if found.Type == ResultTypeFile {
		return p.fileResult(found, query)
	}
	return p.folderResult(found, query)
}

// fileResult is a file as a result: its name, where it is, and what
// opening it does.
func (p *Plugin) fileResult(found SearchResult, query core.Query) core.Result {
	name := filepath.Base(found.FullPath)

	title := name
	if p.settings.DisplayMoreInformationInToolTip {
		title = name
	}

	result := core.NewResult(title, filepath.Dir(found.FullPath), found.FullPath, found.Score)
	result.SetCopyText(found.FullPath)
	result.TitleHighlightData = found.HighlightData
	if len(result.TitleHighlightData) == 0 {
		result.TitleHighlightData = p.api.Matcher().FuzzyMatch(query.Search, name).MatchData
	}
	result.AutoCompleteText = pathWithKeyword(found.FullPath, query.ActionKeyword)
	result.ContextData = found

	if found.Indexed {
		result.IcoPath = found.FullPath
	} else {
		result.IcoPath = found.FullPath
	}

	if p.settings.DisplayMoreInformationInToolTip {
		result.SubTitleToolTip = fileInformation(found.FullPath)
	}
	result.Preview.FilePath = found.FullPath
	result.Preview.Description = p.filePreviewText(found.FullPath)

	result.Action = func(action core.ActionContext) bool {
		switch {
		case action.SpecialKeyState.AltPressed:
			p.openWithShell(found.FullPath, false)
			return false
		case action.SpecialKeyState.CtrlPressed && action.SpecialKeyState.ShiftPressed:
			p.openInEditor(found.FullPath)
			return true
		case action.SpecialKeyState.CtrlPressed:
			p.api.OpenDirectory(filepath.Dir(found.FullPath), found.FullPath)
			return true
		default:
			openFile(found.FullPath)
			return true
		}
	}
	return result
}

// folderResult is a folder as a result: its name, where it is, and what
// opening it does.
func (p *Plugin) folderResult(found SearchResult, query core.Query) core.Result {
	name := filepath.Base(found.FullPath)

	score := found.Score
	if p.settings.BoostHomeFolderScore && isHomeFolder(found.FullPath) {
		score += 100
	}

	result := core.NewResult(name, found.FullPath, folderIcon, score)
	result.SetCopyText(found.FullPath)
	result.TitleHighlightData = found.HighlightData
	if len(result.TitleHighlightData) == 0 {
		result.TitleHighlightData = p.api.Matcher().FuzzyMatch(query.Search, name).MatchData
	}
	result.AutoCompleteText = pathWithKeyword(found.FullPath, query.ActionKeyword)
	result.ContextData = found
	result.Preview.FilePath = found.FullPath

	result.Action = func(action core.ActionContext) bool {
		switch {
		case action.SpecialKeyState.AltPressed:
			p.openWithShell(found.FullPath, true)
			return false
		case action.SpecialKeyState.CtrlPressed && action.SpecialKeyState.ShiftPressed:
			p.api.OpenDirectory(found.FullPath, "")
			return true
		case action.SpecialKeyState.CtrlPressed:
			p.api.OpenDirectory(filepath.Dir(found.FullPath), found.FullPath)
			return true
		case p.settings.DefaultOpenFolderInFileManager:
			p.api.OpenDirectory(found.FullPath, "")
			return true
		default:
			// Otherwise the folder becomes the query, so that what it
			// holds is listed.
			p.api.ChangeQuery(pathWithKeyword(found.FullPath, query.ActionKeyword), false)
			return false
		}
	}
	return result
}

// openFolderResult is the folder a path names, which is offered first so
// that Enter opens it.
func (p *Plugin) openFolderResult(directory, actionKeyword string) core.Result {
	name := filepath.Base(directory)
	if name == "" || name == string(filepath.Separator) {
		name = directory
	}

	result := core.NewResult(name, directory, folderIcon, 0)
	result.SetCopyText(directory)
	result.AutoCompleteText = pathWithKeyword(directory, actionKeyword)
	result.ContextData = SearchResult{FullPath: directory, Type: ResultTypeFolder}
	result.Preview.FilePath = directory
	result.TitleToolTip = p.words.T("plugin_explorer_plugin_ToolTipOpenDirectory")

	result.Action = func(action core.ActionContext) bool {
		switch {
		case action.SpecialKeyState.CtrlPressed:
			p.api.OpenDirectory(filepath.Dir(directory), directory)
			return true
		default:
			p.api.OpenDirectory(directory, "")
			return true
		}
	}
	return result
}

// driveResult is a drive, which shows how full it is.
func (p *Plugin) driveResult(directory, actionKeyword string) core.Result {
	letter := strings.ToUpper(strings.TrimSuffix(directory, string(filepath.Separator)))
	if len(letter) > 1 {
		letter = letter[:1]
	}

	free, total, ok := driveSpace(directory)
	subtitle := ""
	used := 0
	if ok {
		subtitle = p.words.Tf("plugin_explorer_diskfreespace", readableSize(free), readableSize(total))
		if total > 0 {
			used = int((total - free) * 100 / total)
		}
	}

	result := core.NewResult(letter, subtitle, directory, 500)
	result.AutoCompleteText = pathWithKeyword(directory, actionKeyword)
	result.ProgressBar = core.Progress(used)
	result.ProgressBarColor = "#26a0da"
	if used >= 90 {
		result.ProgressBarColor = "#da2626"
	}
	result.ContextData = SearchResult{FullPath: directory, Type: ResultTypeVolume}
	result.TitleToolTip = directory

	result.Action = func(core.ActionContext) bool {
		p.api.ChangeQuery(pathWithKeyword(directory, actionKeyword), false)
		return false
	}
	return result
}

// filePreviewText is what the preview panel says about a file.
func (p *Plugin) filePreviewText(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return path
	}

	lines := []string{path}
	if p.settings.ShowFileSizeInPreviewPanel {
		lines = append(lines, readableSize(info.Size()))
	}
	if p.settings.ShowModifiedDateInPreviewPanel {
		lines = append(lines, p.words.T("plugin_explorer_preview_modified")+" "+
			info.ModTime().Format(p.settings.PreviewPanelDateFormat+" "+p.settings.PreviewPanelTimeFormat))
	}
	if p.settings.ShowCreatedDateInPreviewPanel {
		lines = append(lines, p.words.T("plugin_explorer_preview_created")+" "+
			creationTime(info).Format(p.settings.PreviewPanelDateFormat+" "+p.settings.PreviewPanelTimeFormat))
	}
	if p.settings.ShowFileAgeInPreviewPanel {
		lines = append(lines, core.TimeAgo(info.ModTime()))
	}
	return strings.Join(lines, "\n")
}

// fileInformation is what a file's tooltip says.
func fileInformation(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return path
	}
	return path + "\n" + readableSize(info.Size())
}

// pathWithKeyword writes a path so that it searches inside itself when it
// is chosen, which is what the autocomplete text of a folder is.
func pathWithKeyword(path, actionKeyword string) string {
	if actionKeyword == "" || actionKeyword == globalKeyword {
		return path
	}
	return actionKeyword + " " + path
}

// isHomeFolder reports whether a path is inside the user's own folders.
func isHomeFolder(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(path), strings.ToLower(home))
}
