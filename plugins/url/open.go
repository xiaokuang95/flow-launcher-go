package url

import (
	"fmt"
	"os/exec"
	"strings"
)

// systemOpener opens URLs with the desktop's own browser.
type systemOpener struct{}

// Open opens a URL in the browser the system has.
func (systemOpener) Open(rawURL string) error {
	return openWithSystem(rawURL)
}

// OpenInBrowser opens a URL in a browser of the user's own, in a new window
// or tab, and privately or not.
func (systemOpener) OpenInBrowser(rawURL, browserPath string, newWindow, private bool, privateArgument string) error {
	if browserPath == "" {
		return openWithSystem(rawURL)
	}

	var arguments []string
	if private && privateArgument != "" {
		arguments = append(arguments, strings.Fields(privateArgument)...)
	}
	if newWindow {
		arguments = append(arguments, "--new-window")
	} else {
		arguments = append(arguments, "--new-tab")
	}
	arguments = append(arguments, rawURL)

	return exec.Command(browserPath, arguments...).Start()
}

// openWithSystem opens a URL the way the shell does, which is what the
// user's default browser comes from.
func openWithSystem(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("no URL to open")
	}
	// `start` is a command of cmd.exe rather than a program, so the shell
	// runs it with the URL quoted, which is what makes a URL with an
	// ampersand in it work.
	return exec.Command("cmd", "/c", "start", "", rawURL).Start()
}
