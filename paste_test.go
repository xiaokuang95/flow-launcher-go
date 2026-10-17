package main

import (
	"sync"
	"testing"
	"time"
)

// The paste is a platform action, so a test watches what was asked for:
// which window was brought forward, and whether the key was pressed at all.

// pasteProbe is what the launcher asked the machine to do. The paste runs
// on a goroutine of its own, so what it did is guarded.
type pasteProbe struct {
	mu        sync.Mutex
	activated []uintptr
	pressed   int
	front     uintptr
}

func (p *pasteProbe) activate(window uintptr) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activated = append(p.activated, window)
	p.front = window
}

func (p *pasteProbe) press() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pressed++
}

// state is what the probe holds now.
func (p *pasteProbe) state() (activated []uintptr, pressed int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]uintptr(nil), p.activated...), p.pressed
}

// watchPaste stands in for the machine while a test runs.
func watchPaste(t *testing.T, front uintptr) *pasteProbe {
	t.Helper()

	probe := &pasteProbe{front: front}
	wasForeground, wasActivate, wasPaste := foregroundWindowOf, activateWindowOf, pasteKeysOf

	foregroundWindowOf = func() uintptr {
		probe.mu.Lock()
		defer probe.mu.Unlock()
		return probe.front
	}
	activateWindowOf = func(window uintptr) error {
		probe.activate(window)
		return nil
	}
	pasteKeysOf = func() error {
		probe.press()
		return nil
	}

	t.Cleanup(func() {
		foregroundWindowOf, activateWindowOf, pasteKeysOf = wasForeground, wasActivate, wasPaste
	})
	return probe
}

// pasteIsDone waits for the goroutine a paste runs on.
func pasteIsDone(t *testing.T, probe *pasteProbe) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if activated, pressed := probe.state(); pressed > 0 || len(activated) > 0 {
			// The launcher waits a moment after bringing the window
			// forward, then presses the key.
			time.Sleep(200 * time.Millisecond)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the paste never happened")
}

func TestPastingGoesToTheWindowTheUserCameFrom(t *testing.T) {
	const userWindow = 0x1234
	probe := watchPaste(t, userWindow)

	a := &app{}
	a.cameFrom = userWindow
	a.pasteIntoWindow()
	pasteIsDone(t, probe)

	activated, pressed := probe.state()
	if len(activated) != 1 || activated[0] != userWindow {
		t.Errorf("the window brought forward is %v, want the one the user came from", activated)
	}
	if pressed != 1 {
		t.Errorf("the paste key was pressed %d times, want once", pressed)
	}
}

func TestAPasteWithNowhereToGoIsNotPressed(t *testing.T) {
	// The launcher has not been shown, so there is no window it came from.
	probe := watchPaste(t, 0)

	a := &app{}
	a.pasteIntoWindow()

	// Give the goroutine the time it would have taken.
	time.Sleep(300 * time.Millisecond)

	if activated, pressed := probe.state(); len(activated) != 0 || pressed != 0 {
		t.Errorf("a paste was made with nowhere to paste: %v, %d presses", activated, pressed)
	}
}

func TestTheLauncherDoesNotPasteIntoItsOwnWindow(t *testing.T) {
	// The desktop leaves the focus on the launcher, which is what made the
	// search box look like the clipboard.
	const launcherWindow = 0x5678
	probe := watchPaste(t, launcherWindow)

	a := &app{cameFrom: launcherWindow, windowOf: func() uintptr { return launcherWindow }}
	// The desktop left the focus on the launcher, which is what it does
	// when a window is hidden: nothing of the user's is in front.
	a.pasteIntoWindow()
	time.Sleep(300 * time.Millisecond)

	if _, pressed := probe.state(); pressed != 0 {
		t.Errorf("the paste key was pressed %d times into the launcher's own window, want none", pressed)
	}
}
