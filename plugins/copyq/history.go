package copyq

import (
	"encoding/binary"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// The history of what the clipboard held, which is what CopyQ keeps: the
// last thing copied is first, the same thing copied twice is not kept
// twice, and the user may pin what they want to keep.
//
// CopyQ keeps a tab per kind of thing it was told to watch, and prunes a
// tab to the number of items the user asked for. This keeps one history of
// everything the clipboard held, which is what a launcher can offer: the
// kind is part of the item, and a search over it finds text, files and
// pictures alike.

// Kind is what an item holds.
type Kind string

const (
	// KindText is text.
	KindText Kind = "text"
	// KindFiles are files, copied in a file manager.
	KindFiles Kind = "files"
	// KindImage is a picture.
	KindImage Kind = "image"
)

// Item is one thing the clipboard held.
type Item struct {
	// Kind is what it holds.
	Kind Kind `json:"Kind"`
	// Text is the text, for text.
	Text string `json:"Text"`
	// Files are the paths, for files.
	Files []string `json:"Files"`
	// Image is the picture as a file inside the plugin's cache, for an
	// image: a picture is too large to keep in the history itself.
	Image string `json:"Image"`
	// Time is when it was copied.
	Time time.Time `json:"Time"`
	// Pinned is true for an item the user keeps: a pinned item is not
	// pruned, and is kept at the front of the history.
	Pinned bool `json:"Pinned"`
}

// Title is what the item is called in a result.
func (i Item) Title() string {
	switch i.Kind {
	case KindFiles:
		if len(i.Files) == 0 {
			return ""
		}
		if len(i.Files) == 1 {
			return baseName(i.Files[0])
		}
		return baseName(i.Files[0]) + " + " + itoa(len(i.Files)-1)
	case KindImage:
		if size := i.imageSize(); size != "" {
			return "Image " + size
		}
		return "Image"
	default:
		return oneLine(i.Text)
	}
}

// imageSize is how big a picture is, as "160×100", which is what tells two
// pictures apart. Empty when the picture cannot be read.
func (i Item) imageSize() string {
	if i.Image == "" {
		return ""
	}
	file, err := os.Open(i.Image)
	if err != nil {
		return ""
	}
	defer file.Close()

	// A PNG says how big it is in its own header: a signature, then the
	// length and the name of the first chunk, then the size.
	header := make([]byte, 24)
	if _, err := io.ReadFull(file, header); err != nil {
		return ""
	}
	if string(header[:8]) != pngSignature {
		return ""
	}
	width := int(binary.BigEndian.Uint32(header[16:20]))
	height := int(binary.BigEndian.Uint32(header[20:24]))
	if width <= 0 || height <= 0 {
		return ""
	}
	return itoa(width) + "×" + itoa(height)
}

// pngSignature starts every PNG file.
const pngSignature = "\x89PNG\r\n\x1a\n"

// Subtitle is what is written under it: where a file is, how long a text
// is, when it was copied.
func (i Item) Subtitle(now time.Time) string {
	when := ago(i.Time, now)

	switch i.Kind {
	case KindFiles:
		if len(i.Files) == 0 {
			return when
		}
		return directoryName(i.Files[0]) + " · " + when
	case KindImage:
		return when
	default:
		rest := i.Text
		if first := strings.IndexByte(rest, '\n'); first >= 0 {
			rest = rest[first+1:]
		} else if len(rest) > len(oneLine(i.Text)) {
			rest = ""
		}
		rest = oneLine(rest)
		if rest == "" {
			return when
		}
		return rest + " · " + when
	}
}

// Search is what the item is found by: its text, or the names of the files
// it holds.
func (i Item) Search() string {
	switch i.Kind {
	case KindFiles:
		names := make([]string, 0, len(i.Files))
		for _, file := range i.Files {
			names = append(names, baseName(file))
		}
		return strings.Join(names, " ")
	case KindImage:
		return "image picture screenshot"
	default:
		return i.Text
	}
}

// History is the items the clipboard held, newest first.
type History struct {
	items []Item
	// limit is how many items are kept, the pinned ones not counted.
	limit int
}

// NewHistory makes a history of at most limit items.
func NewHistory(limit int) *History {
	if limit <= 0 {
		limit = defaultLimit
	}
	return &History{limit: limit}
}

// defaultLimit is how many items are kept when the user asked for none,
// which is what CopyQ keeps by default.
const defaultLimit = 200

// Items are the items, newest first.
func (h *History) Items() []Item { return h.items }

// SetItems fills the history from what was kept, as loading it from disk
// does.
func (h *History) SetItems(items []Item) {
	h.items = items
	h.prune()
}

// Add puts an item at the front, unless the same thing is already the
// front one: copying the same thing twice does not fill the history with
// it, which is what CopyQ does.
func (h *History) Add(item Item) bool {
	if item.Kind == "" {
		return false
	}
	if item.Time.IsZero() {
		item.Time = time.Now()
	}
	// The same thing copied again is not kept twice: it is the newest one,
	// and its time is when it was copied last. CopyQ does the same, so
	// that copying something back and forth does not fill the history.
	for i, kept := range h.items {
		if !sameItem(kept, item) {
			continue
		}
		item.Pinned = kept.Pinned
		h.items = append(h.items[:i], h.items[i+1:]...)
		h.items = append([]Item{item}, h.items...)
		h.prune()
		return false
	}

	h.items = append([]Item{item}, h.items...)
	h.prune()
	return true
}

// sameItem reports whether two items hold the same thing.
func sameItem(a, b Item) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindFiles:
		if len(a.Files) != len(b.Files) {
			return false
		}
		for i := range a.Files {
			if !strings.EqualFold(a.Files[i], b.Files[i]) {
				return false
			}
		}
		return true
	case KindImage:
		return a.Image != "" && a.Image == b.Image
	default:
		return a.Text == b.Text
	}
}

