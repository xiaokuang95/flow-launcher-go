package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// HistoryLimit is how many results the launcher remembers opening.
const HistoryLimit = 300

// HistoryItem is one query the user ran, in the reference's older format.
type HistoryItem struct {
	Query            string    `json:"Query"`
	ExecutedDateTime time.Time `json:"ExecutedDateTime"`
}

// LastOpenedHistoryResult is a result the user opened, with the query that
// found it, so that picking it from the home page runs it again.
type LastOpenedHistoryResult struct {
	Title     string `json:"Title"`
	SubTitle  string `json:"SubTitle"`
	PluginID  string `json:"PluginID"`
	IcoPath   string `json:"IcoPath"`
	RecordKey string `json:"RecordKey"`
	// Query is the trimmed query that found the result.
	Query string `json:"Query"`
	// ExecutedDateTime is when the user opened it.
	ExecutedDateTime time.Time `json:"ExecutedDateTime"`

	PluginDirectory string     `json:"-"`
	Glyph           *GlyphInfo `json:"Glyph,omitempty"`
}

// Equals reports whether a result is the one this history entry remembers,
// under the same query.
func (h LastOpenedHistoryResult) Equals(result Result) bool {
	query := ""
	if result.OriginQuery != nil {
		query = result.OriginQuery.TrimmedQuery
	}

	if h.RecordKey == "" || result.RecordKey == "" {
		return h.Title == result.Title &&
			h.SubTitle == result.SubTitle &&
			h.PluginID == result.PluginID &&
			h.Query == query
	}
	return h.RecordKey == result.RecordKey && h.PluginID == result.PluginID && h.Query == query
}

// History remembers what the user opened, which the home page lists.
type History struct {
	// Items is the reference's older history, folded in on load.
	Items []HistoryItem `json:"Items"`
	// LastOpenedHistoryItems is what the home page shows.
	LastOpenedHistoryItems []LastOpenedHistoryResult `json:"LastOpenedHistoryItems"`

	path string
	mu   sync.Mutex
}

// LoadHistory reads the history, or makes an empty one.
func LoadHistory(path string) *History {
	history := &History{path: path}

	var stored struct {
		Items                  []HistoryItem             `json:"Items"`
		LastOpenedHistoryItems []LastOpenedHistoryResult `json:"LastOpenedHistoryItems"`
	}
	if err := json.Unmarshal(mustReadFile(path), &stored); err != nil {
		return history
	}
	history.Items = stored.Items
	history.LastOpenedHistoryItems = stored.LastOpenedHistoryItems
	history.populateFromLegacyHistory()
	return history
}

// populateFromLegacyHistory folds the older list of queries into the list of
// opened results, as the reference does when it loads an old file.
func (h *History) populateFromLegacyHistory() {
	if len(h.Items) == 0 {
		return
	}
	for _, item := range h.Items {
		h.LastOpenedHistoryItems = append(h.LastOpenedHistoryItems, LastOpenedHistoryResult{
			Title:            item.Query,
			Query:            item.Query,
			ExecutedDateTime: item.ExecutedDateTime,
		})
	}
	h.Items = nil
}

// Add remembers that the user opened a result.
func (h *History) Add(result Result) {
	if result.OriginQuery == nil || result.OriginQuery.TrimmedQuery == "" {
		return
	}
	// A result from the home page carries no plugin id, and is left out so
	// that the home page does not fill up with its own entries.
	if result.PluginID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.LastOpenedHistoryItems) > HistoryLimit {
		h.LastOpenedHistoryItems = h.LastOpenedHistoryItems[1:]
	}

	if len(h.LastOpenedHistoryItems) > 0 {
		last := &h.LastOpenedHistoryItems[len(h.LastOpenedHistoryItems)-1]
		if last.Equals(result) {
			last.ExecutedDateTime = time.Now()
			last.IcoPath = result.IcoPath
			last.Glyph = result.Glyph
			return
		}
	}

	h.LastOpenedHistoryItems = append(h.LastOpenedHistoryItems, LastOpenedHistoryResult{
		Title:            result.Title,
		SubTitle:         result.SubTitle,
		PluginID:         result.PluginID,
		IcoPath:          result.IcoPath,
		RecordKey:        result.RecordKey,
		Query:            result.OriginQuery.TrimmedQuery,
		ExecutedDateTime: time.Now(),
		PluginDirectory:  result.PluginDirectory,
		Glyph:            result.Glyph,
	})
}

