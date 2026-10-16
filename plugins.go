package main

import (
	"context"
	"path/filepath"
	"strings"

	"launch/core"

	// The plugins the launcher ships with.
	"launch/plugins/browserbookmark"
	"launch/plugins/calculator"
	"launch/plugins/copyq"
	"launch/plugins/explorer"
	"launch/plugins/pluginindicator"
	"launch/plugins/pluginsmanager"
	"launch/plugins/processkiller"
	"launch/plugins/program"
	"launch/plugins/shell"
	"launch/plugins/sys"
	"launch/plugins/url"
	"launch/plugins/websearch"
	"launch/plugins/windowssettings"
)

// bundledPlugins are the plugins the launcher is built with, in the order
// the settings window lists them.
var bundledPlugins = []func() core.Plugin{
	func() core.Plugin { return calculator.New() },
	func() core.Plugin { return copyq.New() },
	func() core.Plugin { return program.New() },
	func() core.Plugin { return explorer.New() },
	func() core.Plugin { return browserbookmark.New() },
	func() core.Plugin { return sys.New() },
	func() core.Plugin { return shell.New() },
	func() core.Plugin { return url.New() },
	func() core.Plugin { return websearch.New() },
	func() core.Plugin { return windowssettings.New() },
	func() core.Plugin { return processkiller.New() },
	func() core.Plugin { return pluginsmanager.New() },
	func() core.Plugin { return pluginindicator.New() },
}

// loadPlugins gives the launcher its plugins and initializes them. It runs
// on a goroutine, because a plugin may take a moment to read what it needs,
// and the window is up already.
func loadPlugins(a *app) {
	metadatas := make([]*core.PluginMetadata, 0, len(bundledPlugins))
	pairs := make([]core.PluginPair, 0, len(bundledPlugins))

	// The plugins the user installed live in the launcher's own plugins
	// directory, each with a plugin.json saying what runs it.
	external, unsupported := core.LoadExternalPlugins(
		filepath.Join(a.dataDir, "Plugins"), a.plugins)
	for i := range external {
		metadatas = append(metadatas, &external[i].Metadata)
	}
	pairs = append(pairs, external...)
	if len(unsupported) > 0 {
		a.api.LogWarn("launcher", "these plugins cannot be run: "+strings.Join(unsupported, ", "))
	}

	for _, makePlugin := range bundledPlugins {
		plugin := makePlugin()
		metadata := plugin.Metadata()
		metadata.PluginDirectory = filepath.Join(a.dataDir, "Plugins", metadata.ID)
		if metadata.Name == "" {
			metadata.Name = metadata.ID
		}
		metadatas = append(metadatas, &metadata)
		pairs = append(pairs, core.PluginPair{Plugin: plugin, Metadata: metadata})
	}

	// What the user set for a plugin wins over what it asks for.
	a.plugins.UpdatePluginSettings(metadatas)
	for i := range pairs {
		pairs[i].Metadata = *metadatas[i]
	}

	// The launcher's own plugins are given an icon of the launcher's when
	// they ship without one.
	for i := range pairs {
		if pairs[i].Metadata.IcoPath == "" {
			pairs[i].Metadata.IcoPath = filepath.Join("Images", pairs[i].Metadata.ID+".png")
		}
	}

	for _, pair := range pairs {
		a.manager.Load(pair)
	}

	// The manager's plugins are loaded before the first query, so that the
	// query builder knows their action keywords.
	a.manager.Initialize(context.Background())

	a.history.UpdatePluginDirectories(a.manager)
	_ = a.plugins.Save()

	// The plugins that need the window, such as the one that lists the
	// launcher's own plugins, are told it is there.
	a.update(func() {
		a.api.LogInfo("launcher", "plugins loaded")
	})
}
