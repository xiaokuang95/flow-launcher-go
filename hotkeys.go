package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"launch/core"
)

// The keys the launcher binds while it is up are the user's to choose, as
// the reference's hotkey page lets them be: a setting names one the way the
// reference writes it, "Ctrl + Tab" or "F1", and what is pressed is
// compared with it.

// keyBinding is a key and the modifiers it is pressed with.
type keyBinding struct {
	mods ui.Modifiers
	key  ui.Key
	// set is false for a setting that is empty or cannot be read, which
	// binds nothing rather than binding something by accident.
	set bool
}

// matches reports whether a key press is the one bound.
func (b keyBinding) matches(mods ui.Modifiers, key ui.Key) bool {
	return b.set && b.mods == mods && b.key == key
}

// hotkeys are the keys the launcher answers to, read from the settings.
type hotkeys struct {
	preview        keyBinding
	autoComplete   [2]keyBinding
	selectNext     [2]keyBinding
	selectPrev     [2]keyBinding
	selectNextPage keyBinding
	selectPrevPage keyBinding
	contextMenu    keyBinding
	settings       keyBinding
	history        keyBinding
	historyUp      keyBinding
	historyDown    keyBinding
	// openResult is the modifier that runs a result: Ctrl+1 runs the first
	// one when it is Ctrl.
	openResult ui.Modifiers
}

// hotkeysFrom reads the keys the settings bind.
func hotkeysFrom(settings *core.Settings) hotkeys {
	keys := hotkeys{
		preview:        parseHotkey(settings.PreviewHotkey),
		selectNextPage: parseHotkey(settings.SelectNextPageHotkey),
		selectPrevPage: parseHotkey(settings.SelectPrevPageHotkey),
		contextMenu:    parseHotkey(settings.OpenContextMenuHotkey),
		settings:       parseHotkey(settings.SettingWindowHotkey),
		history:        parseHotkey(settings.OpenHistoryHotkey),
		historyUp:      parseHotkey(settings.CycleHistoryUpHotkey),
		historyDown:    parseHotkey(settings.CycleHistoryDownHotkey),
	}
	keys.autoComplete[0] = parseHotkey(settings.AutoCompleteHotkey)
	keys.autoComplete[1] = parseHotkey(settings.AutoCompleteHotkey2)
	keys.selectNext[0] = parseHotkey(settings.SelectNextItemHotkey)
	keys.selectNext[1] = parseHotkey(settings.SelectNextItemHotkey2)
	keys.selectPrev[0] = parseHotkey(settings.SelectPrevItemHotkey)
	keys.selectPrev[1] = parseHotkey(settings.SelectPrevItemHotkey2)
	keys.openResult = parseModifiers(settings.OpenResultModifiers)
	return keys
}

// any reports whether one of a pair of bindings is the key pressed, which
// is what a key the user may bind twice asks.
func anyOf(bindings [2]keyBinding, mods ui.Modifiers, key ui.Key) bool {
	return bindings[0].matches(mods, key) || bindings[1].matches(mods, key)
}

// keyNames are the keys a hotkey setting may name, by the words the
// reference writes them with.
var keyNames = func() map[string]ui.Key {
	names := map[string]ui.Key{
		"enter":     ui.KeyEnter,
		"return":    ui.KeyEnter,
		"escape":    ui.KeyEscape,
		"esc":       ui.KeyEscape,
		"tab":       ui.KeyTab,
		"space":     ui.KeySpace,
		"backspace": ui.KeyBackspace,
		"delete":    ui.KeyDelete,
		"del":       ui.KeyDelete,
		"insert":    ui.KeyInsert,
		"home":      ui.KeyHome,
		"end":       ui.KeyEnd,
		"pageup":    ui.KeyPageUp,
		"pagedown":  ui.KeyPageDown,
		"pgup":      ui.KeyPageUp,
		"pgdn":      ui.KeyPageDown,
		"left":      ui.KeyLeft,
		"right":     ui.KeyRight,
		"up":        ui.KeyUp,
		"down":      ui.KeyDown,
		"minus":     ui.KeyMinus,
		"equal":     ui.KeyEqual,
		"plus":      ui.KeyEqual,
		"comma":     ui.KeyComma,
		"period":    ui.KeyPeriod,
		"slash":     ui.KeySlash,
		"semicolon": ui.KeySemicolon,
		"quote":     ui.KeyQuote,
		"backslash": ui.KeyBackslash,
		"`":         ui.KeyBackquote,
		"backquote": ui.KeyBackquote,
		"lbracket":  ui.KeyBracketLeft,
		"rbracket":  ui.KeyBracketRight,
		"[":         ui.KeyBracketLeft,
		"]":         ui.KeyBracketRight,
	}

	for i := 0; i < 12; i++ {
		names["f"+itoa(i+1)] = ui.KeyF1 + ui.Key(i)
	}
	for r := 'a'; r <= 'z'; r++ {
		names[string(r)] = ui.KeyA + ui.Key(r-'a')
	}
	for d := 0; d <= 9; d++ {
		names[itoa(d)] = ui.Key0 + ui.Key(d)
	}
	return names
}()

// parseHotkey reads a hotkey setting, which is a key and, before it, the
// modifiers it is held with, as the reference writes them: "Ctrl + Tab",
// "Alt+Up", "Shift + F1", "F1".
func parseHotkey(setting string) keyBinding {
	var binding keyBinding

	parts := strings.FieldsFunc(setting, func(r rune) bool {
		return r == '+' || r == ' ' || r == '\t'
	})
	if len(parts) == 0 {
		return binding
	}

	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if mods, ok := modifierNames[name]; ok {
			binding.mods |= mods
			continue
		}
		key, ok := keyNames[name]
		if !ok {
			// A setting this launcher cannot read binds nothing, rather
			// than binding the wrong key.
			return keyBinding{}
		}
		binding.key = key
	}

	if binding.key == ui.KeyUnknown {
		return keyBinding{}
	}
	binding.set = true
	return binding
}

// modifierNames are the words a modifier is written with.
var modifierNames = map[string]ui.Modifiers{
	"ctrl":    ui.Ctrl,
	"control": ui.Ctrl,
	"ctl":     ui.Ctrl,
	"cmd":     ui.Cmd,
	"alt":     ui.Alt,
	"shift":   ui.Shift,
	"win":     ui.Super,
	"windows": ui.Super,
	"super":   ui.Super,
	"meta":    ui.Super,
}

// parseModifiers reads a setting that is modifiers alone, such as the one
// that says which modifier runs a result.
func parseModifiers(setting string) ui.Modifiers {
	var mods ui.Modifiers
	for _, part := range strings.FieldsFunc(setting, func(r rune) bool {
		return r == '+' || r == ' ' || r == '\t'
	}) {
		if modifier, ok := modifierNames[strings.ToLower(strings.TrimSpace(part))]; ok {
			mods |= modifier
		}
	}
	return mods
}

// itoa writes a small number, which is all a key name needs.
func itoa(number int) string {
	if number < 10 {
		return string(rune('0' + number))
	}
	return string(rune('0'+number/10)) + string(rune('0'+number%10))
}
