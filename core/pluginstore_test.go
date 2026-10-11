package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// storeServer stands in for the plugin store, so that a test reads a list
// of its own rather than the project's own.
func storeServer(t *testing.T, plugins []UserPlugin) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(plugins)
	}))
	t.Cleanup(server.Close)
	return server
}

// withStoreURL points the store at a server for one test.
func withStoreURL(t *testing.T, address string) {
	t.Helper()
	original := pluginStoreURLs
	pluginStoreURLs = []string{address}
	t.Cleanup(func() { pluginStoreURLs = original })
}

func TestStoreReadsTheList(t *testing.T) {
	server := storeServer(t, []UserPlugin{
		{ID: "a", Name: "Alpha", Language: "python", Version: "1.0"},
		{ID: "b", Name: "Beta", Language: "csharp", Version: "2.0"},
	})
	withStoreURL(t, server.URL)

	store := NewPluginStore("0.1.0", "")
	plugins, err := store.Plugins(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want both", len(plugins))
	}
}

func TestStoreLeavesOutWhatTheLauncherIsTooOldFor(t *testing.T) {
	server := storeServer(t, []UserPlugin{
		{ID: "a", Name: "Alpha", Version: "1.0"},
		{ID: "b", Name: "Beta", Version: "1.0", MinimumAppVersion: "9.0.0"},
		{ID: "c", Name: "Gamma", Version: "1.0", MinimumAppVersion: "0.1.0"},
	})
	withStoreURL(t, server.URL)

	store := NewPluginStore("1.0.0", "")
	plugins, err := store.Plugins(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		names = append(names, plugin.Name)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %v, want the two the launcher is new enough for", names)
	}
	for _, name := range names {
		if name == "Beta" {
			t.Errorf("the plugin that needs a newer launcher was offered: %v", names)
		}
	}
}

func TestStoreKeepsWhatItRead(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "manifest.json")

	server := storeServer(t, []UserPlugin{{ID: "a", Name: "Alpha", Version: "1.0"}})
	withStoreURL(t, server.URL)

	store := NewPluginStore("1.0.0", path)
	if _, err := store.Plugins(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if !FileExists(path) {
		t.Fatal("the list was not kept")
	}

	// A launcher that cannot reach the store reads what was kept.
	server.Close()
	offline := NewPluginStore("1.0.0", path)
	plugins, err := offline.Plugins(context.Background(), true)
	if err != nil {
		t.Fatalf("reading what was kept failed: %v", err)
	}
	if len(plugins) != 1 || plugins[0].Name != "Alpha" {
		t.Errorf("the list read offline is %v", plugins)
	}
}

func TestVersionSatisfied(t *testing.T) {
	cases := []struct {
		required string
		version  string
		want     bool
	}{
		{"", "1.0.0", true},
		{"1.0.0", "1.0.0", true},
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "1.9.9", false},
		{"1.2.0", "1.10.0", true},
		{"1.10.0", "1.9.0", false},
	}
	for _, c := range cases {
		if got := versionSatisfied(c.required, c.version); got != c.want {
			t.Errorf("versionSatisfied(%q, %q) = %v, want %v", c.required, c.version, got, c.want)
		}
	}
}

func TestUnzipKeepsFilesInsideTheFolder(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "plugin.zip")
	if err := writePluginFile(archive, "Test Plugin", "test-id"); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(directory, "Plugin")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Unzip(archive, destination); err != nil {
		t.Fatal(err)
	}
	if !FileExists(filepath.Join(destination, "plugin.json")) {
		t.Error("the plugin's description was not unpacked")
	}
}

func TestUnzipRefusesAPathOutsideTheFolder(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "bad.zip")

	// A zip entry that names a path above the folder it is unpacked into.
	if err := writeZipWithEntry(archive, "../escaped.txt", "no"); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(directory, "Plugin")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Unzip(archive, destination); err == nil {
		t.Error("a file that would be written outside the folder was allowed")
	}
	if FileExists(filepath.Join(directory, "escaped.txt")) {
		t.Error("the file was written outside the folder")
	}
}
