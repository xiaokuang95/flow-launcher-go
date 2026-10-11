//go:build windows

package sys

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"
)

// systemMachine does the commands to the machine.
type systemMachine struct{}

// The Win32 calls the power commands are made with.
var (
	user32Sys             = syscall.NewLazyDLL("user32.dll")
	powrprof              = syscall.NewLazyDLL("powrprof.dll")
	shell32Sys            = syscall.NewLazyDLL("shell32.dll")
	procLockWorkStation   = user32Sys.NewProc("LockWorkStation")
	procExitWindowsEx     = user32Sys.NewProc("ExitWindowsEx")
	procSetSuspendState   = powrprof.NewProc("SetSuspendState")
	procSHEmptyRecycleBin = shell32Sys.NewProc("SHEmptyRecycleBinW")
)

// The flags the machine is turned off with.
const (
	ewxLogoff      = 0x0
	ewxShutdown    = 0x1
	ewxReboot      = 0x2
	ewxPowerOff    = 0x8
	ewxBootOptions = 0x40000000

	// A generic, planned reason, as the reference gives.
	shutdownReason = 0x00040000 | 0x80000000
)

// ShutDown turns the machine off.
func (systemMachine) ShutDown() error {
	return exitWindows(ewxShutdown|ewxPowerOff, "/s /t 0")
}

// Restart starts it again.
func (systemMachine) Restart() error {
	return exitWindows(ewxReboot, "/r /t 0")
}

// RestartToFirmwareSettings starts it again at its own setup screen.
func (systemMachine) RestartToFirmwareSettings() error {
	return exitWindows(ewxReboot|ewxBootOptions, "/r /o /t 0")
}

// LogOff ends the session.
func (systemMachine) LogOff() error {
	result, _, _ := procExitWindowsEx.Call(ewxLogoff, shutdownReason)
	if result == 0 {
		return errors.New("the session could not be ended")
	}
	return nil
}

// Lock locks the screen.
func (systemMachine) Lock() error {
	result, _, _ := procLockWorkStation.Call()
	if result == 0 {
		return errors.New("the screen could not be locked")
	}
	return nil
}

// Sleep puts the machine to sleep.
func (systemMachine) Sleep() error { return suspend(false) }

// Hibernate writes the session to disk and turns the machine off.
func (systemMachine) Hibernate() error { return suspend(true) }

// OpenIndexingOptions shows what the machine is indexing.
func (systemMachine) OpenIndexingOptions() error {
	return exec.Command("control.exe", "srchadmin.dll").Start()
}

// EmptyRecycleBin throws away what is in the recycle bin.
func (systemMachine) EmptyRecycleBin() error {
	result, _, _ := procSHEmptyRecycleBin.Call(0, 0, 0)
	if result != 0 {
		return errors.New("the recycle bin could not be emptied")
	}
	return nil
}

// exitWindows asks the system to end the session, and falls back to the
// command that does the same thing when the system refuses.
func exitWindows(flags uintptr, fallback string) error {
	result, _, _ := procExitWindowsEx.Call(flags, shutdownReason)
	if result != 0 {
		return nil
	}

	arguments := []string{}
	for _, argument := range []string{fallback} {
		arguments = append(arguments, argument)
	}
	// The command line is given whole, as the reference gives it.
	return exec.Command("cmd.exe", "/c", "shutdown "+fallback).Start()
}

// suspend puts the machine to sleep, or hibernates it.
func suspend(hibernate bool) error {
	flag := uintptr(0)
	if hibernate {
		flag = 1
	}
	result, _, _ := procSetSuspendState.Call(flag, 0, 0)
	if result == 0 {
		return errors.New("the machine could not be suspended")
	}
	return nil
}

// unusedUnsafe keeps the import that the call signatures need.
var _ = unsafe.Pointer(nil)
