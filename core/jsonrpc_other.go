//go:build !windows

package core

import "os/exec"

// hideWindow keeps a plugin that is a program of its own from flashing a
// window on the screen as it runs. Elsewhere a program of its own has no
// window unless it makes one.
func hideWindow(command *exec.Cmd) {}
