package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

func TestAKeySettingIsRead(t *testing.T) {
	for _, test := range []struct {
		setting string
		mods    ui.Modifiers
		key     ui.Key
	}{
		{"F1", 0, ui.KeyF1},
		{"Ctrl + Tab", ui.Ctrl, ui.KeyTab},
		{"Ctrl+O", ui.Ctrl, ui.KeyO},
		{"Ctrl + Shift + Tab", ui.Ctrl | ui.Shift, ui.KeyTab},
		{"Alt + Up", ui.Alt, ui.KeyUp},
		{"Shift + Tab", ui.Shift, ui.KeyTab},
		{"PageUp", 0, ui.KeyPageUp},
		{"Ctrl + H", ui.Ctrl, ui.KeyH},
		{"Ctrl + ]", ui.Ctrl, ui.KeyBracketRight},
		{"ctrl+tab", ui.Ctrl, ui.KeyTab},
		{"F12", 0, ui.KeyF12},
	} {
		got := parseHotkey(test.setting)
		if !got.set || got.mods != test.mods || got.key != test.key {
			t.Errorf("parseHotkey(%q) = %+v, want %v+%v", test.setting, got, test.mods, test.key)
		}
	}
}

func TestAKeySettingThatCannotBeReadBindsNothing(t *testing.T) {
	for _, setting := range []string{"", "  ", "Ctrl +", "Nothing", "Ctrl + NotAKey", "+"} {
		if got := parseHotkey(setting); got.set {
			t.Errorf("parseHotkey(%q) = %+v, want nothing bound", setting, got)
		}
	}
}

func TestTheDefaultSettingsBindWhatTheLauncherAlwaysDid(t *testing.T) {
	keys := hotkeysFrom(core.DefaultSettings())

	for _, test := range []struct {
		name string
		mods ui.Modifiers
		key  ui.Key
	}{
		{"preview", 0, ui.KeyF1},
		{"autoComplete", ui.Ctrl, ui.KeyTab},
		{"selectNext", 0, ui.KeyTab},
		{"selectPrev", ui.Shift, ui.KeyTab},
		// The reference's own defaults bind the next page to PageUp, which
		// is what its settings say and what the user can change.
		{"selectNextPage", 0, ui.KeyPageUp},
		{"selectPrevPage", 0, ui.KeyPageDown},
		{"contextMenu", ui.Ctrl, ui.KeyO},
		{"settings", ui.Ctrl, ui.KeyI},
		{"history", ui.Ctrl, ui.KeyH},
		{"historyUp", ui.Alt, ui.KeyUp},
		{"historyDown", ui.Alt, ui.KeyDown},
	} {
		var bound keyBinding
		switch test.name {
		case "preview":
			bound = keys.preview
		case "autoComplete":
			bound = keys.autoComplete[0]
		case "selectNext":
			bound = keys.selectNext[0]
		case "selectPrev":
			bound = keys.selectPrev[0]
		case "selectNextPage":
			bound = keys.selectNextPage
		case "selectPrevPage":
			bound = keys.selectPrevPage
		case "contextMenu":
			bound = keys.contextMenu
		case "settings":
			bound = keys.settings
		case "history":
			bound = keys.history
		case "historyUp":
			bound = keys.historyUp
		case "historyDown":
			bound = keys.historyDown
		}
		if !bound.matches(test.mods, test.key) {
			t.Errorf("%s is bound to %+v, want %v+%v", test.name, bound, test.mods, test.key)
		}
	}

	// The modifier that runs a result is Alt, which is what Alt+1 does.
	if keys.openResult != ui.Alt {
		t.Errorf("a result is run with %v, want Alt", keys.openResult)
	}
}

func TestAKeySettingThatChangedChangesTheKey(t *testing.T) {
	settings := core.DefaultSettings()
	settings.PreviewHotkey = "Ctrl + P"
	settings.SelectNextItemHotkey = "Ctrl + N"
	settings.SelectPrevItemHotkey = "Ctrl + P"
	settings.OpenResultModifiers = "Ctrl"

	keys := hotkeysFrom(settings)

	if !keys.preview.matches(ui.Ctrl, ui.KeyP) || keys.preview.matches(0, ui.KeyF1) {
		t.Errorf("the preview is bound to %+v, want Ctrl+P", keys.preview)
	}
	if !keys.selectNext[0].matches(ui.Ctrl, ui.KeyN) {
		t.Errorf("the next result is bound to %+v, want Ctrl+N", keys.selectNext[0])
	}
	if !keys.selectPrev[0].matches(ui.Ctrl, ui.KeyP) {
		t.Errorf("the previous result is bound to %+v, want Ctrl+P", keys.selectPrev[0])
	}
	if keys.openResult != ui.Ctrl {
		t.Errorf("a result is run with %v, want Ctrl", keys.openResult)
	}
}

func TestTheSecondBindingOfAKeyIsRead(t *testing.T) {
	settings := core.DefaultSettings()
	settings.AutoCompleteHotkey2 = "Alt + Tab"
	settings.SelectNextItemHotkey2 = "Ctrl + J"
	settings.SelectPrevItemHotkey2 = "Ctrl + K"

	keys := hotkeysFrom(settings)

	if !anyOf(keys.autoComplete, ui.Alt, ui.KeyTab) {
		t.Errorf("the second binding of autocomplete was not read: %+v", keys.autoComplete)
	}
	if !anyOf(keys.selectNext, ui.Ctrl, ui.KeyJ) {
		t.Errorf("the second binding of the next result was not read: %+v", keys.selectNext)
	}
	if !anyOf(keys.selectPrev, ui.Ctrl, ui.KeyK) {
		t.Errorf("the second binding of the previous result was not read: %+v", keys.selectPrev)
	}
}

func TestAModifierAloneIsRead(t *testing.T) {
	if got := parseModifiers("Alt"); got != ui.Alt {
		t.Errorf("parseModifiers(Alt) = %v", got)
	}
	if got := parseModifiers("Ctrl + Shift"); got != ui.Ctrl|ui.Shift {
		t.Errorf("parseModifiers(Ctrl + Shift) = %v", got)
	}
	if got := parseModifiers("nothing"); got != 0 {
		t.Errorf("parseModifiers(nothing) = %v, want none", got)
	}
}
