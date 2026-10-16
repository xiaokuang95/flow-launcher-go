//go:build !windows

package main

import "errors"

// pasteKeys presses the paste key, which only Windows does here.
func pasteKeys() error {
	return errors.New("pasting into the window underneath is only supported on Windows")
}
