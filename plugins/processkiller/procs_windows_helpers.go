//go:build windows

package processkiller

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Win32 pieces the window titles are read with.
var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowText            = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

// unsafeSizeofProcessEntry32 is the size of the process entry the snapshot
// hands back, which has to be set before it is used.
func unsafeSizeofProcessEntry32() int {
	var entry windows.ProcessEntry32
	return int(unsafe.Sizeof(entry))
}

// unsafePointer turns a pointer to a value into the uintptr the Win32 calls
// take.
func unsafePointer[T any](value *T) unsafe.Pointer {
	return unsafe.Pointer(value)
}
