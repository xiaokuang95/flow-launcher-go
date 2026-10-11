//go:build windows

package program

import (
	"strings"
	"syscall"
	"unsafe"
)

// The version information of a program is read with the library Windows
// reads it with.
var (
	version32                  = syscall.NewLazyDLL("version.dll")
	procGetFileVersionInfoSize = version32.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfo     = version32.NewProc("GetFileVersionInfoW")
	procVerQueryValue          = version32.NewProc("VerQueryValueW")
)

// utf16Pointer is a pointer to a UTF-16 string for the Win32 calls.
func utf16Pointer(value string) *uint16 {
	pointer, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return nil
	}
	return pointer
}

// utf16String reads a UTF-16 string of a length in characters, from a
// pointer into a buffer that is alive while it is read.
func utf16String(pointer unsafe.Pointer, length uint32) string {
	if pointer == nil || length == 0 {
		return ""
	}
	units := unsafe.Slice((*uint16)(pointer), length)
	return strings.TrimRight(syscall.UTF16ToString(units), "\x00")
}

// hex4 writes a number of four hexadecimal digits, which is how the version
// information names a language and a code page.
func hex4(value uint16) string {
	const digits = "0123456789abcdef"
	out := []byte{'0', '0', '0', '0'}
	for i := 3; i >= 0; i-- {
		out[i] = digits[value&0xf]
		value >>= 4
	}
	return string(out)
}
