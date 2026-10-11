package core

import (
	"archive/zip"
	"os"
)

// writePluginFile writes a plugin's file, which is what the store hands
// over: a zip holding the plugin and its description.
func writePluginFile(path, name, id string) error {
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
	described := `{"ID":"` + id + `","Name":"` + name + `","Language":"python"}`
	if _, err := entry.Write([]byte(described)); err != nil {
		return err
	}
	return writer.Close()
}

// writeZipWithEntry writes a zip holding one file under the name given.
func writeZipWithEntry(path, name, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	entry, err := writer.Create(name)
	if err != nil {
		return err
	}
	if _, err := entry.Write([]byte(content)); err != nil {
		return err
	}
	return writer.Close()
}
