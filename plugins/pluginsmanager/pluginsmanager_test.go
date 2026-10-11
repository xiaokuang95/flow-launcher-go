package pluginsmanager

import (
	"context"
	"strings"
	"testing"

	"launch/core"
)

// fakeStore stands in for the plugin store and the launcher's own plugins,
// so that a test watches what the plugin asked for rather than installing
// anything.
type fakeStore struct {
	manifest      []core.UserPlugin
	installed     []core.PluginPair
	changed       []string
	installs      []string
	removals      []string
	updates       []string
	manifestCalls int
	downloadFails bool
}

// newTestPlugin makes the plugin over a launcher that has the plugins
// given, which is what it offers to remove and to update.
func newTestPlugin(t *testing.T, installed ...core.PluginPair) (*Plugin, *fakeStore) {
	t.Helper()

	store := &fakeStore{installed: installed}

	// The launcher's own plugins are read from its manager, so the test
	// gives it one holding the plugins the launcher has.
	manager := core.NewPluginManager()
	for _, pair := range installed {
		manager.Load(pair)
	}

	api := core.NewPublicAPI(manager, t.TempDir(), "test")
	api.SetMatcher(core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionRegular})
	api.Hooks.GetPluginManifest = func() []core.UserPlugin { return store.manifest }
	api.Hooks.UpdatePluginManifest = func(bool) error { store.manifestCalls++; return nil }
	api.Hooks.InstallPlugin = func(plugin core.UserPlugin, zip string) bool {
		store.installs = append(store.installs, plugin.Name)
		return true
	}
	api.Hooks.UninstallPlugin = func(id string, removeSettings bool) error {
		store.removals = append(store.removals, id)
		return nil
	}
	api.Hooks.UpdatePlugin = func(id string, plugin core.UserPlugin, zip string) error {
		store.updates = append(store.updates, plugin.Name)
		return nil
	}
	api.Hooks.PluginModified = func(id string) bool { return false }
	api.Hooks.ChangeQuery = func(query string, requery bool) { store.changed = append(store.changed, query) }
	api.Hooks.HideMainWindow = func() {}
	api.Hooks.StartLoadingBar = func() {}
	api.Hooks.StopLoadingBar = func() {}
	api.Hooks.ShowMsg = func(title, subTitle, icon string) {}
	api.Hooks.ShowMsgError = func(title, subTitle string) {}

	plugin := New()
	plugin.api = api
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	// A test never reaches the network: what a download would answer with
	// is what the test says it answers with.
	plugin.downloader = store
	return plugin, store
}

// Download stands in for the network, writing the file a plugin would be
// installed from.
func (f *fakeStore) Download(ctx context.Context, url, path string) error {
	if f.downloadFails {
		return errNoDownload
	}
	return writePluginFile(path, "Test Plugin", "test-id")
}

func titlesOf(results []core.Result) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

func TestTheThreeThingsAreOfferedWhenNothingIsTyped(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{})
	if err != nil {
		t.Fatal(err)
	}
	titles := titlesOf(results)
	for _, want := range []string{installCommand, uninstallCommand, updateCommand} {
		if !containsString(titles, want) {
			t.Errorf("the results %v do not offer %q", titles, want)
		}
	}
	for _, result := range results {
		if result.AutoCompleteText == "" {
			t.Errorf("%q has nothing to complete with", result.Title)
		}
	}
}

func TestChoosingOnePutsItInTheQueryBox(t *testing.T) {
	plugin, store := newTestPlugin(t)

	results, _ := plugin.Query(context.Background(), core.Query{})
	hide := results[0].Action(core.ActionContext{})

	if hide {
		t.Error("the launcher hid, want it to stay up")
	}
	if len(store.changed) != 1 || store.changed[0] != "pm install " {
		t.Errorf("the query box was given %v, want the command", store.changed)
	}
}

func TestTheThreeAreFoundByWhatIsTyped(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "unin"})
	if len(results) != 1 || results[0].Title != uninstallCommand {
		t.Errorf("the results are %v, want only uninstall", titlesOf(results))
	}
}

func TestInstallingOffersThePluginsNotInstalledYet(t *testing.T) {
	plugin, store := newTestPlugin(t, core.PluginPair{Metadata: core.PluginMetadata{ID: "a", Name: "Alpha"}})
	store.manifest = []core.UserPlugin{
		{ID: "a", Name: "Alpha", Author: "Someone", Description: "First", Version: "1.0"},
		{ID: "b", Name: "Beta", Author: "Someone", Description: "Second", Version: "1.0"},
	}

	results, err := plugin.Query(context.Background(), core.Query{Search: "install", SearchTerms: []string{"install"}})
	if err != nil {
		t.Fatal(err)
	}
	titles := titlesOf(results)
	if len(titles) != 1 {
		t.Fatalf("the results are %v, want only the plugin that is not installed", titles)
	}
	if !contains(titles[0], "Beta") {
		t.Errorf("the result is %q, want Beta", titles[0])
	}
	if store.manifestCalls != 1 {
		t.Errorf("the store was read %d times, want once", store.manifestCalls)
	}
}

