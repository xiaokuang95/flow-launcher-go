package processkiller

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// systemProcesses reads the machine's processes and ends them.
type systemProcesses struct{}

// systemProcessNames are the programs that are part of Windows itself, which
// a user never means to end, and the launcher, which would end the window
// they are typing in.
var systemProcessNames = map[string]bool{
	"conhost":  true,
	"svchost":  true,
	"idle":     true,
	"system":   true,
	"rundll32": true,
	"csrss":    true,
	"lsass":    true,
	"lsm":      true,
	"smss":     true,
	"wininit":  true,
	"winlogon": true,
	"services": true,
	"spoolsv":  true,
	"explorer": true,
}

// launcherProcessName is the launcher's own program name, which is never
// offered: ending it would end the window the user is typing in.
const launcherProcessName = "launch"

// Running returns the processes a user could mean.
func (systemProcesses) Running() ([]Process, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	titles := windowTitles()
	var processes []Process

	entry := windows.ProcessEntry32{Size: uint32(unsafeSizeofProcessEntry32())}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}
	for {
		name := strings.TrimSuffix(windows.UTF16ToString(entry.ExeFile[:]), ".exe")
		lower := strings.ToLower(name)

		if !systemProcessNames[lower] && lower != launcherProcessName {
			process := Process{
				Name:        name,
				ID:          int(entry.ProcessID),
				Path:        processPath(entry.ProcessID),
				WindowTitle: titles[entry.ProcessID],
			}
			processes = append(processes, process)
		}

		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return processes, nil
}

// Kill ends a process.
func (systemProcesses) Kill(process Process) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(process.ID))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.TerminateProcess(handle, 1)
}

// processPath is the file a process was started from, which the system will
// only tell about a process the user may look at.
func processPath(id uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, id)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}

// windowTitles are the titles of the windows the user can see, by the
// process that owns each.
func windowTitles() map[uint32]string {
	titles := map[uint32]string{}

	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if !isWindowVisible(hwnd) {
			return 1
		}
		title := windowText(hwnd)
		if title == "" {
			return 1
		}
		var processID uint32
		threadID, _, _ := procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafePointer(&processID)))
		if threadID == 0 || processID == 0 {
			return 1
		}
		// The first window of a process is the one that names it.
		if _, exists := titles[processID]; !exists {
			titles[processID] = title
		}
		return 1
	})

	procEnumWindows.Call(callback, 0)
	return titles
}

// windowText is the title of a window.
func windowText(hwnd windows.HWND) string {
	length, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, length+1)
	copied, _, _ := procGetWindowText.Call(uintptr(hwnd), uintptr(unsafePointer(&buffer[0])), length+1)
	if copied == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:copied])
}

// isWindowVisible reports whether the user can see a window.
func isWindowVisible(hwnd windows.HWND) bool {
	visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return visible != 0
}

// processName is the name of the program a path ends in, which is what a
// result carries back to the context menu.
func processName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// launcherDirectory is where the launcher's own program lives.
func launcherDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(executable)
}
