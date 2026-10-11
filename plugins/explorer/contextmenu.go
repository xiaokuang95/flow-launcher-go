package explorer

import (
	"os"
	"path/filepath"
	"strings"

	"launch/core"
)

// Deleter removes a file or a folder, which a test watches rather than
// letting it happen to the machine it runs on.
type Deleter interface {
	// Delete removes the path, and everything under it when it is a
	// folder.
	Delete(path string, isDir bool) error
}

// systemDeleter removes what the user asked to remove, the way the
// reference does: a file by itself, a folder with everything under it.
type systemDeleter struct{}

func (systemDeleter) Delete(path string, isDir bool) error {
	if isDir {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// LoadContextMenus offers what can be done with the file or the folder a
// result stands for: opening it elsewhere, copying it, pinning it, ending
// it and running it as somebody else.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	found, ok := selectedResult.ContextData.(SearchResult)
	if !ok || found.FullPath == "" {
		return nil
	}

	isFile := found.Type == ResultTypeFile
	isFolder := found.Type == ResultTypeFolder
	isVolume := found.Type == ResultTypeVolume

	var items []core.Result

	// Opening it in the programs the user chose for the job.
	if isFile && p.settings.EditorPath != "" {
		items = append(items, p.openWithEditorResult(found, p.settings.EditorPath))
	}
	if (isFolder || isVolume) && p.settings.FolderEditorPath != "" {
		items = append(items, p.openWithEditorResult(found, p.settings.FolderEditorPath))
	}
	if isFolder {
		items = append(items, p.openWithShellResult(found))
	}

	items = append(items, p.openContainingFolderResult(found))

	if isFile {
		items = append(items, p.openWithMenuResult(found))
	}

	// The folders a search leaves out are the ones the user excluded, so
	// a folder can be left out of a search from here.
	if isFolder {
		items = append(items, p.excludeFromSearchResult(found))
	}

	items = append(items, p.quickAccessResult(found))
	items = append(items,
		p.copyResult(found, "plugin_explorer_copypath", "plugin_explorer_copypath_subtitle", found.FullPath, false),
		p.copyResult(found, "plugin_explorer_copyname", "plugin_explorer_copyname_subtitle", filepath.Base(found.FullPath), false),
		p.copyResult(found, "plugin_explorer_copyfilefolder", fileSubtitleKey(found), found.FullPath, true),
	)

	if isFile || isFolder {
		items = append(items, p.deleteResult(found, isFile))
	}

	if isFile && canRunAsDifferentUser(found.FullPath) {
		items = append(items, p.runAsDifferentUserResult(found))
	}

	return items
}

// fileSubtitleKey is what the copy item says, which is about a file or
// about a folder.
func fileSubtitleKey(found SearchResult) string {
	if found.Type == ResultTypeFile {
		return "plugin_explorer_copyfile_subtitle"
	}
	return "plugin_explorer_copyfolder_subtitle"
}

// openWithEditorResult opens the file, or the folder, in a program the
// user chose.
func (p *Plugin) openWithEditorResult(found SearchResult, editorPath string) core.Result {
	name := p.words.T("plugin_explorer_openwitheditor") + " " + strings.TrimSuffix(filepath.Base(editorPath), filepath.Ext(editorPath))

	icon := fileIcon
	if found.Type != ResultTypeFile {
		icon = folderIcon
	}
	result := core.NewResult(name, "", icon, 0)
	result.Action = func(action core.ActionContext) bool {
		if err := p.openIn(editorPath, found.FullPath); err != nil {
			p.api.ShowMsgError(p.words.Tf("plugin_explorer_openwitheditor_error", found.FullPath, filepath.Base(editorPath), editorPath), "")
			return false
		}
		return true
	}
	return result
}

// openWithShellResult opens the folder in the shell the user chose.
func (p *Plugin) openWithShellResult(found SearchResult) core.Result {
	shellPath := p.settings.ShellPath
	name := p.words.T("plugin_explorer_openwithshell") + " " + strings.TrimSuffix(filepath.Base(shellPath), filepath.Ext(shellPath))

	result := core.NewResult(name, "", folderIcon, 0)
	result.Action = func(action core.ActionContext) bool {
		if err := p.openFolderIn(shellPath, found.FullPath); err != nil {
			p.api.ShowMsgError(p.words.Tf("plugin_explorer_openwithshell_error", found.FullPath, filepath.Base(shellPath), shellPath), "")
			return false
		}
		return true
	}
	return result
}

// openContainingFolderResult shows the folder the item is in, with the
// item itself picked out.
func (p *Plugin) openContainingFolderResult(found SearchResult) core.Result {
	result := core.NewResult(
		p.words.T("plugin_explorer_opencontainingfolder"),
		p.words.T("plugin_explorer_opencontainingfolder_subtitle"),
		folderIcon, 0)
	result.Action = func(action core.ActionContext) bool {
		p.api.OpenDirectory(filepath.Dir(found.FullPath), found.FullPath)
		return true
	}
	return result
}

// openWithMenuResult shows the machine's own list of programs a file can
// be opened with.
func (p *Plugin) openWithMenuResult(found SearchResult) core.Result {
	result := core.NewResult(
		p.words.T("plugin_explorer_openwith"),
		p.words.T("plugin_explorer_openwith_subtitle"),
		"Images/context_menu.png", 0)
	result.Action = func(action core.ActionContext) bool {
		openWithMenu(found.FullPath)
		return true
	}
	return result
}

// excludeFromSearchResult leaves the folder, and what is under it, out of
// the next searches.
func (p *Plugin) excludeFromSearchResult(found SearchResult) core.Result {
	result := core.NewResult(
		p.words.T("plugin_explorer_excludefromindexsearch"),
		p.words.T("plugin_explorer_path")+" "+found.FullPath,
		"Images/excludeindexpath.png", 0)
	result.Action = func(action core.ActionContext) bool {
		for _, excluded := range p.settings.IndexSearchExcludedSubdirectoryPaths {
			if strings.EqualFold(excluded.Path, found.FullPath) {
				return false
			}
		}
		p.settings.IndexSearchExcludedSubdirectoryPaths = append(
			p.settings.IndexSearchExcludedSubdirectoryPaths, AccessLink{Path: found.FullPath})
		p.save()

		p.api.ShowMsg(p.words.T("plugin_explorer_excludedfromindexsearch_msg"),
			p.words.T("plugin_explorer_path")+" "+found.FullPath, explorerIcon)
		return false
	}
	return result
}

// quickAccessResult pins the item, or unpins it when it is pinned
// already.
func (p *Plugin) quickAccessResult(found SearchResult) core.Result {
	pinned := false
	for _, link := range p.settings.QuickAccessLinks {
		if strings.EqualFold(link.Path, found.FullPath) {
			pinned = true
			break
		}
	}

	if !pinned {
		result := core.NewResult(
			p.words.T("plugin_explorer_add_to_quickaccess_title"),
			p.words.T("plugin_explorer_add_to_quickaccess_subtitle"),
			quickAccessIcon, 0)
		result.TitleToolTip = p.words.T("plugin_explorer_contextmenu_titletooltip")
		result.SubTitleToolTip = result.TitleToolTip
		result.Action = func(action core.ActionContext) bool {
			p.settings.QuickAccessLinks = append(p.settings.QuickAccessLinks, AccessLink{
				Name: filepath.Base(found.FullPath),
				Path: found.FullPath,
			})
			p.save()

			p.api.ShowMsg(p.words.T("plugin_explorer_addfilefoldersuccess"),
				p.words.T("plugin_explorer_addfilefoldersuccess_detail"), explorerIcon)
			return true
		}
		return result
	}

	result := core.NewResult(
		p.words.T("plugin_explorer_remove_from_quickaccess_title"),
		p.words.T("plugin_explorer_remove_from_quickaccess_subtitle"),
		"Images/removequickaccess.png", 0)
	result.TitleToolTip = p.words.T("plugin_explorer_contextmenu_remove_titletooltip")
	result.SubTitleToolTip = result.TitleToolTip
	result.Action = func(action core.ActionContext) bool {
		kept := make([]AccessLink, 0, len(p.settings.QuickAccessLinks))
		for _, link := range p.settings.QuickAccessLinks {
			if !strings.EqualFold(link.Path, found.FullPath) {
				kept = append(kept, link)
			}
		}
		p.settings.QuickAccessLinks = kept
		p.save()

		p.api.ShowMsg(p.words.T("plugin_explorer_removefilefoldersuccess"),
			p.words.T("plugin_explorer_removefilefoldersuccess_detail"), explorerIcon)
		return true
	}
	return result
}

// copyResult copies the path, the name, or the item itself.
func (p *Plugin) copyResult(found SearchResult, titleKey, subtitleKey, text string, asFile bool) core.Result {
	result := core.NewResult(p.words.T(titleKey), p.words.T(subtitleKey), "Images/copy.png", 0)
	result.Action = func(action core.ActionContext) bool {
		p.api.CopyToClipboard(text, asFile, true)
		return true
	}
	return result
}

// deleteResult removes the file, or the folder and what is under it, once
// the user has said so.
func (p *Plugin) deleteResult(found SearchResult, isFile bool) core.Result {
	subtitleKey := "plugin_explorer_deletefolder_subtitle"
	if isFile {
		subtitleKey = "plugin_explorer_deletefile_subtitle"
	}

	result := core.NewResult(
		p.words.T("plugin_explorer_deletefilefolder"),
		p.words.T(subtitleKey),
		"Images/deletefilefolder.png", 0)
	result.Action = func(action core.ActionContext) bool {
		answer := p.api.ShowMsgBox(
			p.words.Tf("plugin_explorer_delete_folder_link", found.FullPath),
			p.words.T("plugin_explorer_deletefilefolder"),
			core.MessageBoxOKCancel, core.MessageBoxIconWarning)
		if answer == core.MessageBoxResultCancel {
			return false
		}

		if err := p.deleter.Delete(found.FullPath, !isFile); err != nil {
			p.api.LogError("Explorer", "deleting "+found.FullPath+" failed: "+err.Error())
			p.api.ShowMsgError(p.words.Tf("plugin_explorer_fail_to_delete", found.FullPath), "")
			return false
		}

		p.api.ShowMsg(p.words.T("plugin_explorer_deletefilefoldersuccess"),
			p.words.Tf("plugin_explorer_deletefilefoldersuccess_detail", found.FullPath), explorerIcon)
		return true
	}
	return result
}

// runAsDifferentUserResult runs the file as another user.
func (p *Plugin) runAsDifferentUserResult(found SearchResult) core.Result {
	result := core.NewResult(
		p.words.T("plugin_explorer_runasdifferentuser"),
		p.words.T("plugin_explorer_runasdifferentuser_subtitle"),
		"Images/user.png", 0)
	result.Action = func(action core.ActionContext) bool {
		if err := runAsDifferentUser(found.FullPath); err != nil {
			p.api.ShowMsgError(p.words.T("plugin_explorer_plugin_name"),
				p.words.Tf("plugin_explorer_file_not_found", err.Error()))
			return false
		}
		return true
	}
	return result
}

// canRunAsDifferentUser reports whether a file is one that can be run at
// all, which is what the reference asks before offering it.
func canRunAsDifferentUser(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".bat", ".msi":
		return true
	}
	return false
}

// save writes the settings out, so that what the user just changed is
// there after a restart.
func (p *Plugin) save() {
	_ = core.SaveSettingJsonStorage(p.api, "Explorer", "Settings.json", p.settings)
}
