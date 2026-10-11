//go:build !windows

package core

// systemLocaleFromDesktop reads the locale from the environment on the
// platforms that keep it there.
func systemLocaleFromDesktop() string {
	return ""
}
