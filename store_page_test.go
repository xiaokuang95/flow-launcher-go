package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// storeReader stands in for the plugin store's server: it answers with a
// list of its own and counts how often it was asked.
type storeReader struct {
	plugins []core.UserPlugin
	reads   int
}

func (r *storeReader) RoundTrip(*http.Request) (*http.Response, error) {
	r.reads++
	body, err := json.Marshal(r.plugins)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}

// storeOf makes a store with a list of its own, and an app over it.
func storeOf(t *testing.T, count int) (*app, *storeReader) {
	t.Helper()

	reader := &storeReader{}
	for i := 0; i < count; i++ {
		reader.plugins = append(reader.plugins, core.UserPlugin{
			ID:          "plugin-" + strconv.Itoa(i),
			Name:        "Plugin " + strconv.Itoa(i),
			Description: "What plugin " + strconv.Itoa(i) + " does",
			Author:      "Someone",
		})
	}

	// The store writes the list it read to its cache file, which it does on
	// the goroutine that read it: the file is put somewhere that outlives
	// the test rather than in the test's own directory, which is taken away
	// as soon as the test is over.
	cache, err := os.CreateTemp("", "flow-launcher-store-*.json")
	if err != nil {
		t.Fatal(err)
	}
	cache.Close()
	os.Remove(cache.Name())

	store := core.NewPluginStore("0.1.0", cache.Name())
	store.Client = &http.Client{Transport: reader}

	a := &app{store: store, settings: core.DefaultSettings()}
	a.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	a.api.Hooks.GetPluginManifest = a.pluginManifest
	a.api.Hooks.UpdateManifest = a.updatePluginManifest
	a.api.Hooks.UpdatePluginManifest = a.updatePluginManifest
	return a, reader
}

func TestTheStorePageDrawsOnlyWhatIsOnTheScreen(t *testing.T) {
	a, _ := storeOf(t, 349)
	waitForStore(t, a)
	t.Cleanup(func() { waitForStore(t, a) })

	window := &settingsWindow{app: a}
	window.page = pageOf(t, "pluginStore")

	ui.NewTester(window.view, 900, 620)

	// 349 plugins in a window of 620 DIPs: only the rows that fit are
	// built, so the frame holds a handful of them rather than all of them.
	drawn := window.storeRowsDrawn()
	if drawn == 0 {
		t.Fatalf("the store page drew no rows of 349")
	}
	if drawn > 60 {
		t.Errorf("the store page drew %d rows of 349 in one frame, want only the ones on the screen", drawn)
	}
	t.Logf("drew %d rows of 349 in the last frame", drawn)
}

// waitForStore waits until the store has been read in the background, which
// is what the window waits for too.
func waitForStore(t *testing.T, a *app) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for len(a.pluginManifest()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the plugin store was never read")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTheStorePageFiltersWhatIsTyped(t *testing.T) {
	a, _ := storeOf(t, 100)
	waitForStore(t, a)
	t.Cleanup(func() { waitForStore(t, a) })
	window := &settingsWindow{app: a}

	manifest := a.pluginManifest()
	if len(manifest) != 100 {
		t.Fatalf("the store has %d plugins, want 100", len(manifest))
	}

	window.storeFilter = "Plugin 42"
	if shown := window.storeRows(manifest); len(shown) != 1 {
		t.Errorf("the filter let %d plugins through, want one", len(shown))
	}
	window.storeFilter = "someone"
	if shown := window.storeRows(manifest); len(shown) != 100 {
		t.Errorf("a filter on the author let %d through, want all of them", len(shown))
	}
	window.storeFilter = "nothing at all"
	if shown := window.storeRows(manifest); len(shown) != 0 {
		t.Errorf("a filter nothing matches let %d through", len(shown))
	}
}

func TestDrawingTheStorePageDoesNotReadTheStore(t *testing.T) {
	a, reader := storeOf(t, 10)
	waitForStore(t, a)
	t.Cleanup(func() { waitForStore(t, a) })

	// What the window draws is what has been read already: reading the
	// store is a network call, and the window is drawn on every keystroke.
	before := reader.reads
	for i := 0; i < 5; i++ {
		_ = a.pluginManifest()
	}
	if reader.reads != before {
		t.Errorf("drawing the store page read the store again: %d reads, want %d", reader.reads, before)
	}
}

func TestReadingTheStoreNeverHoldsTheFrame(t *testing.T) {
	a, reader := storeOf(t, 349)

	// The first call asks for the list and answers at once; the ones after
	// it answer with what is there. None of them waits for the network,
	// since they are called while the window is being drawn.
	deadline := time.Now().Add(20 * time.Second)
	arrived := false
	for !arrived && time.Now().Before(deadline) {
		start := time.Now()
		plugins := a.pluginManifest()
		if took := time.Since(start); took > 100*time.Millisecond {
			t.Fatalf("asking for the store took %.1f ms, want it never to wait for the network", float64(took.Microseconds())/1000)
		}
		arrived = len(plugins) > 0
		time.Sleep(10 * time.Millisecond)
	}
	if !arrived {
		t.Fatalf("the store was never read")
	}
	if reader.reads == 0 {
		t.Errorf("the store was never asked")
	}
}

func TestTwoReadersDoNotBothFetch(t *testing.T) {
	a, reader := storeOf(t, 10)

	// Ten frames in a row, while the first read is still running: the
	// store is asked once.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			_ = a.pluginManifest()
			time.Sleep(5 * time.Millisecond)
		}
	}()
	<-done
	waitForStore(t, a)
	t.Cleanup(func() { waitForStore(t, a) })

	if reader.reads != 1 {
		t.Errorf("the store was asked %d times, want once", reader.reads)
	}
}

// pageOf is the number of the settings page with a name.
func pageOf(t *testing.T, name string) int {
	t.Helper()

	for i, page := range settingsPages {
		if page == name {
			return i
		}
	}
	t.Fatalf("there is no settings page called %q", name)
	return 0
}
