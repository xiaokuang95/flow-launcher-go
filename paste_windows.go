//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"syscall"
	"unsafe"
)

// pasteKeys presses Ctrl+V, which puts what is on the clipboard into
// whatever window is in front: choosing a clipboard item copies it and
// then pastes it where the user was.
func pasteKeys() error {
	inputs := []inputEvent{
		keyDown(vkControl),
		keyDown('V'),
		keyUpEvent('V'),
		keyUpEvent(vkControl),
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

// inputEvent is one event, as SendInput reads it: what kind of event it is,
// then the event itself, which is as large as the largest kind — a mouse's.
// The system checks the size it is given, so it has to be the whole of it.
type inputEvent struct {
	kind uint32
	_    uint32
	// body is the event: a keyboard's, which is a key, the scan code, the
	// flags, the time and the extra information.
	body [32]byte
}

// keyDown is a key being pressed, and keyUpEvent the same key let go.
func keyDown(key uint16) inputEvent { return keyEvent(key, 0) }
func keyUpEvent(key uint16) inputEvent {
	return keyEvent(key, keyUp)
}

// keyEvent is one key press or release, written where the system reads it:
// the key first, then the scan code, the flags, the time and the extra
// information.
func keyEvent(key uint16, flags uint32) inputEvent {
	event := inputEvent{kind: inputKeyboard}
	binary.LittleEndian.PutUint16(event.body[0:2], key)
	binary.LittleEndian.PutUint32(event.body[4:8], flags)
	return event
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
