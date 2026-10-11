package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// JSONStorage reads and writes a value as JSON, the way the reference's
// JsonStorage does: it writes to a temporary file first and keeps the last
// good file as a backup, so that a crash halfway through a write cannot
// lose the settings.
type JSONStorage[T any] struct {
	// Path is the file the value is read from and written to.
	Path string
}

// NewJSONStorage makes a storage for a file, creating its directory.
func NewJSONStorage[T any](path string) JSONStorage[T] {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return JSONStorage[T]{Path: path}
}

// Exists reports whether the file is there.
func (s JSONStorage[T]) Exists() bool {
	_, err := os.Stat(s.Path)
	return err == nil
}

// Load reads the value, or returns the fallback when the file is missing or
// cannot be read.
func (s JSONStorage[T]) Load(fallback T) T {
	serialized, err := os.ReadFile(s.Path)
	if err != nil || len(serialized) == 0 {
		return fallback
	}

	var value T
	if err := json.Unmarshal(serialized, &value); err != nil {
		// A file that cannot be read is kept beside the good one rather
		// than overwritten, so that what the user had is not lost.
		_ = os.Rename(s.Path, s.Path+".invalid")
		return fallback
	}
	return value
}

// Save writes the value: the temporary file first, then the old file aside,
// then the new one in its place.
func (s JSONStorage[T]) Save(value T) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}

	serialized, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	temp := s.Path + ".tmp"
	if err := os.WriteFile(temp, serialized, 0o644); err != nil {
		return err
	}

	if _, err := os.Stat(s.Path); err == nil {
		_ = os.Rename(s.Path, s.Path+".bak")
	}
	return os.Rename(temp, s.Path)
}

// Delete removes the file and what it left behind.
func (s JSONStorage[T]) Delete() {
	for _, path := range []string{s.Path, s.Path + ".bak", s.Path + ".tmp"} {
		_ = os.Remove(path)
	}
}
