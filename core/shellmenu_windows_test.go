//go:build windows

package core

import (
	"os"
	"path/filepath"
	"testing"
)

// The shell's own menu is asked of the shell, which is the only thing that
// can say whether the calls in shellmenu_windows.go are the right ones: the
// menu is filled, and what it holds is what the shell would offer. Showing
// it is left out here, since a menu waits for the user.

func TestTheShellFillsAMenuForAFile(t *testing.T) {
	path := writeTestFile(t, "report.txt")

	menu, release := shellMenuForPath(t, path)
	defer release()

	count, _, _ := procGetMenuItemCount.Call(menu)
	if count == 0 {
		t.Fatalf("the shell filled the menu with nothing")
	}
	t.Logf("the shell offered %d items for a text file", count)
}

func TestTheShellFillsAMenuForAFolder(t *testing.T) {
	directory := t.TempDir()

	menu, release := shellMenuForPath(t, directory)
	defer release()

	count, _, _ := procGetMenuItemCount.Call(menu)
	if count == 0 {
		t.Fatalf("the shell filled the menu with nothing")
	}
	t.Logf("the shell offered %d items for a folder", count)
}

func TestTheShellFillsAMenuWithTheExtendedItems(t *testing.T) {
	path := writeTestFile(t, "report.txt")

	plain, releasePlain := shellMenuForPath(t, path)
	defer releasePlain()
	extended, releaseExtended := shellMenuForPathExtended(t, path)
	defer releaseExtended()

	plainCount, _, _ := procGetMenuItemCount.Call(plain)
	extendedCount, _, _ := procGetMenuItemCount.Call(extended)
	if extendedCount < plainCount {
		t.Errorf("the extended menu has %d items, want at least the plain %d", extendedCount, plainCount)
	}
}

func TestAPathTheShellDoesNotKnowHasNoMenu(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-there.txt")

	err := ShowShellContextMenu(missing, ShellContextMenuPoint{}, 0, false)
	if err == nil {
		t.Errorf("a path that is not there was given a menu")
	}
}

// writeTestFile is a file for the shell to offer its menu for.
func writeTestFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// shellMenuForPath is the menu the shell fills for a path, and what gives
// the objects it was made from back.
func shellMenuForPath(t *testing.T, path string) (uintptr, func()) {
	t.Helper()
	return shellMenuForPathWith(t, path, false)
}

// shellMenuForPathExtended is the same menu with the items Shift+right-click
// shows.
func shellMenuForPathExtended(t *testing.T, path string) (uintptr, func()) {
	t.Helper()
	return shellMenuForPathWith(t, path, true)
}

func shellMenuForPathWith(t *testing.T, path string, extended bool) (uintptr, func()) {
	t.Helper()

	desktop, err := shellDesktopFolder()
	if err != nil {
		t.Fatal(err)
	}
	parent, child, err := shellParentAndChild(desktop, path)
	if err != nil {
		ReleaseCOM(desktop)
		t.Fatal(err)
	}
	menuObject, err := shellContextMenuObject(parent, child)
	if err != nil {
		ReleaseCOM(parent)
		ReleaseCOM(desktop)
		t.Fatal(err)
	}
	menu, err := shellMenuFor(menuObject, extended)
	if err != nil {
		ReleaseCOM(menuObject)
		ReleaseCOM(parent)
		ReleaseCOM(desktop)
		t.Fatal(err)
	}

	return menu, func() {
		procDestroyMenu.Call(menu)
		ReleaseCOM(menuObject)
		ReleaseCOM(parent)
		freeCOMString(child)
		ReleaseCOM(desktop)
	}
}
