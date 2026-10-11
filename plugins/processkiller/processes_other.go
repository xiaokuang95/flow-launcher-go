//go:build !windows

package processkiller

import "errors"

// systemProcesses reads the machine's processes and ends them. Only Windows
// is supported: elsewhere the plugin lists nothing rather than guessing at
// what a process is.
type systemProcesses struct{}

// Running returns no processes.
func (systemProcesses) Running() ([]Process, error) { return nil, nil }

// Kill ends a process.
func (systemProcesses) Kill(Process) error {
	return errors.New("ending processes is only supported on Windows")
}
