package copyq

import "testing"

// TestAZSettingsFileFromAnOlderBuildIsFilledIn is what the user's own file
// was: it said nothing about the hotkey, so pressing Alt+C did nothing.
func TestAZSettingsFileFromAnOlderBuildIsFilledIn(t *testing.T) {
	old := Settings{ActionKeyword: "cb", MaxItems: 200, WatchText: true, WatchFiles: true}

	old.applyDefaults()

	if old.Hotkey != DefaultSettings().Hotkey {
		t.Errorf("the hotkey is %q, want the default %q", old.Hotkey, DefaultSettings().Hotkey)
	}
	defaults := DefaultSettings()
	if old.WatchText != defaults.WatchText || old.WatchFiles != defaults.WatchFiles ||
		old.WatchImages != defaults.WatchImages || old.PasteAfterChoosing != defaults.PasteAfterChoosing {
		t.Errorf("the switches are %v/%v/%v/%v, want the defaults %v/%v/%v/%v",
			old.WatchText, old.WatchFiles, old.WatchImages, old.PasteAfterChoosing,
			defaults.WatchText, defaults.WatchFiles, defaults.WatchImages, defaults.PasteAfterChoosing)
	}
	if old.MaxItems != 200 || old.ActionKeyword != "cb" {
		t.Errorf("what the file did say was changed: %+v", old)
	}
	if old.Version != settingsVersion {
		t.Errorf("the file was not marked as written by this build: %+v", old)
	}

	// A file this build wrote says what its switches are, and what the user
	// turned off stays off.
	current := DefaultSettings()
	current.PasteAfterChoosing = false
	current.WatchImages = false
	current.applyDefaults()
	if current.PasteAfterChoosing || current.WatchImages {
		t.Errorf("a switch the user turned off was turned on: %+v", current)
	}
}
