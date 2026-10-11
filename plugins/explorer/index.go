package explorer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

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
type walkIndex struct{}

// Available reports whether a search can be made.
func (walkIndex) Available() (bool, string) { return true, "" }

// Search finds the files and folders whose name matches, best first.
func (w walkIndex) Search(query string, limit int, excluded []string) ([]SearchResult, error) {
	return w.find(query, limit, excluded)
}

// ContentSearch finds the files whose text matches. A walk cannot read
// every file it passes, so a content search answers with the files whose
// name matches, which is what the user is then offered to look in.
func (w walkIndex) ContentSearch(query string, limit int, excluded []string) ([]SearchResult, error) {
	return w.find(query, limit, excluded)
}

// find walks the folders a user keeps their own files in and matches the
// names against what was typed.
func (w walkIndex) find(query string, limit int, excluded []string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > searchLimit {
		limit = searchLimit
	}

	// The precision is the lowest one, so that a name that is only nearly
	// right is still offered.
	matcher := core.StringMatcher{UserSettingSearchPrecision: core.SearchPrecisionLow}

	type scored struct {
		result SearchResult
		score  int
	}
	var found []scored
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
			if inExcludedFolder(path, excluded) {
				return nil
			}

			// The folders looked in overlap, so the same file can be
			// reached twice.
			if seen[path] {
				return nil
			}

			match := matcher.FuzzyMatch(query, name)
			if !match.IsSearchPrecisionScoreMet() {
				return nil
			}
			seen[path] = true

			found = append(found, scored{
				result: SearchResult{
					FullPath:      path,
					Type:          ResultTypeFile,
					Score:         match.Score(),
					HighlightData: match.MatchData,
				},
				score: match.Score(),
			})
			return nil
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

// roots are the folders a search looks in: the user's own folders.
func (w walkIndex) roots() []string {
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
