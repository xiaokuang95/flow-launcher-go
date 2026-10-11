package core

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Engine runs queries over the plugins and turns what they answer into the
// list the launcher shows: results sorted by score, with the ones the user
// pinned on top and the ones they pick often lifted.
type Engine struct {
	Manager  *PluginManager
	Settings *Settings
	History  *History
	Selected *UserSelectedRecord
	TopMost  *TopMostRecord

	// CustomShortcuts expand what the user typed, longest key first, as the
	// reference does.
	CustomShortcuts []CustomShortcut
	// BuiltinShortcuts expand to something the launcher works out at the
	// time, such as what the clipboard holds.
	BuiltinShortcuts map[string]func() string

	// SearchDelayTime delays a plugin's query, which keeps a slow plugin
	// from holding up the ones that answer at once. Zero searches at once.
	SearchDelayTime time.Duration
	// QueryTimeout bounds how long one plugin may take.
	QueryTimeout time.Duration
}

// NewEngine makes an engine over the plugins and the settings.
func NewEngine(manager *PluginManager, settings *Settings) *Engine {
	return &Engine{
		Manager:          manager,
		Settings:         settings,
		SearchDelayTime:  time.Duration(settings.SearchDelayTime) * time.Millisecond,
		QueryTimeout:     10 * time.Second,
		BuiltinShortcuts: map[string]func() string{},
	}
}

// ResultBatch is one plugin's answer to a query, as it arrives.
type ResultBatch struct {
	// Plugin is the plugin that answered.
	Plugin PluginMetadata
	// Results are its results, scored and ready to show.
	Results []Result
	// Error is what the plugin failed with, when it failed.
	Error error
}

// Search runs a query and returns everything the plugins answered, sorted
// by score. It waits for every plugin.
func (e *Engine) Search(ctx context.Context, queryText string) []Result {
	var mu sync.Mutex
	var all []Result

	// Each answer holds every result found so far, so the last one is the
	// whole list.
	e.SearchStreaming(ctx, queryText, func(batch ResultBatch) {
		mu.Lock()
		defer mu.Unlock()
		all = batch.Results
	})

	mu.Lock()
	defer mu.Unlock()
	return e.rank(all)
}

// SearchStreaming runs a query and calls onResults as each plugin answers,
// with everything found so far, so that the launcher can show results while
// the slower plugins are still working. A plugin that answers again
// replaces its own earlier results and leaves the others alone.
//
// The plugins answer at the same time, so onResults is called from more
// than one goroutine and what it reads and writes has to be its own.
func (e *Engine) SearchStreaming(ctx context.Context, queryText string, onResults func(ResultBatch)) {
	query := e.BuildQuery(queryText)
	if query == nil {
		return
	}

	var mu sync.Mutex
	var all []Result

	emit := func(batch ResultBatch) {
		mu.Lock()
		all = mergeResults(all, batch)
		list := append([]Result(nil), all...)
		mu.Unlock()

		onResults(ResultBatch{Plugin: batch.Plugin, Results: list, Error: batch.Error})
	}

	if query.IsHomeQuery {
		e.searchHome(ctx, query, emit)
		return
	}

	plugins := e.Manager.ValidPluginsForQuery(query)
	e.searchPlugins(ctx, plugins, query, emit)
}

// searchHome answers the home query: what the plugins offer when nothing
// has been typed, and the history.
func (e *Engine) searchHome(ctx context.Context, query *Query, onResults func(ResultBatch)) {
	if e.Settings.ShowHomePage {
		e.searchPlugins(ctx, e.Manager.HomePlugins(), query, onResults)
	}

	if e.Settings.ShowHistoryResultsForHomePage && e.History != nil {
		results := e.History.HomeResults(e.Settings.MaxHistoryResultsToShowForHomePage)
		if len(results) > 0 {
			onResults(ResultBatch{
				Plugin:  PluginMetadata{ID: historyPluginID, Name: "History"},
				Results: results,
			})
		}
	}
}

// searchPlugins queries every plugin at once, and reports each plugin's own
// answer as it arrives.
func (e *Engine) searchPlugins(ctx context.Context, plugins []PluginPair, query *Query, onResults func(ResultBatch)) {
	var wg sync.WaitGroup

	for _, pair := range plugins {
		wg.Add(1)
		go func(pair PluginPair) {
			defer wg.Done()

			if e.SearchDelayTime > 0 {
				select {
				case <-time.After(e.delayFor(pair)):
				case <-ctx.Done():
					return
				}
			}

			batch := e.queryPlugin(ctx, pair, query)
			if ctx.Err() != nil {
				return
			}
			onResults(batch)
		}(pair)
	}

	wg.Wait()
}

// delayFor is how long a plugin waits before it is asked, which is its own
// setting or the launcher's.
func (e *Engine) delayFor(pair PluginPair) time.Duration {
	if pair.Metadata.SearchDelayTime != nil {
		return time.Duration(*pair.Metadata.SearchDelayTime) * time.Millisecond
	}
	return e.SearchDelayTime
}

