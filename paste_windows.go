//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"
)

// pasteKeys presses Ctrl+V, which puts what is on the clipboard into
// whatever window is in front: choosing a clipboard item copies it and
// then pastes it where the user was.
func pasteKeys() error {
	inputs := []keyboardInput{
		{virtualKey: vkControl},
		{virtualKey: 'V'},
		{virtualKey: 'V', flags: keyUp},
		{virtualKey: vkControl, flags: keyUp},
	}

	sent, _, err := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if sent != uintptr(len(inputs)) {
		return err
	}
	return nil
}

// foregroundWindow is the window the user is working in, which is what a
// paste goes to.
func foregroundWindow() uintptr {
	window, _, _ := procGetForegroundWindow.Call()
	return window
}

// activateWindow puts a window in front of the others, so that what is
// typed next reaches it. A window may refuse the focus, and a window that
// belongs to another program refuses it more often when the program that
// asks is not the one in front: the keyboard is handed over for the moment
// it takes.
func activateWindow(window uintptr) error {
	if window == 0 {
		return errors.New("there is no window to paste into")
	}

	if result, _, _ := procSetForegroundWindow.Call(window); result != 0 {
		return nil
	}

	// The focus follows the input: while this program holds it, the window
	// it means to hand it to is put in front, and then the focus is given
	// back.
	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground == 0 {
		return errors.New("the window to paste into would not come forward")
	}
	thisThread, _, _ := procGetCurrentThreadId.Call()
	otherThread, _, _ := procGetWindowThreadProcessId.Call(foreground, 0)

	procAttachThreadInput.Call(thisThread, otherThread, 1)
	result, _, _ := procSetForegroundWindow.Call(window)
	procAttachThreadInput.Call(thisThread, otherThread, 0)

	if result == 0 {
		return errors.New("the window to paste into would not come forward")
	}
	return nil
}

// keyboardInput is one key press or release, as SendInput reads it: the
// kind, the key, the flags, and the padding the system expects.
type keyboardInput struct {
	kind       uint32
	_          uint32
	virtualKey uint16
	scanCode   uint16
	flags      uint32
	time       uint32
	extra      uintptr
}

const (
	// inputKeyboard is the kind of event a key is.
	inputKeyboard = 1
	// keyUp is set on a key being let go.
	keyUp = 0x0002
	// vkControl is the Ctrl key.
	vkControl = 0x11
)

// user32 and kernel32 are the system's own libraries.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
)

var (
	procSendInput                = user32.NewProc("SendInput")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
)
