//go:build windows

package program

import (
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// openRegistryKey opens a key of the registry, either by name under
// another key or by one of the root handles.
func openRegistryKey(parent uintptr, path string) (registry.Key, error) {
	switch parent {
	case hkeyLocalMachine:
		return registry.OpenKey(registry.LOCAL_MACHINE, path, registry.READ|registry.WOW64_64KEY)
	case hkeyCurrentUser:
		return registry.OpenKey(registry.CURRENT_USER, path, registry.READ|registry.WOW64_64KEY)
	default:
		return registry.OpenKey(registry.Key(parent), path, registry.READ)
	}
}

// openRegistrySubKey opens a key by name under another key.
func openRegistrySubKey(parent registry.Key, name string) (registry.Key, error) {
	return registry.OpenKey(parent, name, registry.READ)
}

// closeRegistryKey gives a key back.
func closeRegistryKey(key registry.Key) { _ = key.Close() }

// registrySubKeyNames are the names of the subkeys of a key.
func registrySubKeyNames(key registry.Key) []string {
	names, err := key.ReadSubKeyNames(0)
	if err != nil {
		return nil
	}
	return names
}

// registryDefaultValue is the unnamed value of a key, which is where App
// Paths keeps a program's own path.
func registryDefaultValue(key registry.Key) string {
	value, _, err := key.GetStringValue("")
	if err != nil {
		return ""
	}
	return value
}

// versionInfoString reads one of the strings a program carries about
// itself, such as the description Windows shows beside it.
//
// The version information is a resource the program was built with: a
// header, then a list of languages and code pages, each with its own block
// of strings. Only the first block is read, which is the one the system
// itself shows.
func versionInfoString(path, name string) string {
	size, _, _ := procGetFileVersionInfoSize.Call(uintptr(unsafe.Pointer(utf16Pointer(path))), 0)
	if size == 0 {
		return ""
	}

	buffer := make([]byte, size)
	ok, _, _ := procGetFileVersionInfo.Call(
		uintptr(unsafe.Pointer(utf16Pointer(path))), 0, size,
		uintptr(unsafe.Pointer(&buffer[0])))
	if ok == 0 {
		return ""
	}

	// A translation is a language and a code page, four bytes each.
	var translations uintptr
	var translationLength uint32
	ok, _, _ = procVerQueryValue.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(utf16Pointer(`\VarFileInfo\Translation`))),
		uintptr(unsafe.Pointer(&translations)),
		uintptr(unsafe.Pointer(&translationLength)))
	if ok == 0 || translationLength < 4 {
		return ""
	}

	// The pointer is into the buffer that was read, so it is read as an
	// offset into it rather than as an address of its own.
	translation := unsafe.Add(unsafe.Pointer(&buffer[0]), translations-uintptr(unsafe.Pointer(&buffer[0])))
	language := *(*uint16)(translation)
	codePage := *(*uint16)(unsafe.Add(translation, 2))
	query := `\StringFileInfo\` + hex4(language) + hex4(codePage) + `\` + name

	var value uintptr
	var valueLength uint32
	ok, _, _ = procVerQueryValue.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(utf16Pointer(query))),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&valueLength)))
	if ok == 0 || valueLength == 0 {
		return ""
	}

	text := unsafe.Add(unsafe.Pointer(&buffer[0]), value-uintptr(unsafe.Pointer(&buffer[0])))
	return utf16String(text, valueLength)
}
