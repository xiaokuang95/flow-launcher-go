package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf16"
)

// How the reference builds the hashes that identify a result. The
// multiplier and the starting value are the ones Java's String.hashCode
// uses, which the reference copied so that the overflow is well defined.
const (
	hashMultiplier = 31
	hashInitial    = 23
)

// staticHashCode is the reference's GenerateStaticHashCode: it folds a
// string into an int32, wrapping around as the reference's unchecked
// arithmetic does. It folds UTF-16 code units, as the reference's
// char-by-char loop does, so that the counts and pins an older Flow
// Launcher wrote still name the same results.
func staticHashCode(s string, start int) int {
	hash := int32(start)
	for _, unit := range utf16.Encode([]rune(s)) {
		hash = hash*hashMultiplier + int32(unit)
	}
	return int(hash)
}

// ResultHashCode identifies a result by its record key, or by its title and
// subtitle.
func ResultHashCode(result Result) int {
	if result.RecordKey == "" {
		return staticHashCode(result.SubTitle, staticHashCode(result.Title, hashInitial))
	}
	return staticHashCode(result.RecordKey, hashInitial)
}

// QueryAndResultHashCode identifies a result within the query that found
// it. A result chosen from a context menu has no query, and is identified
// by itself alone.
func QueryAndResultHashCode(query *Query, result Result) int {
	if query == nil {
		return ResultHashCode(result)
	}

	hash := staticHashCode(query.Search, staticHashCode(query.ActionKeyword, hashInitial))
	if result.RecordKey == "" {
		hash = staticHashCode(result.Title, hash)
		return staticHashCode(result.SubTitle, hash)
	}
	return staticHashCode(result.RecordKey, hash)
}

// UserSelectedRecord counts how often the user picked a result, so that the
// ones they pick often rise. A result picked after a query of its own is
// counted separately, and weighs five times as much, because asking for
// something by name says more than picking it from a list.
type UserSelectedRecord struct {
	// RecordsWithQuery maps the hash of a result in its query to the count.
	RecordsWithQuery map[int]int `json:"recordsWithQuery"`
	// Records is the reference's older map, keyed by a string, which is
	// read once and folded into RecordsWithQuery.
	Records map[string]int `json:"records,omitempty"`

	path string
	mu   sync.Mutex
}

// LoadUserSelectedRecord reads the counts, or makes empty ones.
func LoadUserSelectedRecord(path string) *UserSelectedRecord {
	record := &UserSelectedRecord{RecordsWithQuery: map[int]int{}, path: path}

	var stored struct {
		RecordsWithQuery map[int]int    `json:"recordsWithQuery"`
		Records          map[string]int `json:"records"`
	}
	if err := json.Unmarshal(mustReadFile(path), &stored); err != nil {
		return record
	}
	if stored.RecordsWithQuery != nil {
		record.RecordsWithQuery = stored.RecordsWithQuery
	}
	record.Records = stored.Records
	return record
}

// Add counts one pick of a result.
func (r *UserSelectedRecord) Add(result Result) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.transformOldRecords()

	r.RecordsWithQuery[QueryAndResultHashCode(result.OriginQuery, result)]++
	r.RecordsWithQuery[ResultHashCode(result)]++
}

// GetSelectedCount is how much a result's score should rise from the times
// it was picked.
func (r *UserSelectedRecord) GetSelectedCount(result Result) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := r.RecordsWithQuery[QueryAndResultHashCode(result.OriginQuery, result)] * 5
	count += r.RecordsWithQuery[ResultHashCode(result)]
	return count
}

// transformOldRecords folds the reference's older string keys into the
// current map, so that counts from an older launcher are not lost.
func (r *UserSelectedRecord) transformOldRecords() {
	if r.Records == nil {
		return
	}
	old := r.Records
	r.Records = nil
	for key, value := range old {
		hash := staticHashCode(key, hashInitial)
		if _, exists := r.RecordsWithQuery[hash]; !exists {
			r.RecordsWithQuery[hash] = value
		}
	}
}

