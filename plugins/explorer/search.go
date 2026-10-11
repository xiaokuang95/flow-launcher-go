package explorer

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"launch/core"
)

// ResultType is what a search found.
type ResultType int

// What a search can find.
const (
	ResultTypeFile ResultType = iota
	ResultTypeFolder
	ResultTypeVolume
)

// SearchResult is one file or folder the explorer found.
type SearchResult struct {
	// FullPath is where it is.
	FullPath string
	// Type is what it is.
	Type ResultType
	// Score is how well it matched, which the index search does not know.
	Score int
	// Indexed is true for a result of the machine's own index, which is
	// what decides the icon and the tooltip.
	Indexed bool
	// HighlightData are the runes of the name the query matched.
	HighlightData []int
}

// FileIndex is the machine's own index of its files, which is what makes a
// search of the whole machine quick. It is kept behind a field so that a
// test can search a list of its own.
type FileIndex interface {
	// Available reports whether the index can be searched, and why not
	// when it cannot.
	Available() (bool, string)
	// Search finds the files whose name matches, up to a limit, with the
	// launcher's own matcher: a name written in Chinese is found by its
	// pinyin, and the precision is the lowest one.
	Search(query string, limit int, excluded []string, matcher core.StringMatcher) ([]SearchResult, error)
	// ContentSearch finds the files whose text matches.
	ContentSearch(query string, limit int, excluded []string, matcher core.StringMatcher) ([]SearchResult, error)
}

// pathSearch answers a path with what it holds, and with the folders and
// files whose name starts the way the path does.
func (p *Plugin) pathSearch(ctx context.Context, query core.Query) ([]core.Result, error) {
	search := query.Search

	// A path written with the other separator is read the same way.
	search = strings.ReplaceAll(search, "/", string(filepath.Separator))
	search = expandEnvironmentVariables(search)

	// The part of the path that names a folder that is there.
	directory := previousDirectory(search)
	if !core.DirectoryExists(directory) {
		return nil, nil
	}

	results := []core.Result{}
	// The folder itself is offered first, so that Enter opens it.
	if strings.HasSuffix(directory, ":"+string(filepath.Separator)) {
		results = append(results, p.driveResult(directory, query.ActionKeyword))
	} else {
		results = append(results, p.openFolderResult(directory, query.ActionKeyword))
	}

	entries, err := p.listDirectory(ctx, search)
	if err != nil {
		return results, nil
	}
	for _, entry := range entries {
		results = append(results, p.resultFor(query, entry))
	}
	return results, nil
}

// listDirectory lists what the folder the path names holds, and what it
// holds below when the path asks for that with a `>`.
func (p *Plugin) listDirectory(ctx context.Context, search string) ([]SearchResult, error) {
	recursive := false
	if index := strings.Index(search, ">"); index > 0 {
		// A `>` in a path asks for everything below it.
		search = strings.TrimSpace(search[:index])
		recursive = true
	}

	directory := previousDirectory(search)

	// What is typed is a prefix of the names to list, unless the path
	// already names a folder that is there, in which case everything in it
	// is listed.
	prefix := ""
	if !strings.HasSuffix(search, string(filepath.Separator)) && !core.DirectoryExists(search) {
		prefix = strings.ToLower(filepath.Base(search))
	}

	var results []SearchResult
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		name := entry.Name()
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), prefix) {
			// A folder that does not match is not looked inside, when the
			// path asked for everything below.
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		results = append(results, SearchResult{
			FullPath: filepath.Join(path, name),
			Type:     typeOf(entry),
		})
		if !recursive && entry.IsDir() {
			return nil
		}
		return nil
	}

	if recursive {
		_ = filepath.WalkDir(directory, walk)
	} else {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := walk(directory, entry, nil); err != nil {
				break
			}
		}
	}

	// Folders first, then files, each by their path, as the reference
	// lists them.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Type != results[j].Type {
			return results[i].Type > results[j].Type
		}
		return results[i].FullPath < results[j].FullPath
	})
	return results, nil
}

// typeOf says what an entry is.
func typeOf(entry os.DirEntry) ResultType {
	if entry.IsDir() {
		if entry.Name() == "" {
			return ResultTypeVolume
		}
		return ResultTypeFolder
	}
	return ResultTypeFile
}

