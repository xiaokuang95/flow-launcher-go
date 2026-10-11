//go:build windows

package core

import (
	"os"
	"path/filepath"
	"strings"
)

// shortcutTarget reads where a Windows shortcut points.
//
// The launcher reads it with the shell's own link object, so that a link
// with a relative target, an environment variable in its path or a special
// folder resolves the way the shell resolves it. Reading the file's bytes
// instead would answer for the common case and be wrong for the rest.
func shortcutTarget(path string) string {
	target, _, _ := ReadShortcut(path)
	if target == "" {
		return path
	}
	return target
}

// ShortcutDescription is the description a shortcut carries, which the
// program plugin shows as the result's subtitle.
func ShortcutDescription(path string) string {
	_, description, _ := ReadShortcut(path)
	return description
}

// ShortcutArguments are the arguments a shortcut runs its target with.
func ShortcutArguments(path string) string {
	_, _, arguments := ReadShortcut(path)
	return arguments
}

// isShortcut reports whether a path is a Windows shortcut.
func isShortcut(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".lnk")
}

// StartMenuDirectories are the directories Windows keeps its shortcuts in.
func StartMenuDirectories() []string {
	appData := os.Getenv("APPDATA")
	programData := os.Getenv("ProgramData")

	var directories []string
	if appData != "" {
		directories = append(directories,
			filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"),
		)
	}
	if programData != "" {
		directories = append(directories,
			filepath.Join(programData, "Microsoft", "Windows", "Start Menu", "Programs"),
		)
	}
	return directories
}
