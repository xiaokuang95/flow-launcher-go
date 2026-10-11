package pluginsmanager

import (
	"archive/zip"
	"os"
	"path/filepath"
)

// writePluginFile writes a plugin's file, which is what the store hands
// over and what the launcher reads a plugin out of. It is used by the
// tests, which need a file to read rather than a network to wait for.
func writePluginFile(path, name, id string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	entry, err := writer.Create("plugin.json")
	if err != nil {
		return err
	}
	described := `{"ID":"` + id + `","Name":"` + name + `","Version":"1.0","Author":"Someone","ActionKeyword":"*"}`
	if _, err := entry.Write([]byte(described)); err != nil {
		return err
	}
	return writer.Close()
}