// Pin keeps an item, or lets it go, and answers with the item as it is
// now.
func (h *History) Pin(index int, pinned bool) bool {
	if index < 0 || index >= len(h.items) {
		return false
	}
	h.items[index].Pinned = pinned
	h.prune()
	return true
}

// Remove takes an item out.
func (h *History) Remove(index int) bool {
	if index < 0 || index >= len(h.items) {
		return false
	}
	h.items = append(h.items[:index], h.items[index+1:]...)
	return true
}

// Clear takes everything out but what the user pinned, which is what
// CopyQ's own "clear" leaves.
func (h *History) Clear() {
	kept := make([]Item, 0, len(h.items))
	for _, item := range h.items {
		if item.Pinned {
			kept = append(kept, item)
		}
	}
	h.items = kept
}

// prune keeps the pinned items and the newest of the rest, up to the
// limit.
func (h *History) prune() {
	if len(h.items) == 0 {
		return
	}

	pinned := make([]Item, 0, len(h.items))
	rest := make([]Item, 0, len(h.items))
	for _, item := range h.items {
		if item.Pinned {
			pinned = append(pinned, item)
			continue
		}
		rest = append(rest, item)
	}

	if len(rest) > h.limit {
		rest = rest[:h.limit]
	}
	// Pinned items stay in front, in the order they were pinned; the rest
	// follow newest first.
	h.items = append(pinned, rest...)
}

// Search finds the items a query matches, best first, up to limit.
func (h *History) Search(query string, limit int) []int {
	if limit <= 0 {
		limit = len(h.items)
	}

	found := make([]int, 0, limit)
	for i, item := range h.items {
		if query != "" && !containsFold(item.Search(), query) {
			continue
		}
		found = append(found, i)
		if len(found) == limit {
			break
		}
	}
	return found
}

// SortByTime puts the items in the order they were copied, newest first,
// which is how a history read from disk is shown before anything is
// copied.
func (h *History) SortByTime() {
	sort.SliceStable(h.items, func(i, j int) bool {
		if h.items[i].Pinned != h.items[j].Pinned {
			return h.items[i].Pinned
		}
		return h.items[i].Time.After(h.items[j].Time)
	})
}
