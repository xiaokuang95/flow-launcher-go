//go:build !windows

package explorer

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
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
	stat := syscall.Statfs_t{}
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, false
	}
	blockSize := int64(stat.Bsize)
	return int64(stat.Bavail) * blockSize, int64(stat.Blocks) * blockSize, true
}

// creationTime is when a file was made; elsewhere the system does not keep
// it, so the time it last changed is used.
func creationTime(info os.FileInfo) time.Time { return info.ModTime() }

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

// openWithShell opens a folder in the file manager, or a file in its own
// program.
func (p *Plugin) openWithShell(path string, folder bool) { openWithShell(path) }

// openWithShell opens a path with the desktop's own opener.
func openWithShell(path string) {
	if path == "" {
		return
	}
	command := "xdg-open"
	if isDarwin() {
		command = "open"
	}
	_ = exec.Command(command, path).Start()
}
