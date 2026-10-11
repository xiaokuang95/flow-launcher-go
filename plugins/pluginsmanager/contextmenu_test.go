package pluginsmanager

import (
	"testing"

	"launch/core"
)

// watchBrowser makes the plugin's browser the test's own, so that what a
// menu item asks to open can be watched.
func watchBrowser(plugin *Plugin) *[]string {
	opened := &[]string{}
	plugin.api.Hooks.OpenURL = func(rawURL string, inPrivate *bool) {
		*opened = append(*opened, rawURL)
	}
	return opened
}

func TestTheStoreMenuOpensThePluginPages(t *testing.T) {
	plugin, _ := newTestPlugin(t)
	opened := watchBrowser(plugin)
	storePlugin := core.UserPlugin{
		Name:          "Putty",
		Website:       "https://github.com/jjw24/Flow.Launcher.Plugin.Putty",
		UrlSourceCode: "https://github.com/jjw24/Flow.Launcher.Plugin.Putty/tree/master",
	}

	items := plugin.LoadContextMenus(core.Result{ContextData: storePlugin})
	if len(items) != 4 {
		t.Fatalf("the menu has %v, want four items", titlesOf(items))
	}

	for _, item := range items {
		item.Action(core.ActionContext{})
	}

	want := []string{
		"https://github.com/jjw24/Flow.Launcher.Plugin.Putty",
		"https://github.com/jjw24/Flow.Launcher.Plugin.Putty/tree/master",
		"https://github.com/jjw24/Flow.Launcher.Plugin.Putty/issues",
		"https://github.com/Flow-Launcher/Flow.Launcher.PluginsManifest",
	}
	if len(*opened) != len(want) {
		t.Fatalf("the browser was asked for %v, want %v", *opened, want)
	}
	for i := range want {
		if (*opened)[i] != want[i] {
			t.Errorf("the browser was asked for %q, want %q", (*opened)[i], want[i])
		}
	}
}

func TestTheStoreMenuLeavesOtherResultsAlone(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	if items := plugin.LoadContextMenus(core.Result{ContextData: "something else"}); len(items) != 0 {
		t.Errorf("the menu has %v, want nothing for a result the store did not make", titlesOf(items))
	}
}

func TestIssuesAreFoundUnderTheSourceCode(t *testing.T) {
	for _, test := range []struct {
		sourceCode string
		want       string
	}{
		{"https://github.com/jjw24/Flow.Launcher.Plugin.Putty/tree/master", "https://github.com/jjw24/Flow.Launcher.Plugin.Putty/issues"},
		{"https://github.com/Flow-Launcher/Flow.Launcher", "https://github.com/Flow-Launcher/Flow.Launcher/issues"},
		{"https://gitlab.com/someone/plugin", "https://gitlab.com/someone/plugin"},
		{"", ""},
	} {
		if got := issuesURL(test.sourceCode); got != test.want {
			t.Errorf("issuesURL(%q) = %q, want %q", test.sourceCode, got, test.want)
		}
	}
}
