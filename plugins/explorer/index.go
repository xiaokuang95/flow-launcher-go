package explorer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"launch/core"
)

// The explorer finds files by name by walking the folders a person keeps
// their own work in, rather than by asking the machine's own index of its
// files.
//
// The Windows index is reached through an OLE DB provider, and that
// provider answers a query only once it has been given a catalog and a set
// of properties, which the reference's own helper sets for it before it
// hands the query over. Asking it directly answers every query with
// "there is an error in the command", so this walks instead.
//
// The walk is bounded: a depth, a number of entries looked at, and the
// folders that never hold what someone is searching for. What it cannot
// reach is a folder nobody keeps their own files in.
const (
	// searchDepth is how far below a folder the walk goes.
	searchDepth = 6
	// searchBudget is how many entries are looked at in one search, so that
	// a search of a large folder answers rather than runs.
	searchBudget = 20000
	// searchLimit is how many results a search answers with at most.
	searchLimit = 200
)

// skippedFolders are the folders a search never looks inside: they hold a
// program's own files rather than the user's, and they are deep.
var skippedFolders = map[string]bool{
	"appdata":                   true,
	"node_modules":              true,
	".git":                      true,
	".svn":                      true,
	".cache":                    true,
	"__pycache__":               true,
	"windows":                   true,
	"$recycle.bin":              true,
	"system volume information": true,
	"program files":             true,
	"program files (x86)":       true,
	"programdata":               true,
}

// walkIndex finds the machine's files by walking the folders the user keeps
// their own work in.
//
// The walk is kept between searches, because the folders it reads do not
// change between two keystrokes and walking them again for every one of
// them is what makes typing feel slow. It is read again when it is old, or
// when the user asks for it.
type walkIndex struct {
	mu       sync.Mutex
	entries  []walkEntry
	walkedAt time.Time
	// walking is true while the folders are being read, so that only one
	// walk runs at a time.
	walking bool
}

// walkEntry is one file the walk found.
type walkEntry struct {
	path  string
	name  string
	isDir bool
}

// indexAge is how long a walk is used before the folders are read again.
const indexAge = 30 * time.Second

// Available reports whether a search can be made.
func (w *walkIndex) Available() (bool, string) { return true, "" }

// Search finds the files and folders whose name matches, best first.
func (w *walkIndex) Search(query string, limit int, excluded []string, matcher core.StringMatcher) ([]SearchResult, error) {
	return w.find(query, limit, excluded, matcher)
}

// ContentSearch finds the files whose text matches. A walk cannot read
// every file it passes, so a content search answers with the files whose
// name matches, which is what the user is then offered to look in.
func (w *walkIndex) ContentSearch(query string, limit int, excluded []string, matcher core.StringMatcher) ([]SearchResult, error) {
	return w.find(query, limit, excluded, matcher)
}

// Reload reads the folders again, as F5 asks.
func (w *walkIndex) Reload() {
	w.mu.Lock()
	w.entries = nil
	w.walkedAt = time.Time{}
	w.mu.Unlock()
}

// Warm reads the folders in the background, so that the first search the
// user makes does not have to wait for the walk.
func (w *walkIndex) Warm() {
	go w.refresh()
}

// entriesNow is what the walk found. Reading the folders again takes about
// a third of a second, which must not happen while the user is typing, so
// what was found last answers the search however old it is and the folders
// are read again in the background. Only a search with nothing at all to
// answer with waits for a walk.
func (w *walkIndex) entriesNow() []walkEntry {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.entries) > 0 {
		if time.Since(w.walkedAt) >= indexAge && !w.walking {
			go w.refresh()
		}
		return w.entries
	}

	w.entries = w.walk()
	w.walkedAt = time.Now()
	return w.entries
}

// refresh reads the folders again and keeps what it found, in the
// background.
func (w *walkIndex) refresh() {
	w.mu.Lock()
	if w.walking {
		w.mu.Unlock()
		return
	}
	w.walking = true
	w.mu.Unlock()

	entries := w.walk()

	w.mu.Lock()
	w.entries, w.walkedAt, w.walking = entries, time.Now(), false
	w.mu.Unlock()
}