func TestInstallingAsksTheLauncher(t *testing.T) {
	plugin, store := newTestPlugin(t)
	store.manifest = []core.UserPlugin{{ID: "b", Name: "Beta", Version: "1.0", UrlDownload: "https://example.com/b.zip"}}

	results, _ := plugin.Query(context.Background(), core.Query{Search: "install", SearchTerms: []string{"install"}})
	results[0].Action(core.ActionContext{})

	// The file was downloaded, so the launcher is asked to install the
	// plugin it holds.
	if len(store.installs) != 1 || store.installs[0] != "Beta" {
		t.Errorf("the launcher was asked to install %v, want Beta", store.installs)
	}
}

func TestADownloadThatFailsSaysSo(t *testing.T) {
	plugin, store := newTestPlugin(t)
	store.manifest = []core.UserPlugin{{ID: "b", Name: "Beta", Version: "1.0", UrlDownload: "https://example.com/b.zip"}}
	store.downloadFails = true

	said := ""
	plugin.api.Hooks.ShowMsgError = func(title, subTitle string) { said = title }

	results, _ := plugin.Query(context.Background(), core.Query{Search: "install", SearchTerms: []string{"install"}})
	results[0].Action(core.ActionContext{})

	if len(store.installs) != 0 {
		t.Errorf("the launcher was asked to install %v although the download failed", store.installs)
	}
	if said == "" {
		t.Error("nothing was said about the failed download")
	}
}

func TestUninstallingOffersThePluginsTheLauncherHas(t *testing.T) {
	plugin, store := newTestPlugin(t,
		core.PluginPair{Metadata: core.PluginMetadata{ID: "a", Name: "Alpha"}},
		core.PluginPair{Metadata: core.PluginMetadata{ID: "b", Name: "Beta"}},
	)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "uninstall", SearchTerms: []string{"uninstall"}})
	if len(results) != 2 {
		t.Fatalf("the results are %v, want both plugins", titlesOf(results))
	}

	results[0].Action(core.ActionContext{})
	if len(store.removals) != 1 {
		t.Errorf("the launcher was asked to remove %v, want one", store.removals)
	}
}

func TestUpdatingOffersOnlyNewerVersions(t *testing.T) {
	plugin, store := newTestPlugin(t,
		core.PluginPair{Metadata: core.PluginMetadata{ID: "a", Name: "Alpha", Version: "1.0"}},
		core.PluginPair{Metadata: core.PluginMetadata{ID: "b", Name: "Beta", Version: "1.0"}},
		core.PluginPair{Metadata: core.PluginMetadata{ID: "d", Name: "Delta", Version: "1.0"}},
	)
	store.manifest = []core.UserPlugin{
		{ID: "a", Name: "Alpha", Version: "2.0"},
		{ID: "b", Name: "Beta", Version: "1.0"},
		{ID: "c", Name: "Gamma", Version: "1.5"},
	}

	results, _ := plugin.Query(context.Background(), core.Query{Search: "update", SearchTerms: []string{"update"}})
	titles := titlesOf(results)

	if len(titles) != 1 {
		t.Fatalf("the results are %v, want only the plugin with a newer version", titles)
	}
	if !contains(titles[0], "Alpha") {
		t.Errorf("the result is %q, want Alpha", titles[0])
	}
	// The newer version is shown beside the one the launcher has.
	if !contains(titles[0], "1.0") || !contains(titles[0], "2.0") {
		t.Errorf("the result is %q, want both versions", titles[0])
	}
}

func TestVersionsAreComparedPartByPart(t *testing.T) {
	cases := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"2.0", "1.0", true},
		{"1.10", "1.9", true},
		{"1.0", "1.0", false},
		{"1.0", "2.0", false},
		{"1.0.1", "1.0", true},
		{"1.0", "1.0.1", false},
	}
	for _, c := range cases {
		if got := isNewer(c.candidate, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.candidate, c.current, got, c.want)
		}
	}
}

func TestAZipFileIsRecognised(t *testing.T) {
	if isZipLink("https://example.com/plugin.zip") != true {
		t.Error("a link to a file was not recognised")
	}
	if isZipLink("https://example.com/plugin") != false {
		t.Error("a link that is not to a file was taken for one")
	}
	if isZipPath("not-a-file.zip") {
		t.Error("a file that is not there was taken for one")
	}
}

// contains reports whether a text holds a part, which is how the tests
// read a result's title.
func contains(value, want string) bool {
	return strings.Contains(value, want)
}

// containsString reports whether a list holds a value.
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
