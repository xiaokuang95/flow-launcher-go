//go:build windows

package main

import (
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

var procSendInput = syscall.NewLazyDLL("user32.dll").NewProc("SendInput")
