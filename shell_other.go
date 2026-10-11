//go:build !windows

package main

import (
	"os/exec"
	"strings"
)

// openWithShell opens a path or a URL with the desktop's own opener.
func openWithShell(target string) {
	if target == "" {
		return
	}
	command := "xdg-open"
	if strings.Contains(target, "darwin") || isDarwin() {
		command = "open"
	}
	_ = exec.Command(command, target).Start()
}

// runShell runs a command line in the shell, and waits for it.
func runShell(command, filename string) {
	if command == "" {
		return
	}
	shell := "/bin/sh"
	if filename != "" {
		shell = filename
	}
	_ = exec.Command(shell, "-c", command).Start()
}
