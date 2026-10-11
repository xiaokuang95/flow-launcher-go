//go:build windows

package windowssettings

import (
	"os/exec"
	"syscall"
)

// systemOpener opens a page of the Windows settings.
type systemOpener struct{}

// Open opens what a settings page's command names, which is a URI of the
// settings app itself.
func (systemOpener) Open(command string) error {
	if command == "" {
		return nil
	}
	// The shell is asked to open it, which is what makes a URI of the
	// settings app reach the settings app.
	program := exec.Command("cmd.exe", "/c", "start", "", command)
	program.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return program.Start()
}
