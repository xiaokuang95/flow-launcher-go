package core

import (
	"path/filepath"
	"sync"
)

// PluginSettings is what the user changed about one plugin.
type PluginSettings struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Version string `json:"Version"`

	// ActionKeywords is the plugin's keywords as the user set them.
	ActionKeywords []string `json:"ActionKeywords"`
	// DefaultActionKeywords is what the plugin asked for, kept so that the
	// settings window can offer to put it back.
	DefaultActionKeywords []string `json:"-"`

	Priority int `json:"Priority"`

	// SearchDelayTime delays this plugin's queries, in milliseconds.
	SearchDelayTime *int `json:"SearchDelayTime"`
	// DefaultSearchDelayTime is what the plugin asked for.
	DefaultSearchDelayTime *int `json:"-"`

	Disabled     bool `json:"Disabled"`
	HomeDisabled bool `json:"HomeDisabled"`
}

// PluginsSettings is the user's plugin settings, as Plugins.json holds them.
type PluginsSettings struct {
	PythonExecutablePath string `json:"PythonExecutablePath"`
	NodeExecutablePath   string `json:"NodeExecutablePath"`

	Plugins map[string]PluginSettings `json:"Plugins"`

	mu sync.Mutex `json:"-"`
	// Path is the file these settings are read from and written to.
	Path string `json:"-"`
}

// LoadPluginsSettings reads the plugin settings, or makes empty ones.
func LoadPluginsSettings(path string) *PluginsSettings {
	settings := JSONStorage[*PluginsSettings]{Path: path}.Load(nil)
	if settings == nil {
		settings = &PluginsSettings{}
	}
	if settings.Plugins == nil {
		settings.Plugins = map[string]PluginSettings{}
	}
	settings.Path = path
	return settings
}

// UpdatePluginSettings applies the user's settings to the plugins' metadata,
// and records the plugins it has not seen before.
func (s *PluginsSettings) UpdatePluginSettings(metadatas []*PluginMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Plugins == nil {
		s.Plugins = map[string]PluginSettings{}
	}

	for _, metadata := range metadatas {
		stored, exists := s.Plugins[metadata.ID]
		if !exists {
			s.Plugins[metadata.ID] = PluginSettings{
				ID:                     metadata.ID,
				Name:                   metadata.Name,
				Version:                metadata.Version,
				ActionKeywords:         append([]string(nil), metadata.ActionKeywords...),
				DefaultActionKeywords:  append([]string(nil), metadata.ActionKeywords...),
				Priority:               metadata.Priority,
				SearchDelayTime:        metadata.SearchDelayTime,
				DefaultSearchDelayTime: metadata.SearchDelayTime,
				Disabled:               metadata.Disabled,
				HomeDisabled:           metadata.HomeDisabled,
			}
			continue
		}

		// The plugin's own values fill the gaps; the user's win.
		if stored.Version == "" {
			stored.Version = metadata.Version
		}
		stored.DefaultActionKeywords = append([]string(nil), metadata.ActionKeywords...)
		stored.DefaultSearchDelayTime = metadata.SearchDelayTime

		if len(stored.ActionKeywords) > 0 {
			metadata.ActionKeywords = append([]string(nil), stored.ActionKeywords...)
			metadata.ActionKeyword = stored.ActionKeywords[0]
		} else {
			metadata.ActionKeywords = nil
			metadata.ActionKeyword = ""
		}
		metadata.Disabled = stored.Disabled
		metadata.Priority = stored.Priority
		metadata.SearchDelayTime = stored.SearchDelayTime
		metadata.HomeDisabled = stored.HomeDisabled

		s.Plugins[metadata.ID] = stored
	}
}

// GetPluginSettings returns what the user set for a plugin.
func (s *PluginsSettings) GetPluginSettings(id string) (PluginSettings, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, ok := s.Plugins[id]
	return settings, ok
}

// SetPluginSettings records what the user set for a plugin.
func (s *PluginsSettings) SetPluginSettings(id string, settings PluginSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Plugins == nil {
		s.Plugins = map[string]PluginSettings{}
	}
	s.Plugins[id] = settings
}

// RemovePluginSettings forgets a plugin, as uninstalling it does.
func (s *PluginsSettings) RemovePluginSettings(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Plugins, id)
}

// Save writes the settings out.
func (s *PluginsSettings) Save() error {
	if s.Path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return JSONStorage[*PluginsSettings]{Path: s.Path}.Save(s)
}

// pluginSettingsPath is where a plugin's own settings live, under the
// launcher's settings directory.
func pluginSettingsPath(settingsDir, pluginName, file string) string {
	return filepath.Join(settingsDir, "Plugins", pluginName, file)
}
