package main

import (
	"context"
	"testing"

	"launch/core"
	"launch/plugins/copyq"
)

// TestTheClipboardHistoryAnswersAQuery runs the plugin through the
// launcher's own engine, which is what typing does: the query keyword
// reaches it, the history answers, and what is chosen is put back on the
// clipboard.
func TestTheClipboardHistoryAnswersAQuery(t *testing.T) {
	a, err := newApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// The clipboard the plugin reads, which the test fills instead of the
	// machine's own.
	clipboard := &fakeClipboardForEngine{content: core.ClipboardContent{Kind: core.ClipboardText, Text: "hello from the clipboard"}}
	pair, ok := a.manager.GetPluginForID("a8e6b0e4c1d94c9f8d3f5a2b7c0e6d41")
	if !ok {
		t.Fatalf("the plugin is not loaded: %v", a.manager.AllLoadedPlugins())
	}
	if plugin, ok := pair.Plugin.(*copyq.Plugin); ok {
		plugin.WatchClipboard(clipboard)
	}

	results := a.engine.Search(context.Background(), "cb")
	if len(results) == 0 {
		t.Fatalf("the query found nothing")
	}
	t.Logf("the query found %d results, the first is %q", len(results), results[0].Title)
	if results[0].Title != "hello from the clipboard" {
		t.Errorf("the first result is %q", results[0].Title)
	}

	results = a.engine.Search(context.Background(), "cb hello")
	if len(results) == 0 || results[0].Title != "hello from the clipboard" {
		t.Errorf("the search found %v", results)
	}
}

type fakeClipboardForEngine struct {
	content core.ClipboardContent
	written []copyq.Item
}

func (f *fakeClipboardForEngine) Read() (core.ClipboardContent, error) { return f.content, nil }
func (f *fakeClipboardForEngine) Write(item copyq.Item) error {
	f.written = append(f.written, item)
	return nil
}
