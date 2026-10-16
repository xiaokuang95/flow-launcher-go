package copyq

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The small helpers the history and the results use.

// baseName is the name of a file without the folders it is in.
func baseName(path string) string { return filepath.Base(path) }

// directoryName is the folder a file is in.
func directoryName(path string) string { return filepath.Dir(path) }

// oneLine writes text as one line, which is what a result's title is: the
// first line, with the whitespace between words as it is.
func oneLine(text string) string {
	if first := strings.IndexAny(text, "\r\n"); first >= 0 {
		text = text[:first]
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 120 {
		runes := []rune(text)
		if len(runes) > 120 {
			text = string(runes[:119]) + "…"
		}
	}
	return text
}

// containsFold reports whether text holds what is being looked for, without
// minding the case.
func containsFold(text, lookFor string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(lookFor))
}

// itoa writes a number, which is all the history needs.
func itoa(number int) string { return strconv.Itoa(number) }

// ago writes how long ago something was, the way a person reads it.
func ago(when, now time.Time) string {
	if when.IsZero() {
		return ""
	}

	elapsed := now.Sub(when)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return itoa(int(elapsed.Minutes())) + "m ago"
	case elapsed < 24*time.Hour:
		return itoa(int(elapsed.Hours())) + "h ago"
	default:
		return itoa(int(elapsed.Hours()/24)) + "d ago"
	}
}