// Remove forgets an entry, and with removeAllMatchingResults every entry
// that names the same result.
func (h *History) Remove(item LastOpenedHistoryResult, removeAllMatchingResults bool) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !removeAllMatchingResults {
		for i, candidate := range h.LastOpenedHistoryItems {
			if candidate.Title == item.Title && candidate.Query == item.Query &&
				candidate.ExecutedDateTime.Equal(item.ExecutedDateTime) {
				h.LastOpenedHistoryItems = append(h.LastOpenedHistoryItems[:i], h.LastOpenedHistoryItems[i+1:]...)
				return 1
			}
		}
		return 0
	}

	kept := h.LastOpenedHistoryItems[:0]
	removed := 0
	for _, candidate := range h.LastOpenedHistoryItems {
		if candidate.Title == item.Title &&
			candidate.SubTitle == item.SubTitle &&
			candidate.PluginID == item.PluginID &&
			candidate.RecordKey == item.RecordKey {
			removed++
			continue
		}
		kept = append(kept, candidate)
	}
	h.LastOpenedHistoryItems = kept
	return removed
}

// UpdatePluginDirectories points the remembered icons at where the plugins
// live now, which matters after the launcher is moved or a plugin updated.
func (h *History) UpdatePluginDirectories(manager *PluginManager) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.LastOpenedHistoryItems {
		item := &h.LastOpenedHistoryItems[i]
		if item.PluginID == "" {
			continue
		}
		if pair, ok := manager.GetPluginForID(item.PluginID); ok {
			item.PluginDirectory = pair.Metadata.PluginDirectory
		}
	}
}

// Save writes the history out.
func (h *History) Save() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	serialized, err := json.MarshalIndent(struct {
		Items                  []HistoryItem             `json:"Items"`
		LastOpenedHistoryItems []LastOpenedHistoryResult `json:"LastOpenedHistoryItems"`
	}{h.Items, h.LastOpenedHistoryItems}, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(h.path, serialized)
}

// HomeResults are the entries the home page lists, most recent last.
func (h *History) HomeResults(limit int) []Result {
	h.mu.Lock()
	defer h.mu.Unlock()

	if limit <= 0 || limit > len(h.LastOpenedHistoryItems) {
		limit = len(h.LastOpenedHistoryItems)
	}
	// The last opened are the most recent, and the home page shows them
	// newest first.
	items := h.LastOpenedHistoryItems[len(h.LastOpenedHistoryItems)-limit:]

	results := make([]Result, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		query := item.Query
		result := NewResult(item.Title, item.SubTitle, item.IcoPath, 0)
		result.PluginID = item.PluginID
		result.PluginDirectory = item.PluginDirectory
		result.Glyph = item.Glyph
		result.ShowBadge = true
		result.BadgeIcoPath = HistoryIconPath
		result.Action = func(ActionContext) bool {
			if historyAction != nil {
				historyAction(query)
			}
			return false
		}
		results = append(results, result)
	}
	return results
}

// historyAction is what the launcher does when a history entry is picked:
// it goes back to the results and types the query again.
var historyAction func(query string)

// SetHistoryAction tells the history entries how to run their query again.
func SetHistoryAction(action func(query string)) { historyAction = action }

// HistoryIconPath is the icon of a history entry.
const HistoryIconPath = "Images/history.png"

// TimeAgo says how long ago something happened, in the words the reference
// uses.
func TimeAgo(when time.Time) string {
	span := time.Since(when)

	switch {
	case span.Hours() > 24*365:
		years := int(span.Hours() / (24 * 365))
		if int(span.Hours())%(24*365) != 0 {
			years++
		}
		return plural(years, "year")
	case span.Hours() > 24*30:
		months := int(span.Hours() / (24 * 30))
		if int(span.Hours())%(24*31) != 0 {
			months++
		}
		return plural(months, "month")
	case span.Hours() > 24:
		return plural(int(span.Hours()/24), "day")
	case span.Hours() > 1:
		return plural(int(span.Hours()), "hour")
	case span.Minutes() > 1:
		return plural(int(span.Minutes()), "minute")
	case span.Seconds() > 5:
		return fmt.Sprintf("about %d seconds ago", int(span.Seconds()))
	default:
		return "just now"
	}
}

func plural(count int, unit string) string {
	if count == 1 {
		return fmt.Sprintf("about %d %s ago", count, unit)
	}
	return fmt.Sprintf("about %d %ss ago", count, unit)
}

// LastExecuteTime is the subtitle of a history entry.
func LastExecuteTime(when time.Time) string {
	return "Last executed: " + TimeAgo(when)
}

// TitleCaseWords capitalizes the words of a title, for a plugin that shows
// one it did not write.
func TitleCaseWords(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
