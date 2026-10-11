package core

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// IconProvider is what a platform supplies so that a result can show the
// icon of a file, an application or a shortcut.
type IconProvider interface {
	// FileIcon returns the icon of a path as PNG bytes, or nil when the
	// path has no icon.
	FileIcon(path string) []byte
	// LargeIconSize is the size in pixels the icons are asked for.
	LargeIconSize() int
}

// iconCache keeps the icons of paths, so that a window asking for the same
// icon every frame reads it once.
type iconCache struct {
	mu     sync.Mutex
	byPath map[string][]byte
}

var sharedIcons = &iconCache{byPath: map[string][]byte{}}

// IconBytes returns the icon of a path as PNG bytes: the file itself when
// it is an image, and otherwise what the platform says the path looks like.
// An empty answer means the path has no icon.
func IconBytes(path string) []byte {
	if path == "" {
		return nil
	}

	sharedIcons.mu.Lock()
	if cached, ok := sharedIcons.byPath[path]; ok {
		sharedIcons.mu.Unlock()
		return cached
	}
	sharedIcons.mu.Unlock()

	var icon []byte
	if isImageFile(path) {
		if content, err := os.ReadFile(path); err == nil {
			icon = content
		}
	}
	if icon == nil && platformIcons != nil {
		icon = platformIcons.FileIcon(path)
	}

	sharedIcons.mu.Lock()
	sharedIcons.byPath[path] = icon
	sharedIcons.mu.Unlock()
	return icon
}

// platformIcons is the platform's icon provider, nil where there is none.
var platformIcons IconProvider

// SetIconProvider gives the launcher the platform's icons.
func SetIconProvider(provider IconProvider) { platformIcons = provider }

// isImageFile reports whether a path is a picture the window can draw
// itself.
func isImageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	default:
		return false
	}
}

// FileExists reports whether a path is a file that is there.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// DirectoryExists reports whether a path is a directory that is there.
func DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