// queryPlugin asks one plugin and scores what it answers.
func (e *Engine) queryPlugin(ctx context.Context, pair PluginPair, query *Query) ResultBatch {
	batch := ResultBatch{Plugin: pair.Metadata}

	if e.Manager.IsInitializing(pair.Metadata.ID) {
		result := NewResult(
			T("", "pluginStillInitializing"),
			T("", "pluginStillInitializingSubtitle"),
			pair.Metadata.IcoPath, 0)
		result.AutoCompleteText = query.TrimmedQuery
		result.PluginDirectory = pair.Metadata.PluginDirectory
		result.ActionKeywordAssigned = query.ActionKeyword
		result.PluginID = pair.Metadata.ID
		result.OriginQuery = query
		result.Action = func(ActionContext) bool {
			e.Manager.API.ReQuery(true)
			return false
		}
		batch.Results = []Result{result}
		return batch
	}

	timeout := e.QueryTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	pluginCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	var results []Result
	var err error

	if query.IsHomeQuery {
		// The home query asks the plugins that contribute to it for what
		// they offer when nothing has been typed.
		if home, ok := pair.Plugin.(HomePlugin); ok {
			results, err = home.HomeQuery(pluginCtx)
		} else {
			results, err = pair.Plugin.Query(pluginCtx, *query)
		}
	} else {
		results, err = pair.Plugin.Query(pluginCtx, *query)
	}
	elapsed := time.Since(started)

	if ctx.Err() != nil {
		return ResultBatch{Plugin: pair.Metadata}
	}

	if err != nil {
		batch.Error = err
		result := NewResult(
			Tf("", "pluginFailedToRespond", pair.Metadata.Name),
			T("", "pluginFailedToRespondSubtitle"),
			pair.Metadata.IcoPath, 0)
		result.AutoCompleteText = query.TrimmedQuery
		result.PluginDirectory = pair.Metadata.PluginDirectory
		result.ActionKeywordAssigned = query.ActionKeyword
		result.PluginID = pair.Metadata.ID
		result.OriginQuery = query
		batch.Results = []Result{result}
		return batch
	}

	// The plugin's own results are copied, so that the launcher can change
	// the scores of results the plugin still holds.
	scored := make([]Result, 0, len(results))
	for _, result := range results {
		clone := result.Clone()
		if clone.BadgeIcoPath == "" {
			clone.BadgeIcoPath = pair.Metadata.IcoPath
		}
		scored = append(scored, clone)
	}

	UpdatePluginMetadata(scored, pair.Metadata, query)
	batch.Results = e.applyScore(scored, pair.Metadata)

	e.Manager.noteQueryTime(pair.Metadata.ID, elapsed.Milliseconds())
	return batch
}

// applyScore lifts a result the user pinned, and adds what the user's
// picking it before and the plugin's priority are worth.
func (e *Engine) applyScore(results []Result, metadata PluginMetadata) []Result {
	for i := range results {
		result := &results[i]

		if index := e.TopMost.GetTopMostIndex(*result); index != -1 {
			// A pinned result scores by its place among the pinned ones.
			result.Score = MaxScore - index
			continue
		}

		priorityScore := metadata.Priority * 150

		selectedCount := 0
		if result.AddSelectedCount {
			selectedCount = e.Selected.GetSelectedCount(*result)
		}

		total := int64(result.Score) + int64(selectedCount) + int64(priorityScore)
		if total > int64(MaxScore) {
			result.Score = MaxScore
		} else {
			result.Score += selectedCount + priorityScore
		}
	}
	return results
}

// mergeResults adds one plugin's answer to the list so far, replacing that
// plugin's earlier results, and sorts the two by score.
func mergeResults(existing []Result, batch ResultBatch) []Result {
	merged := make([]Result, 0, len(existing)+len(batch.Results))
	for _, result := range existing {
		if result.PluginID != batch.Plugin.ID {
			merged = append(merged, result)
		}
	}
	merged = append(merged, batch.Results...)

	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })
	return merged
}

// rank sorts the results and keeps the ones the launcher shows.
func (e *Engine) rank(results []Result) []Result {
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	limit := e.Settings.MaxResultsToShow
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// BuildQuery turns what the user typed into the query the plugins get,
// expanding the shortcuts first.
func (e *Engine) BuildQuery(text string) *Query {
	if strings.TrimSpace(text) == "" {
		return BuildQuery("", "", e.Manager.NonGlobalPlugins())
	}

	expanded := e.expandShortcuts(text)
	return BuildQuery(text, expanded, e.Manager.NonGlobalPlugins())
}

// expandShortcuts replaces the custom shortcuts and the built-in ones, the
// longest key first so that a longer shortcut wins over a shorter one that
// starts the same way.
func (e *Engine) expandShortcuts(text string) string {
	shortcuts := append([]CustomShortcut(nil), e.CustomShortcuts...)
	sort.SliceStable(shortcuts, func(i, j int) bool { return len(shortcuts[i].Key) > len(shortcuts[j].Key) })

	expanded := text
	for _, shortcut := range shortcuts {
		if shortcut.Key == "" {
			continue
		}
		if expanded == shortcut.Key {
			expanded = shortcut.Expanded
		}
		expanded = strings.ReplaceAll(expanded, "@"+shortcut.Key, shortcut.Expanded)
	}

	for key, value := range e.BuiltinShortcuts {
		// A shortcut is only worked out when it is written, so that one
		// that reads the clipboard reads it only when it is needed.
		if !strings.Contains(expanded, key) {
			continue
		}
		if fn := value; fn != nil {
			expanded = strings.ReplaceAll(expanded, key, fn())
		}
	}

	return strings.TrimSpace(expanded)
}

// historyPluginID is the id the home page's history results carry.
const historyPluginID = "History"

// noteQueryTime records how long a plugin took, which the settings window
// shows.
func (m *PluginManager) noteQueryTime(id string, milliseconds int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pair, ok := m.allLoaded[id]
	if !ok {
		return
	}
	pair.Metadata.QueryCount++
	if pair.Metadata.QueryCount == 1 {
		pair.Metadata.AvgQueryTime = milliseconds
	} else {
		pair.Metadata.AvgQueryTime = (pair.Metadata.AvgQueryTime + milliseconds) / 2
	}
	m.allLoaded[id] = pair
	if initialized, ok := m.allInitialized[id]; ok {
		initialized.Metadata = pair.Metadata
		m.allInitialized[id] = initialized
	}
}
