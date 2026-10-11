//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps a plugin that is a program of its own from flashing a
// window on the screen as it runs.
func hideWindow(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
