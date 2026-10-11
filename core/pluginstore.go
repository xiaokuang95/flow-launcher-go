package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The plugin store: the list of plugins the launcher can install, which is
// a file the Flow Launcher project keeps in a repository of its own.
//
// The same list is offered by several addresses, and the first that answers
// is used, so that a store that cannot be reached in one place is still
// reachable in another.

// pluginStoreURLs are where the list of plugins is kept, the first being
// the project's own.
var pluginStoreURLs = []string{
	"https://raw.githubusercontent.com/Flow-Launcher/Flow.Launcher.PluginsManifest/main/plugins.json",
	"https://fastly.jsdelivr.net/gh/Flow-Launcher/Flow.Launcher.PluginsManifest@main/plugins.json",
	"https://gcore.jsdelivr.net/gh/Flow-Launcher/Flow.Launcher.PluginsManifest@main/plugins.json",
	"https://cdn.jsdelivr.net/gh/Flow-Launcher/Flow.Launcher.PluginsManifest@main/plugins.json",
}

// storeFreshness is how long a list that was read once is used before it is
// read again.
const storeFreshness = 2 * time.Minute

// PluginStore is the list of plugins the launcher can install.
type PluginStore struct {
	// Version is the launcher's version, which decides which plugins are
	// offered: a plugin that needs a newer launcher is left out.
	Version string
	// Client is what the list is read through; nil uses the launcher's own
	// client, with the user's proxy setting.
	Client *http.Client
	// CachePath is where the list is kept between runs; empty keeps it in
	// memory alone.
	CachePath string

	mu        sync.Mutex
	plugins   []UserPlugin
	fetchedAt time.Time
}

// NewPluginStore makes a store over the launcher's own client.
func NewPluginStore(version, cachePath string) *PluginStore {
	return &PluginStore{Version: version, CachePath: cachePath}
}

// Plugins is the list of plugins, read again when it is old.
func (s *PluginStore) Plugins(ctx context.Context, force bool) ([]UserPlugin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !force && len(s.plugins) > 0 && time.Since(s.fetchedAt) < storeFreshness {
		return s.plugins, nil
	}

	// A list read on an earlier run is used when the store cannot be
	// reached now, so that the plugin store is not empty offline.
	if plugins, err := s.fetch(ctx); err == nil && len(plugins) > 0 {
		s.plugins = plugins
		s.fetchedAt = time.Now()
		s.save(plugins)
		return s.plugins, nil
	} else if err != nil && len(s.plugins) == 0 {
		if cached := s.load(); len(cached) > 0 {
			s.plugins = cached
			return s.plugins, nil
		}
		return nil, err
	}

	return s.plugins, nil
}

// fetch reads the list from the first address that answers.
func (s *PluginStore) fetch(ctx context.Context) ([]UserPlugin, error) {
	var lastErr error
	for _, address := range pluginStoreURLs {
		plugins, err := s.fetchFrom(ctx, address)
		if err == nil && len(plugins) > 0 {
			return plugins, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("the plugin store answered with nothing")
	}
	return nil, lastErr
}

// fetchFrom reads the list from one address.
func (s *PluginStore) fetchFrom(ctx context.Context, address string) ([]UserPlugin, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("the plugin store answered %s", response.Status)
	}

	var plugins []UserPlugin
	if err := json.NewDecoder(response.Body).Decode(&plugins); err != nil {
		return nil, err
	}

	// A plugin that needs a newer launcher than this one is left out, so
	// that the user is not offered something that cannot run.
	offered := make([]UserPlugin, 0, len(plugins))
	for _, plugin := range plugins {
		if s.Version != "" && !versionSatisfied(plugin.MinimumAppVersion, s.Version) {
			continue
		}
		offered = append(offered, plugin)
	}
	return offered, nil
}

// save keeps the list for the next run.
func (s *PluginStore) save(plugins []UserPlugin) {
	if s.CachePath == "" {
		return
	}
	storage := JSONStorage[[]UserPlugin]{Path: s.CachePath}
	if err := storage.Save(plugins); err != nil {
		// A list that cannot be kept is read again next time.
		_ = err
	}
}

// load reads the list kept from an earlier run.
func (s *PluginStore) load() []UserPlugin {
	if s.CachePath == "" {
		return nil
	}
	return JSONStorage[[]UserPlugin]{Path: s.CachePath}.Load(nil)
}

// versionSatisfied reports whether a launcher version is new enough for a
// plugin. An empty requirement is satisfied by anything.
func versionSatisfied(required, version string) bool {
	if strings.TrimSpace(required) == "" {
		return true
	}
	requiredParts := versionParts(required)
	versionPartsOfLauncher := versionParts(version)

	for i := range requiredParts {
		var have int
		if i < len(versionPartsOfLauncher) {
			have = versionPartsOfLauncher[i]
		}
		if have != requiredParts[i] {
			return have > requiredParts[i]
		}
	}
	return true
}

// versionParts are the numbers of a version.
func versionParts(version string) []int {
	var parts []int
	for _, part := range strings.Split(version, ".") {
		number := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			number = number*10 + int(r-'0')
		}
		parts = append(parts, number)
	}
	return parts
}

// StoreCachePath is where the list of plugins is kept between runs.
func StoreCachePath(dataDir string) string {
	return filepath.Join(dataDir, "Cache", "PluginsManifest.json")
}

// InstallPluginFromStore installs a plugin of the store, downloading the
// file it comes in and putting it where the launcher reads plugins from.
//
// The file is a zip holding the plugin and its plugin.json; the launcher
// leaves the folder as it is and reads the description out of it at the
// next start.
func InstallPluginFromStore(ctx context.Context, api *PublicAPI, pluginsDirectory string, plugin UserPlugin) error {
	if plugin.UrlDownload == "" {
		return errors.New("the plugin has no file to download")
	}

	// The download is written to the cache, then unpacked into the plugins
	// directory, so that a download that fails leaves nothing behind.
	downloads := filepath.Join(api.PluginCacheDir, "Downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(downloads, plugin.ID+".zip")
	if err := api.HttpDownloadAsync(ctx, plugin.UrlDownload, archive, nil); err != nil {
		return err
	}

	// A plugin is installed into a folder named after it, so that two
	// plugins cannot overwrite each other.
	destination := filepath.Join(pluginsDirectory, safeFolderName(plugin.Name))
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	if err := Unzip(archive, destination); err != nil {
		return err
	}

	api.LogInfo("PluginStore", "installed "+plugin.Name+" into "+destination)
	return nil
}

// safeFolderName makes a plugin's name safe to use as a folder name.
func safeFolderName(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}, name)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		cleaned = "plugin"
	}
	return cleaned
}

// Unzip unpacks a zip file into a directory, which is how a plugin is
// installed from the file it comes in.
func Unzip(archive, destination string) error {
	return unzipArchive(archive, destination)
}
