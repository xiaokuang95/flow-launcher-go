package pluginsmanager

import (
	"archive/zip"
	"encoding/json"
	"io"
	"strings"

	"launch/core"
)

// readPluginFile reads a plugin's own description out of the file it comes
// in, which is a zip holding a plugin.json beside the plugin itself.
func readPluginFile(path string) (core.UserPlugin, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return core.UserPlugin{}, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !strings.EqualFold(filepathBase(file.Name), "plugin.json") {
			continue
		}

		content, err := file.Open()
		if err != nil {
			return core.UserPlugin{}, err
		}
		body, err := io.ReadAll(content)
		content.Close()
		if err != nil {
			return core.UserPlugin{}, err
		}

		// A plugin's own description, in the way the reference writes it:
		// one action keyword or several, and the words the launcher shows.
		var described struct {
			ID             string   `json:"ID"`
			Name           string   `json:"Name"`
			Description    string   `json:"Description"`
			Author         string   `json:"Author"`
			Version        string   `json:"Version"`
			Language       string   `json:"Language"`
			Website        string   `json:"Website"`
			IcoPath        string   `json:"IcoPath"`
			ActionKeyword  string   `json:"ActionKeyword"`
			ActionKeywords []string `json:"ActionKeywords"`
		}
		if err := json.Unmarshal(body, &described); err != nil {
			return core.UserPlugin{}, err
		}
		if described.ID == "" || described.Name == "" {
			return core.UserPlugin{}, errorString("the plugin's description has no name or id")
		}

		return core.UserPlugin{
			ID:          described.ID,
			Name:        described.Name,
			Description: described.Description,
			Author:      described.Author,
			Version:     described.Version,
			Language:    described.Language,
			Website:     described.Website,
			IcoPath:     described.IcoPath,
		}, nil
	}

	return core.UserPlugin{}, errorString("the file holds no plugin.json")
}

// filepathBase is the last element of a path written with either
// separator, which a zip file's entries use.
func filepathBase(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	if index := strings.LastIndex(name, "/"); index >= 0 {
		return name[index+1:]
	}
	return name
}
