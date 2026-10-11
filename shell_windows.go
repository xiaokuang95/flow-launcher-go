//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// openWithShell opens a path or a URL the way the shell does, which is what
// the user's default browser and file manager come from.
func openWithShell(target string) {
	if target == "" {
		return
	}
	// `start` is a command of cmd.exe rather than a program, so the shell
	// runs it with the path quoted, which is what makes a path with spaces
	// work.
	cmd := exec.Command("cmd", "/c", "start", "", target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

// runShell runs a command line in the shell, and waits for it.
func runShell(command, filename string) {
	if command == "" {
		return
	}
	shell := "cmd.exe"
	if filename != "" {
		shell = filename
	}
	cmd := exec.Command(shell, "/c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}
