//go:build !windows

package core

import "errors"

// ShellContextMenuPoint is where on the screen a menu is shown.
type ShellContextMenuPoint struct {
	X int32
	Y int32
}

// ShellContextMenuPointAtCursor is where the mouse is.
func ShellContextMenuPointAtCursor() ShellContextMenuPoint { return ShellContextMenuPoint{} }

// ShowShellContextMenu shows the machine's own context menu for a path,
// which only Windows has.
func ShowShellContextMenu(path string, point ShellContextMenuPoint, owner uintptr, extendedVerbs bool) error {
	return errors.New("the machine's own context menu is only there on Windows")
}
