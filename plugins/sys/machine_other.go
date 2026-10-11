//go:build !windows

package sys

import "errors"

// systemMachine does the commands to the machine. Only Windows is
// supported: elsewhere the commands report that they cannot be done rather
// than doing something else.
type systemMachine struct{}

func (systemMachine) ShutDown() error                  { return errUnsupported }
func (systemMachine) Restart() error                   { return errUnsupported }
func (systemMachine) RestartToFirmwareSettings() error { return errUnsupported }
func (systemMachine) LogOff() error                    { return errUnsupported }
func (systemMachine) Lock() error                      { return errUnsupported }
func (systemMachine) Sleep() error                     { return errUnsupported }
func (systemMachine) Hibernate() error                 { return errUnsupported }
func (systemMachine) OpenIndexingOptions() error       { return errUnsupported }
func (systemMachine) EmptyRecycleBin() error           { return errUnsupported }

// errUnsupported is what a command answers away from Windows.
var errUnsupported = errors.New("this command is only supported on Windows")
