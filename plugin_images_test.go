package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
	"launch/plugins/calculator"
	"launch/plugins/explorer"
)

// explorerImages stands in for the pictures a plugin embeds: a plugin's
// own embed is written inside its package, so its paths are "Images/...".
//
//go:embed plugins/explorer/Images/*.png
var explorerImages embed.FS

// pluginImages is those pictures as a plugin sees them.
func pluginImages(t *testing.T) fs.FS {
	t.Helper()

	images, err := fs.Sub(explorerImages, "plugins/explorer")
	if err != nil {
		t.Fatal(err)
	}
	return images
}

func TestAPluginWritesThePicturesItShips(t *testing.T) {
	directory := t.TempDir()

	if err := core.InstallImages(directory, pluginImages(t), "Images/*.png"); err != nil {
		t.Fatal(err)
	}

	icon := filepath.Join(directory, "Images", "file.png")
	content, err := os.ReadFile(icon)
	if err != nil {
		t.Fatalf("the plugin's picture is not where the launcher reads it: %v", err)
	}

	// A picture that cannot be decoded is no better than no picture.
	if _, err := ui.DecodeBitmap(content); err != nil {
		t.Errorf("the picture the plugin wrote cannot be read: %v", err)
	}
}

func TestAPictureThatIsThereIsLeftAlone(t *testing.T) {
	directory := t.TempDir()
	images := filepath.Join(directory, "Images")
	if err := os.MkdirAll(images, 0o755); err != nil {
		t.Fatal(err)
	}
	// A user who replaced a plugin's picture keeps theirs.
	written := []byte("the user's own picture")
	if err := os.WriteFile(filepath.Join(images, "file.png"), written, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := core.InstallImages(directory, pluginImages(t), "Images/*.png"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(images, "file.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(written) {
		t.Errorf("the picture was written over")
	}
	// And the ones that were not there are written.
	if _, err := os.Stat(filepath.Join(images, "folder.png")); err != nil {
		t.Errorf("the other pictures were not written: %v", err)
	}
}

func TestTheCalculatorAndTheExplorerShipTheirPictures(t *testing.T) {
	// Both name pictures in their results, so both must have them to write.
	for name, plugin := range map[string]core.Plugin{"calculator": calculator.New(), "explorer": explorer.New()} {
		metadata := plugin.Metadata()
		if metadata.IcoPath == "" {
			t.Errorf("%s names no icon", name)
		}
		if _, err := os.Stat(filepath.Join("plugins", name, filepath.FromSlash(metadata.IcoPath))); err != nil {
			t.Errorf("%s does not ship %s: %v", name, metadata.IcoPath, err)
		}
	}
}
