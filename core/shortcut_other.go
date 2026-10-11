//go:build !windows

package core

import "path/filepath"

// ReadShortcut reads what a shortcut points at. Only Windows has them, so
// elsewhere a shortcut is a file like any other.
func ReadShortcut(path string) (target, description, arguments string) {
	return path, "", ""
}

// ShortcutDescription is the description a shortcut carries. Only Windows
// has shortcuts.
func ShortcutDescription(path string) string { return "" }

// ShortcutArguments are the arguments a shortcut runs its target with.
func ShortcutArguments(path string) string { return "" }

// StartMenuDirectories are the directories the desktop keeps its
// applications in.
func StartMenuDirectories() []string {
	directories := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
	}
	if home, err := homeDirectory(); err == nil {
		directories = append([]string{filepath.Join(home, ".local", "share", "applications")}, directories...)
	}
	return directories
}

func homeDirectory() (string, error) {
	return userHomeDir()
}