// Save writes the counts out.
func (r *UserSelectedRecord) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	serialized, err := json.MarshalIndent(struct {
		RecordsWithQuery map[int]int    `json:"recordsWithQuery"`
		Records          map[string]int `json:"records,omitempty"`
	}{r.RecordsWithQuery, r.Records}, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(r.path, serialized)
}

// Record is what a pinned result is remembered by.
type Record struct {
	Title     string `json:"Title"`
	SubTitle  string `json:"SubTitle"`
	PluginID  string `json:"PluginID"`
	RecordKey string `json:"RecordKey"`
}

// Equals reports whether a pinned record is a result.
func (r Record) Equals(result Result) bool {
	if r.RecordKey == "" || result.RecordKey == "" {
		return r.Title == result.Title &&
			r.SubTitle == result.SubTitle &&
			r.PluginID == result.PluginID
	}
	return r.RecordKey == result.RecordKey && r.PluginID == result.PluginID
}

// TopMostRecord holds the results the user pinned to the top, per query. A
// query can have several, and the last pinned is the highest.
type TopMostRecord struct {
	// Records maps a query to the results pinned for it, in the order they
	// were pinned.
	Records map[string][]Record `json:"records"`

	path string
	mu   sync.Mutex
}

// LoadTopMostRecord reads the pinned results, or makes an empty set.
func LoadTopMostRecord(path string) *TopMostRecord {
	record := &TopMostRecord{Records: map[string][]Record{}, path: path}

	var stored struct {
		Records map[string][]Record `json:"records"`
	}
	if err := json.Unmarshal(mustReadFile(path), &stored); err != nil {
		return record
	}
	if stored.Records != nil {
		record.Records = stored.Records
	}
	return record
}

// IsTopMost reports whether a result is pinned for its query.
func (r *TopMostRecord) IsTopMost(result Result) bool { return r.GetTopMostIndex(result) != -1 }

// GetTopMostIndex is where a result sits among the pinned ones, or -1 when
// it is not pinned. The most recently pinned has the lowest index, so that
// it scores highest.
func (r *TopMostRecord) GetTopMostIndex(result Result) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if result.OriginQuery == nil {
		return -1
	}
	records, ok := r.Records[result.OriginQuery.TrimmedQuery]
	if !ok {
		return -1
	}

	for index, record := range records {
		if record.Equals(result) {
			return len(records) - 1 - index
		}
	}
	return -1
}

// AddOrUpdate pins a result for its query, replacing the pin it had.
func (r *TopMostRecord) AddOrUpdate(result Result) {
	if result.OriginQuery == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	record := Record{
		Title:     result.Title,
		SubTitle:  result.SubTitle,
		PluginID:  result.PluginID,
		RecordKey: result.RecordKey,
	}

	query := result.OriginQuery.TrimmedQuery
	existing := r.Records[query]

	kept := make([]Record, 0, len(existing)+1)
	for _, candidate := range existing {
		if !candidate.Equals(result) {
			kept = append(kept, candidate)
		}
	}
	r.Records[query] = append(kept, record)
}

// Remove unpins a result.
func (r *TopMostRecord) Remove(result Result) {
	if result.OriginQuery == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	query := result.OriginQuery.TrimmedQuery
	existing, ok := r.Records[query]
	if !ok {
		return
	}

	kept := make([]Record, 0, len(existing))
	for _, candidate := range existing {
		if !candidate.Equals(result) {
			kept = append(kept, candidate)
		}
	}
	if len(kept) == 0 {
		delete(r.Records, query)
	} else {
		r.Records[query] = kept
	}
}

// Save writes the pinned results out.
func (r *TopMostRecord) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	serialized, err := json.MarshalIndent(struct {
		Records map[string][]Record `json:"records"`
	}{r.Records}, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(r.path, serialized)
}

// mustReadFile reads a file, returning nothing when it is missing.
func mustReadFile(path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return content
}

// writeFile writes a file, making its directory.
func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
