//go:build !windows

package main

import "errors"

// pasteKeys presses the paste key, which only Windows does here.
func pasteKeys() error {
	return errors.New("pasting into the window underneath is only supported on Windows")
}

// foregroundWindow is the window the user is working in.
func foregroundWindow() uintptr { return 0 }

// activateWindow puts a window in front of the others.
func activateWindow(uintptr) error {
	return errors.New("bringing a window forward is only supported on Windows")
}
