//go:build windows

package explorer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

// openIn opens a path in a program the user chose.
func (p *Plugin) openIn(program, path string) error {
	return exec.Command(program, path).Start()
}

// openFolderIn opens a folder in the program the user chose, with the
// folder as the program's own folder.
func (p *Plugin) openFolderIn(program, path string) error {
	command := exec.Command(program)
	command.Dir = path
	return command.Start()
}

// openWithMenu shows the machine's own list of programs a file can be
// opened with, which is what the shell's Open With dialog is.
func openWithMenu(path string) {
	systemDirectory, err := windowsDirectory()
	if err != nil {
		return
	}
	command := exec.Command("rundll32.exe", filepath.Join(systemDirectory, "shell32.dll")+",OpenAs_RunDLL "+path)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = command.Start()
}

// runAsDifferentUser runs a file as another user, which the shell asks for
// with the runasuser verb.
func runAsDifferentUser(path string) error {
	command := exec.Command("powershell.exe", "-NoProfile", "-Command",
		"Start-Process -FilePath "+quoteForPowerShell(path)+" -Verb RunAsUser")
	return command.Start()
}

// quoteForPowerShell writes a path so that PowerShell reads it as one
// argument, whatever is in it.
func quoteForPowerShell(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}

// windowsDirectory is where the system's own programs live.
func windowsDirectory() (string, error) {
	directory := os.Getenv("SystemRoot")
	if directory == "" {
		return "", errors.New("SystemRoot is not set")
	}
	return filepath.Join(directory, "System32"), nil
}

// openInEditor opens a file in the editor the user chose.
func (p *Plugin) openInEditor(path string) {
	if p.settings.EditorPath == "" {
		openWithShell(path)
		return
	}
	_ = p.openIn(p.settings.EditorPath, path)
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
