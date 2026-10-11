//go:build !windows

package program

import "errors"

// appPathsPrograms are the programs the registry lists. Only Windows has
// that registry.
func appPathsPrograms() []string { return nil }

// fileDescription is what a program says it does. Only Windows keeps that
// beside the program.
func fileDescription(string) string { return "" }

// startProgram runs a program.
func startProgram(file, directory, arguments string, runAsAdmin bool) error {
	return errors.New("running programs is only supported on Windows")
}

// startAsDifferentUser runs a program as another user.
func startAsDifferentUser(file, directory, arguments string) error {
	return errors.New("running programs as another user is only supported on Windows")
}
