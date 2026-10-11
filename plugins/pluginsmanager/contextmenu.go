package pluginsmanager

import (
	"regexp"
	"strings"

	"launch/core"
)

// The images the menu's items are drawn with, as the reference names them.
const (
	sourceCodeIcon = "Images/sourcecode.png"
	requestIcon    = "Images/request.png"
	manifestIcon   = "Images/manifestsite.png"
)

// branchInSourceCode is the part of a repository link that names a branch,
// which is not part of where its issues are.
var branchInSourceCode = regexp.MustCompile(`/tree/\w+$`)

// LoadContextMenus offers where a plugin of the store can be looked at:
// its own page, its source, its issues, and the repository every plugin of
// the store is listed in.
func (p *Plugin) LoadContextMenus(selectedResult core.Result) []core.Result {
	plugin, ok := selectedResult.ContextData.(core.UserPlugin)
	if !ok {
		return nil
	}

	website := core.NewResult(
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_openwebsite_title"),
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_openwebsite_subtitle"),
		selectedResult.IcoPath, 0)
	website.PluginDirectory = selectedResult.PluginDirectory
	website.Action = func(action core.ActionContext) bool {
		p.api.OpenUrl(plugin.Website, nil)
		return true
	}

	sourceCode := core.NewResult(
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_gotosourcecode_title"),
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_gotosourcecode_subtitle"),
		sourceCodeIcon, 0)
	sourceCode.Action = func(action core.ActionContext) bool {
		p.api.OpenUrl(plugin.UrlSourceCode, nil)
		return true
	}

	issue := core.NewResult(
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_newissue_title"),
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_newissue_subtitle"),
		requestIcon, 0)
	issue.Action = func(action core.ActionContext) bool {
		p.api.OpenUrl(issuesURL(plugin.UrlSourceCode), nil)
		return true
	}

	manifest := core.NewResult(
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_pluginsmanifest_title"),
		p.words.T("plugin_pluginsmanager_plugin_contextmenu_pluginsmanifest_subtitle"),
		manifestIcon, 0)
	manifest.Action = func(action core.ActionContext) bool {
		p.api.OpenUrl(pluginsManifestURL, nil)
		return true
	}

	return []core.Result{website, sourceCode, issue, manifest}
}

// pluginsManifestURL is the repository that lists the plugins of the
// store.
const pluginsManifestURL = "https://github.com/Flow-Launcher/Flow.Launcher.PluginsManifest"

// issuesURL is where a plugin's issues are reported: its source, with the
// branch taken off, and the issues of that repository. A source that is
// not on GitHub is offered as it is.
func issuesURL(sourceCode string) string {
	if !strings.HasPrefix(sourceCode, "https://github.com") {
		return sourceCode
	}
	return branchInSourceCode.ReplaceAllString(sourceCode, "") + "/issues"
}
