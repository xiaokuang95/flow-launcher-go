package core

import (
	"io/fs"
	"os"
	"path/filepath"
)

// A plugin the launcher was built with is not a folder on disk: it is a Go
// package inside the launcher, and the pictures it names ("Images/file.png")
// are read from its own directory, which is where the launcher puts what it
// is given. A plugin that ships pictures therefore writes them there when it
// is initialized, which is what the reference's plugins have already, being
// folders of their own.

// InstallImages writes the pictures a plugin ships where the launcher reads
// them from. A picture that is already there is left alone, so that a
// launcher that was started before does not write them again.
func InstallImages(directory string, files fs.FS, pattern string) error {
	if directory == "" {
		return nil
	}

	return fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !matchesPattern(path, pattern) {
			return nil
		}

		target := filepath.Join(directory, filepath.FromSlash(path))
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		content, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

// matchesPattern reports whether a path is one of the pictures named, which
// is a pattern such as "Images/*.png".
func matchesPattern(path, pattern string) bool {
	matched, err := filepath.Match(pattern, filepath.ToSlash(path))
	return err == nil && matched
}
