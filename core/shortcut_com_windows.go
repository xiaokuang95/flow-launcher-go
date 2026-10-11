//go:build windows

package core

import (
	"syscall"
	"unsafe"
)

// The shell's link object is a COM object. It is read through the methods
// the object's own table holds, in the order the interface is declared in.

// The shell's link object, and the two interfaces it is read through.
var (
	clsidShellLink = COMGuid{0x00021401, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidShellLinkW  = COMGuid{0x000214F9, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPersistFile = COMGuid{0x0000010B, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// The methods of IShellLinkW, in the order its vtable holds them after the
// three of IUnknown. The order is the one the interface is declared in,
// which is alphabetical rather than the order the documentation lists the
// methods in, so it is worth reading off the header rather than recalling.
const (
	vtblGetPath             = 3
	vtblGetDescription      = 6
	vtblGetWorkingDirectory = 8
	vtblGetArguments        = 10
	vtblGetShowCmd          = 14
	vtblResolve             = 19
)

// The methods of IPersistFile, after the three of IUnknown.
const (
	vtblLoad = 5
)

// COM's own flags.
const (
	stgmRead      = 0x0
	slrNoUI       = 0x1
	slgpShortPath = 0x1
	maxPath       = 260
)

// ReadShortcut reads what a Windows shortcut points at, the description it
// carries and the arguments it runs with. The path itself is returned when
// the link cannot be read, so that a shortcut that has lost its target
// still names something.
func ReadShortcut(path string) (target, description, arguments string) {
	if !isShortcut(path) {
		return path, "", ""
	}

	pathPointer := UTF16Pointer(path)
	if pathPointer == nil {
		return path, "", ""
	}

	OnCOMThread(func() {
		link := CreateCOMObject(&clsidShellLink, &iidShellLinkW, clsctxInprocServer)
		if link == 0 {
			return
		}
		defer ReleaseCOM(link)

		// Load the file, then let the shell resolve a relative target
		// without showing anything to the user.
		if persistFile := QueryCOMInterface(link, &iidPersistFile); persistFile != 0 {
			syscall.SyscallN(COMMethod(persistFile, vtblLoad), persistFile,
				uintptr(unsafe.Pointer(pathPointer)), stgmRead)
			ReleaseCOM(persistFile)
		}
		syscall.SyscallN(COMMethod(link, vtblResolve), link, 0, slrNoUI)

		buffer := make([]uint16, maxPath)
		count, _, _ := syscall.SyscallN(
			COMMethod(link, vtblGetPath),
			link,
			uintptr(unsafe.Pointer(&buffer[0])),
			maxPath,
			0,
			slgpShortPath,
		)
		if count == 0 {
			target = syscall.UTF16ToString(buffer)
		}

		descriptionBuffer := make([]uint16, maxPath)
		count, _, _ = syscall.SyscallN(
			COMMethod(link, vtblGetDescription),
			link,
			uintptr(unsafe.Pointer(&descriptionBuffer[0])),
			maxPath,
		)
		if count == 0 {
			description = syscall.UTF16ToString(descriptionBuffer)
		}

		argumentsBuffer := make([]uint16, maxPath)
		count, _, _ = syscall.SyscallN(
			COMMethod(link, vtblGetArguments),
			link,
			uintptr(unsafe.Pointer(&argumentsBuffer[0])),
			maxPath,
		)
		if count == 0 {
			arguments = syscall.UTF16ToString(argumentsBuffer)
		}
	})

	if target == "" {
		target = path
	}
	return target, description, arguments
}
