//go:build windows

package core

import (
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// COM is how Windows hands a program its own objects: the shell's link
// object, the search index. A program asks for one by its class, and reads
// it through the methods the object's table holds.
//
// COM has to be started on the thread that uses it, and Go moves a
// goroutine between threads, so the launcher keeps one thread of its own
// for COM and runs every call there.

// The kinds of server a COM object is made by.
const (
	clsctxInprocServer  = 0x1
	clsctxLocalServer   = 0x4
	clsctxInprocHandler = 0x2
	clsctxRemoteServer  = 0x10
	clsctxAll           = clsctxInprocServer | clsctxInprocHandler | clsctxLocalServer | clsctxRemoteServer
)

// COMGuid is a class or interface identifier.
type COMGuid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// ParseCOMGuid reads an identifier written the way the documentation and
// the headers write one, as 00021401-0000-0000-C000-000000000046, so that a
// call site can say which interface it wants in the same words the header
// does.
func ParseCOMGuid(text string) COMGuid {
	var guid COMGuid
	parts := strings.Split(text, "-")
	if len(parts) != 5 {
		return guid
	}

	first, _ := strconv.ParseUint(parts[0], 16, 32)
	second, _ := strconv.ParseUint(parts[1], 16, 16)
	third, _ := strconv.ParseUint(parts[2], 16, 16)
	guid.Data1 = uint32(first)
	guid.Data2 = uint16(second)
	guid.Data3 = uint16(third)

	rest := parts[3] + parts[4]
	for i := 0; i < len(rest) && i/2 < len(guid.Data4); i += 2 {
		value, err := strconv.ParseUint(rest[i:i+2], 16, 8)
		if err != nil {
			return COMGuid{}
		}
		guid.Data4[i/2] = byte(value)
	}
	return guid
}

var comCalls = make(chan func(), 8)

func init() {
	ole32 := syscall.NewLazyDLL("ole32.dll")
	coInitializeEx := ole32.NewProc("CoInitializeEx")

	go func() {
		runtime.LockOSThread()
		// COINIT_APARTMENTTHREADED: the shell's objects are apartments.
		coInitializeEx.Call(0, 2)
		for call := range comCalls {
			call()
		}
	}()
}

// OnCOMThread runs a function on the thread COM was started on, and waits
// for it.
func OnCOMThread(call func()) {
	done := make(chan struct{})
	comCalls <- func() {
		defer close(done)
		call()
	}
	<-done
}

// COMMethod is the address of a method of a COM object: the object's table
// of methods, and the place in it the method holds.
//
// The address is one the system handed back, not one Go made, so turning
// it back into a pointer is safe here. `go vet` cannot tell the two apart
// and reports this as a possible misuse; the address never points into
// memory Go moves.
func COMMethod(object uintptr, index int) uintptr {
	table := *(*uintptr)(unsafe.Pointer(object)) //nolint:govet // an address the system handed back
	return *(*uintptr)(unsafe.Pointer(table + uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

// ReleaseCOM gives back a COM object.
func ReleaseCOM(object uintptr) {
	if object == 0 {
		return
	}
	syscall.SyscallN(COMMethod(object, 2), object)
}

// QueryCOMInterface asks an object for one of the interfaces it offers.
func QueryCOMInterface(object uintptr, iid *COMGuid) uintptr {
	wanted, _ := QueryCOMInterfaceResult(object, iid)
	return wanted
}

// QueryCOMInterfaceResult asks an object for one of the interfaces it
// offers, and says what the object answered: a program that cannot reach an
// object needs to know whether it was refused or failed.
func QueryCOMInterfaceResult(object uintptr, iid *COMGuid) (uintptr, uint32) {
	var wanted uintptr
	result, _, _ := syscall.SyscallN(
		COMMethod(object, 0),
		object,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&wanted)))
	if result != 0 {
		return 0, uint32(result)
	}
	return wanted, 0
}

// CreateCOMObject makes an object of a class the system knows.
func CreateCOMObject(class, iid *COMGuid, context uintptr) uintptr {
	ole32 := syscall.NewLazyDLL("ole32.dll")
	coCreateInstance := ole32.NewProc("CoCreateInstance")

	var object uintptr
	result, _, _ := coCreateInstance.Call(
		uintptr(unsafe.Pointer(class)),
		0,
		context,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&object)))
	if result != 0 {
		return 0
	}
	return object
}

// UTF16Pointer is a pointer to a UTF-16 string, for the calls that take
// one.
func UTF16Pointer(value string) *uint16 {
	pointer, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return nil
	}
	return pointer
}

// UTF16FromPointer reads a UTF-16 string that ends at its null, from an
// address into memory that is alive while it is read.
func UTF16FromPointer(address uintptr, limit int) string {
	if address == 0 {
		return ""
	}
	var units []uint16
	for i := 0; i < limit; i++ {
		unit := *(*uint16)(unsafe.Pointer(address + uintptr(i)*2)) //nolint:govet // an address the system handed back
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return syscall.UTF16ToString(units)
}

// UTF16FromPointerOfLength reads a UTF-16 string of a length in characters.
func UTF16FromPointerOfLength(address uintptr, length int) string {
	if address == 0 || length <= 0 {
		return ""
	}
	units := unsafe.Slice((*uint16)(unsafe.Pointer(address)), length) //nolint:govet // an address the system handed back
	return syscall.UTF16ToString(units)
}
