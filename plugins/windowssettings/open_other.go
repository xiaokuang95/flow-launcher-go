//go:build !windows

package windowssettings

import "errors"

// systemOpener opens a page of the Windows settings. Only Windows has
// them.
type systemOpener struct{}

// Open reports that the settings cannot be opened here.
func (systemOpener) Open(string) error {
	return errors.New("the Windows settings are only on Windows")
}
