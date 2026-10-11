//go:build windows

package explorer

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

// readableSize writes a number of bytes the way a person reads it.
func readableSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}

	value := float64(bytes)
	for _, name := range []string{"KB", "MB", "GB", "TB", "PB"} {
		value /= unit
		if value < unit {
			return strconv.FormatFloat(value, 'f', 2, 64) + " " + name
		}
	}
	return strconv.FormatFloat(value, 'f', 2, 64) + " EB"
}

// driveSpace is how much room a drive has, and how much it holds.
func driveSpace(path string) (free, total int64, ok bool) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, false
	}

	var freeToCaller, totalBytes, totalFree uint64
	result, _, _ := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pointer)),
		uintptr(unsafe.Pointer(&freeToCaller)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFree)))
	if result == 0 {
		return 0, 0, false
	}
	return int64(totalFree), int64(totalBytes), true
}

// creationTime is when a file was made, which only Windows records.
func creationTime(info os.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, stat.CreationTime.Nanoseconds())
	}
	return info.ModTime()
}

// openFile opens a file in the program it belongs to.
func openFile(path string) { openWithShell(path) }

// openInEditor opens a file in the editor the user chose.
func (p *Plugin) openInEditor(path string) {
	if p.settings.EditorPath == "" {
		openWithShell(path)
		return
	}
	_ = exec.Command(p.settings.EditorPath, path).Start()
}

// openWithShell shows a folder in the file manager, or a file's own menu.
func (p *Plugin) openWithShell(path string, folder bool) {
	if folder {
		p.api.OpenDirectory(path, "")
		return
	}
	openWithShell(path)
}

// openWithShell opens a path the way the shell does, which is what makes a
// document open in its own program.
func openWithShell(path string) {
	if path == "" {
		return
	}
	command := exec.Command("cmd", "/c", "start", "", path)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = command.Start()
}

// procGetDiskFreeSpaceEx is how much room a drive has.
var procGetDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
