//go:build !windows

package core

import "path/filepath"

// IconForFile is the icon of a file. Only Windows reads the shell's icons,
// so elsewhere a result shows the picture when the file is one.
func IconForFile(path string) []byte { return nil }

// ShortcutTarget reads the path a shortcut points at. Only Windows has
// them.
func ShortcutTarget(path string) string { return path }

// isShortcut reports whether a path is a shortcut of the platform.
func isShortcut(path string) bool {
	extension := filepath.Ext(path)
	return extension == ".desktop"
}
