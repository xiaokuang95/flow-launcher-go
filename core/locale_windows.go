//go:build windows

package core

import (
	"syscall"
	"unsafe"
)

// systemLocaleFromDesktop asks Windows for the user's language, which is
// what the interface follows when the setting is "System".
func systemLocaleFromDesktop() string {
	const localeNameMaxLength = 85

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getUserDefaultLocaleName := kernel32.NewProc("GetUserDefaultLocaleName")
	if err := getUserDefaultLocaleName.Find(); err != nil {
		return ""
	}

	buffer := make([]uint16, localeNameMaxLength)
	length, _, _ := getUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(localeNameMaxLength),
	)
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer)
}