// indexSearch finds the files the machine's index knows about.
func (p *Plugin) indexSearch(ctx context.Context, query core.Query) ([]core.Result, error) {
	available, _ := p.index.Available()
	if !available {
		return nil, nil
	}

	found, err := p.index.Search(query.Search, p.settings.MaxResult, p.excludedPaths(), p.api.Matcher())
	if err != nil {
		p.api.LogError("Explorer", "searching the index failed: "+err.Error())
		return nil, nil
	}

	results := make([]core.Result, 0, len(found))
	for _, entry := range found {
		results = append(results, p.resultFor(query, entry))
	}
	return results, nil
}

// contentSearch finds the files whose text matches.
func (p *Plugin) contentSearch(ctx context.Context, query core.Query) ([]core.Result, error) {
	available, _ := p.index.Available()
	if !available {
		return nil, nil
	}

	found, err := p.index.ContentSearch(query.Search, p.settings.MaxResult, p.excludedPaths(), p.api.Matcher())
	if err != nil {
		p.api.LogError("Explorer", "searching the index failed: "+err.Error())
		return nil, nil
	}

	results := make([]core.Result, 0, len(found))
	for _, entry := range found {
		results = append(results, p.resultFor(query, entry))
	}
	return results, nil
}

// quickAccessResults are the folders the user pinned, all of them when
// nothing is typed and the ones that match otherwise.
func (p *Plugin) quickAccessResults(query core.Query, search string) []core.Result {
	matcher := p.api.Matcher()

	type scored struct {
		link  AccessLink
		score int
	}
	var matched []scored

	for _, link := range p.settings.QuickAccessLinks {
		score := 0
		if search != "" {
			nameMatch := matcher.FuzzyMatch(search, link.Name)
			pathMatch := matcher.FuzzyMatch(search, link.Path)
			if pathMatch.Score() > nameMatch.Score() {
				score = pathMatch.Score()
			} else {
				score = nameMatch.Score()
			}
			if score <= 0 {
				continue
			}
		}
		matched = append(matched, scored{link: link, score: score})
	}

	results := make([]core.Result, 0, len(matched))
	for _, entry := range matched {
		link := entry.link
		result := p.openFolderResult(link.Path, query.ActionKeyword)
		result.Title = link.Name
		result.Score = entry.score
		result.IcoPath = "Images/quickaccess.png"
		result.Action = func(action core.ActionContext) bool {
			switch {
			case action.SpecialKeyState.CtrlPressed:
				p.api.OpenDirectory(link.Path, "")
			default:
				p.api.ChangeQuery(pathWithKeyword(link.Path, query.ActionKeyword), false)
			}
			return false
		}
		results = append(results, result)
	}
	return results
}

// excludedPaths are the folders a search of the index leaves out.
func (p *Plugin) excludedPaths() []string {
	paths := make([]string, 0, len(p.settings.IndexSearchExcludedSubdirectoryPaths))
	for _, excluded := range p.settings.IndexSearchExcludedSubdirectoryPaths {
		paths = append(paths, excluded.Path)
	}
	return paths
}

// expandEnvironmentVariables replaces %VAR% with what the machine has.
func expandEnvironmentVariables(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '%' {
			out.WriteByte(value[i])
			i++
			continue
		}
		end := strings.IndexByte(value[i+1:], '%')
		if end < 0 {
			out.WriteByte(value[i])
			i++
			continue
		}
		name := value[i+1 : i+1+end]
		if name == "" {
			out.WriteByte('%')
			i++
			continue
		}
		out.WriteString(os.Getenv(name))
		i += end + 2
	}
	return out.String()
}

// previousDirectory is the folder a path names: the path itself when it is
// a folder that is there, and the folder it is inside when it is a name
// still being typed.
func previousDirectory(path string) string {
	if strings.HasSuffix(path, string(filepath.Separator)) || core.DirectoryExists(path) {
		return path
	}
	parent := filepath.Dir(path)
	if parent == "." {
		return path
	}
	return parent
}

// looksLikePath reports whether what the user typed is a path rather than a
// search: something with a separator or a drive letter in it.
func looksLikePath(search string) bool {
	if strings.ContainsAny(search, "/\\") {
		return true
	}
	// A drive letter, as in "c:".
	if len(search) >= 2 && search[1] == ':' {
		return true
	}
	// An environment variable, as in "%appdata%".
	return strings.Contains(search, "%")
}
