package main

// i18nkeys prints the launcher's own translations, so that the interface
// can be written against the keys the reference uses.
//
// Usage: go run ./tools/i18nkeys [language] [pattern]

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"launch/core"
)

func main() {
	language := core.DefaultLanguage
	pattern := ""

	if len(os.Args) > 1 {
		language = os.Args[1]
	}
	if len(os.Args) > 2 {
		pattern = strings.ToLower(os.Args[2])
	}

	// Every key of the language, found by looking each one up.
	keys := core.LanguageKeys(language)
	sort.Strings(keys)

	for _, key := range keys {
		if pattern != "" && !strings.Contains(strings.ToLower(key), pattern) {
			continue
		}
		fmt.Printf("%s\t%s\n", key, core.T(language, key))
	}
}
