//go:build !darwin

package explorer

// isDarwin says the desktop's opener is `xdg-open`.
func isDarwin() bool { return false }
