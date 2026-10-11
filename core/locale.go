package core

import (
	"os"
	"strings"
)

// SystemLocale returns the language of the machine, as "en-US" or "zh-CN".
func SystemLocale() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(name); value != "" {
			return strings.Split(strings.Split(value, ".")[0], "@")[0]
		}
	}

	// The settings of the desktop, which is where the system's language is
	// kept on Windows and macOS.
	if locale := systemLocaleFromDesktop(); locale != "" {
		return locale
	}
	return DefaultLanguage
}

// userHomeDir is the user's home directory.
func userHomeDir() (string, error) { return os.UserHomeDir() }
