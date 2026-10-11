package core

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// unzipArchive unpacks a zip file into a directory.
//
// A zip entry may name a path of its own, so each is kept inside the
// destination: a file that tries to write outside it is refused rather
// than allowed to put a file anywhere on the machine.
func unzipArchive(archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, entry := range reader.File {
		target, err := insideDestination(destination, entry.Name)
		if err != nil {
			return err
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		content, err := entry.Open()
		if err != nil {
			return err
		}
		file, err := os.Create(target)
		if err != nil {
			content.Close()
			return err
		}
		_, err = io.Copy(file, content)
		content.Close()
		file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// insideDestination is where an entry goes, or an error when it names a
// path outside the destination.
func insideDestination(destination, name string) (string, error) {
	cleaned := filepath.Clean(strings.ReplaceAll(name, `\`, "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")

	target := filepath.Join(destination, cleaned)
	root := filepath.Clean(destination)

	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", fmt.Errorf("the file %q would be written outside the plugin's folder", name)
	}
	return target, nil
}
