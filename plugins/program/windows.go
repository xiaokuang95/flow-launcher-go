//go:build windows

package program

import (
	"os/exec"
	"strings"
)

// appPathsPrograms are the programs the registry lists under App Paths,
// which is how a program is found by the name the user types in the Run
// box.
func appPathsPrograms() []string {
	keys := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`,
	}
	var programs []string
	for _, root := range []uintptr{hkeyLocalMachine, hkeyCurrentUser} {
		for _, key := range keys {
			programs = append(programs, registrySubKeyDefaults(root, key)...)
		}
	}
	return programs
}

// The registry roots and the access the defaults are read with.
const (
	hkeyCurrentUser  = 0x80000001
	hkeyLocalMachine = 0x80000002

	keyRead        = 0x20019
	keyWow64_32Key = 0x0200
	keyWow64_64Key = 0x0100
)

// registrySubKeyDefaults reads the default value of every subkey of a key,
// which is where App Paths keeps each program's own path.
func registrySubKeyDefaults(root uintptr, path string) []string {
	key, err := openRegistryKey(root, path)
	if err != nil {
		return nil
	}
	defer closeRegistryKey(key)

	var programs []string
	for _, name := range registrySubKeyNames(key) {
		subKey, err := openRegistrySubKey(key, name)
		if err != nil {
			continue
		}
		if value := registryDefaultValue(subKey); value != "" {
			programs = append(programs, value)
		}
		closeRegistryKey(subKey)
	}
	return programs
}

// fileDescription is what a program says it does, which is the description
// Windows shows beside it.
func fileDescription(path string) string {
	if path == "" {
		return ""
	}
	return versionInfoString(path, "FileDescription")
}

// startProgram runs a program, optionally as an administrator.
//
// The shell is asked to start it, which is what makes a document open in
// its own program and an elevation prompt appear: a program started
// directly cannot be elevated, and a shortcut to a document is not a
// program to run.
func startProgram(file, directory, arguments string, runAsAdmin bool) error {
	if file == "" {
		return nil
	}

	verb := ""
	if runAsAdmin {
		verb = "runas"
	}

	command := exec.Command("cmd.exe", "/c", "start", "", file)
	if arguments != "" {
		command = exec.Command("cmd.exe", "/c", "start", "", file, arguments)
	}
	command.Dir = directory
	if verb != "" {
		// Elevation is the shell's own business, so it is asked for
		// rather than done here.
		command = exec.Command("powershell.exe", "-NoProfile", "-Command",
			"Start-Process -FilePath "+quoteForPowerShell(file)+" -Verb RunAs")
	}
	return command.Start()
}

// startAsDifferentUser runs a program as another user, which the system
// asks a password for.
func startAsDifferentUser(file, directory, arguments string) error {
	if file == "" {
		return nil
	}
	command := exec.Command("powershell.exe", "-NoProfile", "-Command",
		"Start-Process -FilePath "+quoteForPowerShell(file)+" -Verb RunAsUser")
	command.Dir = directory
	return command.Start()
}

// quoteForPowerShell writes a path so that PowerShell reads it as one
// argument, whatever is in it.
func quoteForPowerShell(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}
