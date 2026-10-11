//go:build windows

package core

import (
	"errors"
	"path/filepath"
	"syscall"
	"unsafe"
)

// The machine's own context menu for a file or a folder is the shell's: a
// program asks the shell's folder object for the menu object of a path,
// lets it fill a menu with the items it would show, shows that menu at a
// place on the screen and runs what the user chose. It is what the
// reference's explorer does when the user presses Alt over a result, and
// what it offers as "Show the Windows context menu".
//
// The menu belongs to a window, and the shell's objects belong to the
// thread that made them, so this runs on the thread the launcher's window
// lives on, through the hook the launcher sets.

// The shell's own interfaces, by the identifiers the headers give them.
var (
	iidShellFolder = ParseCOMGuid("000214E6-0000-0000-C000-000000000046")
	iidContextMenu = ParseCOMGuid("000214E4-0000-0000-C000-000000000046")
)

// Where the methods this file calls sit in their objects' tables, counted
// from the three IUnknown methods every object starts with.
const (
	// IShellFolder: ParseDisplayName, EnumObjects, BindToObject,
	// BindToStorage, CompareIDs, CreateViewObject, GetAttributesOf,
	// GetUIObjectOf, GetDisplayNameOf, SetNameOf.
	vtblShellFolderParseDisplayName = 3
	vtblShellFolderBindToObject     = 5
	vtblShellFolderGetUIObjectOf    = 10
	// IContextMenu
	vtblContextMenuQueryContextMenu = 3
	vtblContextMenuInvokeCommand    = 4
)

// What the menu is asked for and shown with.
const (
	// cmfNormal is the menu a plain right-click shows.
	cmfNormal = 0x00000000
	// cmfExtendedVerbs is what Shift+right-click shows.
	cmfExtendedVerbs = 0x00000200
	// tpmReturnCmd answers with what was chosen rather than telling a
	// window of the caller's about it.
	tpmReturnCmd = 0x0100
	// shellMenuFirst and shellMenuLast are the identifiers the shell may
	// give its own items.
	shellMenuFirst = 1
	shellMenuLast  = 0x7FFF
	// shellShowNormal is how a program the menu starts is shown.
	shellShowNormal = 1
)

// ShellContextMenuPoint is where on the screen a menu is shown.
type ShellContextMenuPoint struct {
	X int32
	Y int32
}

// shellInvokeCommandInfo is what the shell reads about an item it is asked
// to run, as CMINVOKECOMMANDINFO describes it. The fields are pointers the
// shell reads as they are given, since a command is named by its offset in
// the menu rather than by a word.
type shellInvokeCommandInfo struct {
	size       uint32
	mask       uint32
	owner      uintptr
	verb       uintptr
	parameters uintptr
	directory  uintptr
	show       int32
	hotkey     uint32
	icon       uintptr
}

// ShowShellContextMenu shows the machine's own context menu for a path and
// runs what the user chose. It answers when the menu is closed, which is
// what makes it a call the user makes rather than one a search does.
//
// It has to be called on the thread the launcher's window lives on, which
// is what OnMainThread is for.
func ShowShellContextMenu(path string, point ShellContextMenuPoint, owner uintptr, extendedVerbs bool) error {
	if path == "" {
		return errors.New("there is no path to show a menu for")
	}
	return showShellContextMenu(path, point, owner, extendedVerbs)
}

// showShellContextMenu is the call itself.
func showShellContextMenu(path string, point ShellContextMenuPoint, owner uintptr, extendedVerbs bool) error {
	desktop, err := shellDesktopFolder()
	if err != nil {
		return err
	}
	defer ReleaseCOM(desktop)

	parent, child, err := shellParentAndChild(desktop, path)
	if err != nil {
		return err
	}
	defer ReleaseCOM(parent)
	defer freeCOMString(child)

	menuObject, err := shellContextMenuObject(parent, child)
	if err != nil {
		return err
	}
	defer ReleaseCOM(menuObject)

	menu, err := shellMenuFor(menuObject, extendedVerbs)
	if err != nil {
		return err
	}
	defer procDestroyMenu.Call(menu)

	// The menu is shown at the place the user is, and answers with what
	// was chosen.
	chosen, _, _ := procTrackPopupMenuEx.Call(
		menu,
		tpmReturnCmd,
		uintptr(point.X),
		uintptr(point.Y),
		owner,
		0,
	)
	if chosen == 0 {
		// The user closed the menu without choosing anything.
		return nil
	}

	return invokeShellMenuCommand(menuObject, uint(chosen))
}

// shellDesktopFolder is the shell's own object for the desktop, which is
// where every path is looked up from.
func shellDesktopFolder() (uintptr, error) {
	var desktop uintptr
	if result, _, _ := procSHGetDesktopFolder.Call(uintptr(unsafe.Pointer(&desktop))); failed(result) || desktop == 0 {
		return 0, errors.New("the shell's desktop folder could not be reached")
	}
	return desktop, nil
}