// find matches what was typed against the files the walk found, using the
// launcher's own matcher, so that a name written in Chinese is found by its
// pinyin and accents are folded away when the user asked for that.
func (w *walkIndex) find(query string, limit int, excluded []string, matcher core.StringMatcher) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > searchLimit {
		limit = searchLimit
	}

	// The precision is the lowest one, so that a name that is only nearly
	// right is still offered.
	matcher.UserSettingSearchPrecision = core.SearchPrecisionLow

	// Every character of the query has to appear in a name, in order, for
	// the match to have any chance; checking that first is far cheaper
	// than scoring twenty thousand names, and rejects almost all of them.
	// The check compares letters as they are written, so it is only sound
	// while the matcher does the same.
	needle := []rune(strings.ToLower(query))
	prefilter := !matcher.IgnoreAccents

	type scored struct {
		result SearchResult
		score  int
	}
	var found []scored

	for _, entry := range w.entriesNow() {
		if inExcludedFolder(entry.path, excluded) {
			continue
		}

		if prefilter && !mightMatch(needle, entry.name, matcher.Alphabet) {
			continue
		}

		match := matcher.FuzzyMatch(query, entry.name)
		if !match.IsSearchPrecisionScoreMet() {
			continue
		}

		kind := ResultTypeFile
		if entry.isDir {
			kind = ResultTypeFolder
		}
		found = append(found, scored{
			result: SearchResult{
				FullPath:      entry.path,
				Type:          kind,
				Score:         match.Score(),
				HighlightData: match.MatchData,
			},
			score: match.Score(),
		})
	}

	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	if len(found) > limit {
		found = found[:limit]
	}

	results := make([]SearchResult, 0, len(found))
	for _, entry := range found {
		results = append(results, entry.result)
	}
	return results, nil
}

// mightMatch is the cheap check above for one name: every character of the
// query has to appear in it, in order. A name written in Chinese is matched
// by its pinyin, which the characters themselves do not hold, so what the
// matcher will compare it with is checked as well.
func mightMatch(needle []rune, name string, alphabet core.Alphabet) bool {
	if isSubsequence(needle, name) {
		return true
	}
	if alphabet == nil || !core.HasChinese(name) {
		return false
	}
	translated, _ := alphabet.Translate(name)
	return isSubsequence(needle, translated)
}

// isSubsequence reports whether every character of the needle appears in
// the text, in order, whatever their case. A fuzzy match cannot succeed
// without it.
func isSubsequence(needle []rune, text string) bool {
	if len(needle) == 0 {
		return true
	}

	at := 0
	for _, r := range text {
		if toLowerRune(r) == needle[at] {
			at++
			if at == len(needle) {
				return true
			}
		}
	}
	return false
}

// toLowerRune is the lower case of a rune, for the cheap check above.
func toLowerRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// walk reads the folders a user keeps their own files in, once.
func (w *walkIndex) walk() []walkEntry {
	var entries []walkEntry
	seen := map[string]bool{}
	looked := 0

	for _, root := range w.roots() {
		if looked > searchBudget {
			break
		}

		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				// A folder the user may not look into is left alone.
				return nil
			}
			looked++
			if looked > searchBudget {
				return filepath.SkipAll
			}

			name := entry.Name()
			if entry.IsDir() {
				if skippedFolders[strings.ToLower(name)] || depthBelow(root, path) > searchDepth {
					return filepath.SkipDir
				}
				return nil
			}

			// The folders looked in overlap, so the same file can be
			// reached twice.
			if seen[path] {
				return nil
			}
			seen[path] = true

			entries = append(entries, walkEntry{path: path, name: name})
			return nil
		})
	}
	return entries
}

// roots are the folders a search looks in: the user's own folders.
func (w *walkIndex) roots() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}

	roots := []string{}
	for _, name := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Videos", "Music"} {
		candidate := filepath.Join(home, name)
		if core.DirectoryExists(candidate) {
			roots = append(roots, candidate)
		}
	}
	// The home folder itself is looked in last, so that what is inside the
	// folders above is found first.
	return append(roots, home)
}

// depthBelow is how far a path is below a folder.
func depthBelow(root, path string) int {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return 0
	}
	if relative == "." {
		return 0
	}
	return strings.Count(relative, string(filepath.Separator)) + 1
}

// inExcludedFolder reports whether a path is inside one of the folders the
// user left out of a search.
func inExcludedFolder(path string, excluded []string) bool {
	lower := strings.ToLower(path)
	for _, folder := range excluded {
		if folder == "" {
			continue
		}
		if strings.HasPrefix(lower, strings.ToLower(folder)) {
			return true
		}
	}
	return false
}
