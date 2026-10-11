//go:build !darwin

package main

// isDarwin is false everywhere but macOS, where the desktop's opener is
// `open` rather than `xdg-open`.
func isDarwin() bool { return false }