// shellParentAndChild is the shell's object for the folder a path is in,
// and the identifier of the file itself inside that folder, which is what
// the menu is asked for.
func shellParentAndChild(desktop uintptr, path string) (uintptr, uintptr, error) {
	directory := filepath.Dir(path)
	parentID, err := shellIdentifier(desktop, directory)
	if err != nil {
		return 0, 0, err
	}
	defer freeCOMString(parentID)

	var parent uintptr
	result, _, _ := syscall.SyscallN(
		COMMethod(desktop, vtblShellFolderBindToObject),
		desktop,
		parentID,
		0,
		uintptr(unsafe.Pointer(&iidShellFolder)),
		uintptr(unsafe.Pointer(&parent)),
	)
	if failed(result) || parent == 0 {
		return 0, 0, errors.New("the folder " + directory + " could not be reached")
	}

	child, err := shellIdentifier(parent, filepath.Base(path))
	if err != nil {
		ReleaseCOM(parent)
		return 0, 0, err
	}
	return parent, child, nil
}

// shellIdentifier is the shell's identifier for a path, as a folder reads
// it.
func shellIdentifier(folder uintptr, path string) (uintptr, error) {
	pathPointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var eaten uint32
	var attributes uint32
	var identifier uintptr
	result, _, _ := syscall.SyscallN(
		COMMethod(folder, vtblShellFolderParseDisplayName),
		folder,
		0,
		0,
		uintptr(unsafe.Pointer(pathPointer)),
		uintptr(unsafe.Pointer(&eaten)),
		uintptr(unsafe.Pointer(&identifier)),
		uintptr(unsafe.Pointer(&attributes)),
	)
	if failed(result) || identifier == 0 {
		return 0, errors.New("the shell does not know the path " + path)
	}
	return identifier, nil
}

// shellContextMenuObject is the shell's menu object for a file in a folder,
// which knows what may be done with it.
func shellContextMenuObject(parent, child uintptr) (uintptr, error) {
	children := []uintptr{child}

	var menu uintptr
	result, _, _ := syscall.SyscallN(
		COMMethod(parent, vtblShellFolderGetUIObjectOf),
		parent,
		0,
		1,
		uintptr(unsafe.Pointer(&children[0])),
		uintptr(unsafe.Pointer(&iidContextMenu)),
		0,
		uintptr(unsafe.Pointer(&menu)),
	)
	if failed(result) || menu == 0 {
		return 0, errors.New("the shell has no menu for that path")
	}
	return menu, nil
}

// shellMenuFor is a menu the shell has filled with what it would offer for
// the path.
func shellMenuFor(menuObject uintptr, extendedVerbs bool) (uintptr, error) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return 0, errors.New("a menu could not be made")
	}

	flags := uintptr(cmfNormal)
	if extendedVerbs {
		flags = cmfExtendedVerbs
	}

	result, _, _ := syscall.SyscallN(
		COMMethod(menuObject, vtblContextMenuQueryContextMenu),
		menuObject,
		menu,
		0,
		shellMenuFirst,
		shellMenuLast,
		flags,
	)
	if failed(result) {
		procDestroyMenu.Call(menu)
		return 0, errors.New("the shell would not fill the menu")
	}

	count, _, _ := procGetMenuItemCount.Call(menu)
	if count == 0 {
		procDestroyMenu.Call(menu)
		return 0, errors.New("the shell had nothing to offer for that path")
	}
	return menu, nil
}

// invokeShellMenuCommand runs the item the user chose. The shell knows it
// by the identifier it was given, which is passed as its offset in the
// menu rather than as a word.
func invokeShellMenuCommand(menuObject uintptr, command uint) error {
	if command < shellMenuFirst {
		return nil
	}

	info := shellInvokeCommandInfo{
		size: uint32(unsafe.Sizeof(shellInvokeCommandInfo{})),
		verb: uintptr(command - shellMenuFirst),
		show: shellShowNormal,
	}

	result, _, _ := syscall.SyscallN(
		COMMethod(menuObject, vtblContextMenuInvokeCommand),
		menuObject,
		uintptr(unsafe.Pointer(&info)),
	)
	if failed(result) {
		return errors.New("the shell would not run the item that was chosen")
	}
	return nil
}

// freeCOMString gives back memory the shell handed over.
func freeCOMString(pointer uintptr) {
	if pointer != 0 {
		procCoTaskMemFree.Call(pointer)
	}
}

// failed reports whether a call answered with a failure, which COM writes
// as a negative result.
func failed(result uintptr) bool {
	return int32(result) < 0
}

// The calls of the shell and of the desktop this file makes.
var (
	procSHGetDesktopFolder = shell32.NewProc("SHGetDesktopFolder")
	procCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	procDestroyMenu        = user32.NewProc("DestroyMenu")
	procGetMenuItemCount   = user32.NewProc("GetMenuItemCount")
	procTrackPopupMenuEx   = user32.NewProc("TrackPopupMenuEx")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
	procCoTaskMemFree      = ole32.NewProc("CoTaskMemFree")
)

// ShellContextMenuPointAtCursor is where the mouse is, which is where the
// reference shows the menu from a result.
func ShellContextMenuPointAtCursor() ShellContextMenuPoint {
	var point struct {
		X int32
		Y int32
	}
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	return ShellContextMenuPoint{X: point.X, Y: point.Y}
}

// ole32 is where COM's own calls live; shell32 and user32 are already
// loaded by the icon reader.
var ole32 = syscall.NewLazyDLL("ole32.dll")
