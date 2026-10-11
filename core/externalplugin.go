package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A plugin the launcher was not built with lives in its own folder under
// the launcher's plugins directory, with a plugin.json saying what it is
// and what runs it. A plugin written in another language is a program of
// its own, which the launcher runs for each request.

// PluginDescription is what a plugin's plugin.json says about it, as the
// reference writes it.
type PluginDescription struct {
	ID                     string   `json:"ID"`
	Name                   string   `json:"Name"`
	Description            string   `json:"Description"`
	Author                 string   `json:"Author"`
	Version                string   `json:"Version"`
	Language               string   `json:"Language"`
	Website                string   `json:"Website"`
	IcoPath                string   `json:"IcoPath"`
	ExecuteFileName        string   `json:"ExecuteFileName"`
	ActionKeyword          string   `json:"ActionKeyword"`
	ActionKeywords         []string `json:"ActionKeywords"`
	HideActionKeywordPanel bool     `json:"HideActionKeywordPanel"`
	MinimumAppVersion      string   `json:"MinimumAppVersion"`
	// SearchDelayTime delays this plugin's queries, in milliseconds.
	SearchDelayTime *int `json:"SearchDelayTime"`
}

// Metadata turns a description into what the launcher works with.
func (d PluginDescription) Metadata(directory string) PluginMetadata {
	keywords := d.ActionKeywords
	if len(keywords) == 0 && d.ActionKeyword != "" {
		keywords = []string{d.ActionKeyword}
	}
	// A plugin that names no keyword answers every query, as the
	// reference's own plugins do.
	if len(keywords) == 0 {
		keywords = []string{GlobalPluginWildcardSign}
	}

	icoPath := strings.ReplaceAll(d.IcoPath, `\`, "/")
	return PluginMetadata{
		ID:                     d.ID,
		Name:                   d.Name,
		Description:            d.Description,
		Author:                 d.Author,
		Version:                d.Version,
		Language:               d.Language,
		Website:                d.Website,
		IcoPath:                icoPath,
		ExecuteFileName:        d.ExecuteFileName,
		ExecuteFilePath:        filepath.Join(directory, d.ExecuteFileName),
		PluginDirectory:        directory,
		ActionKeyword:          keywords[0],
		ActionKeywords:         keywords,
		HideActionKeywordPanel: d.HideActionKeywordPanel,
		MinimumAppVersion:      d.MinimumAppVersion,
		SearchDelayTime:        d.SearchDelayTime,
	}
}

// LoadExternalPlugins reads the plugins that live in the launcher's
// plugins directory, and makes a plugin of each one that can be run.
//
// A plugin written in a language the launcher has no host for, such as
// .NET, is left out and named in the answer, so that the launcher can say
// what it could not load rather than pretending there is nothing there.
func LoadExternalPlugins(pluginsDirectory string, pluginsSettings *PluginsSettings) ([]PluginPair, []string) {
	entries, err := os.ReadDir(pluginsDirectory)
	if err != nil {
		return nil, nil
	}

	var pairs []PluginPair
	var unsupported []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(pluginsDirectory, entry.Name())

		description, ok := readPluginDescription(directory)
		if !ok {
			continue
		}
		if description.ID == "" || description.Name == "" {
			continue
		}

		metadata := description.Metadata(directory)
		if IsDotNet(description.Language) {
			unsupported = append(unsupported, description.Name+" ("+description.Language+")")
			continue
		}

		runner, err := RunnerFor(description.Language, metadata.ExecuteFilePath,
			pluginsSettings.PythonExecutablePath, pluginsSettings.NodeExecutablePath,
			pluginEnvironment(metadata))
		if err != nil {
			unsupported = append(unsupported, description.Name+" ("+description.Language+")")
			continue
		}

		pairs = append(pairs, PluginPair{
			Plugin:   NewJSONRPCPlugin(metadata, directory, runner),
			Metadata: metadata,
		})
	}

	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].Metadata.Name < pairs[j].Metadata.Name })
	sort.Strings(unsupported)
	return pairs, unsupported
}

// readPluginDescription reads what a plugin folder says about the plugin
// inside it.
func readPluginDescription(directory string) (PluginDescription, bool) {
	content, err := os.ReadFile(filepath.Join(directory, "plugin.json"))
	if err != nil {
		return PluginDescription{}, false
	}

	var description PluginDescription
	if err := json.Unmarshal(content, &description); err != nil {
		return PluginDescription{}, false
	}
	return description, true
}

// pluginEnvironment is what a plugin of another language is told about the
// launcher, which the reference's own plugins read.
func pluginEnvironment(metadata PluginMetadata) []string {
	programDirectory := filepath.Dir(metadata.PluginDirectory)
	return []string{
		"FLOW_VERSION=" + metadata.Version,
		"FLOW_PROGRAM_DIRECTORY=" + programDirectory,
		"FLOW_APPLICATION_DIRECTORY=" + programDirectory,
		"PYTHONDONTWRITEBYTECODE=1",
	}
}
