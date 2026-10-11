package core

import "testing"

// sampleSettings is a plugin's settings, with the kinds of field the window
// knows how to show.
type sampleSettings struct {
	Enabled    bool              `json:"Enabled"`
	Name       string            `json:"Name"`
	Limit      int               `json:"Limit"`
	Ratio      float64           `json:"Ratio"`
	Suffixes   map[string]bool   `json:"Suffixes"`
	Sources    []string          `json:"Sources"`
	unexported string            //nolint:unused // it is here to be left out
	Nested     nestedSettings    `json:"Nested"`
	Ignored    string            `json:"-"`
	NoTag      string            // read by its own name
	hidden     map[string]bool   //nolint:unused // and so is this
	ExtraField map[string]string `json:"Extra"`
}

type nestedSettings struct {
	Deep bool `json:"Deep"`
}

// fieldNamed finds one field of the form by its key.
func fieldNamed(t *testing.T, fields []SettingField, key string) SettingField {
	t.Helper()

	for _, field := range fields {
		if field.Key == key {
			return field
		}
	}
	t.Fatalf("the form has no field called %q: %v", key, keysOf(fields))
	return SettingField{}
}

func keysOf(fields []SettingField) []string {
	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.Key)
	}
	return keys
}

func TestASettingsValueBecomesAForm(t *testing.T) {
	settings := &sampleSettings{
		Enabled:  true,
		Name:     "chrome",
		Limit:    10,
		Suffixes: map[string]bool{"exe": true, "lnk": false},
		Sources:  []string{"a", "b"},
		Nested:   nestedSettings{Deep: true},
	}

	fields := SettingFields(settings)

	// The kinds the window knows how to show.
	if got := fieldNamed(t, fields, "Enabled"); got.Kind != SettingSwitch || got.Value() != true {
		t.Errorf("Enabled is %v/%v, want a switch that is on", got.Kind, got.Value())
	}
	if got := fieldNamed(t, fields, "Name"); got.Kind != SettingText || got.Value() != "chrome" {
		t.Errorf("Name is %v/%v, want text", got.Kind, got.Value())
	}
	if got := fieldNamed(t, fields, "Limit"); got.Kind != SettingNumber || got.Value() != 10 {
		t.Errorf("Limit is %v/%v, want a number", got.Kind, got.Value())
	}
	// A field of a struct inside the settings is a setting of its own.
	if got := fieldNamed(t, fields, "Deep"); got.Kind != SettingSwitch || got.Value() != true {
		t.Errorf("Deep is %v/%v, want the nested switch", got.Kind, got.Value())
	}
	if got := fieldNamed(t, fields, "NoTag"); got.Key != "NoTag" {
		t.Errorf("a field without a tag is read by its own name")
	}

	// The entries of a map of switches are switches of their own.
	if got := fieldNamed(t, fields, "exe"); got.Kind != SettingSwitch || got.Value() != true {
		t.Errorf("exe is %v/%v, want a switch that is on", got.Kind, got.Value())
	}
	if got := fieldNamed(t, fields, "lnk"); got.Value() != false {
		t.Errorf("lnk is %v, want a switch that is off", got.Value())
	}

	// What the window cannot show is left out rather than shown wrongly.
	for _, unwanted := range []string{"Sources", "Extra", "Ignored", "-", "unexported"} {
		for _, field := range fields {
			if field.Key == unwanted {
				t.Errorf("the form holds %q, which cannot be shown", unwanted)
			}
		}
	}
}

func TestAFormChangeIsWrittenToTheSettings(t *testing.T) {
	settings := &sampleSettings{Suffixes: map[string]bool{"exe": true}}

	fields := SettingFields(settings)
	fieldNamed(t, fields, "Enabled").Set(true)
	fieldNamed(t, fields, "Name").Set("notepad")
	fieldNamed(t, fields, "Limit").Set(5)
	fieldNamed(t, fields, "exe").Set(false)

	if !settings.Enabled {
		t.Errorf("the switch was not written")
	}
	if settings.Name != "notepad" {
		t.Errorf("the name is %q, want what was written", settings.Name)
	}
	if settings.Limit != 5 {
		t.Errorf("the limit is %d, want what was written", settings.Limit)
	}
	if settings.Suffixes["exe"] {
		t.Errorf("the entry of the map is %v, want what was written", settings.Suffixes["exe"])
	}
}

func TestAFieldSaysWhatItCannotTake(t *testing.T) {
	settings := &sampleSettings{}
	fields := SettingFields(settings)

	if err := fieldNamed(t, fields, "Limit").Set("a word"); err == nil {
		t.Errorf("a number took a word")
	}
	if err := fieldNamed(t, fields, "Enabled").Set("yes"); err == nil {
		t.Errorf("a switch took a word")
	}
}

func TestASettingsValueTheWindowCannotShow(t *testing.T) {
	if fields := SettingFields(nil); fields != nil {
		t.Errorf("a plugin without settings has a form of %v", keysOf(fields))
	}
	if fields := SettingFields("a string"); fields != nil {
		t.Errorf("a string is not a form: %v", keysOf(fields))
	}
	var nothing *sampleSettings
	if fields := SettingFields(nothing); fields != nil {
		t.Errorf("a settings value that is not there is not a form: %v", keysOf(fields))
	}
}

func TestAFieldIsCalledAfterItsName(t *testing.T) {
	for _, test := range []struct{ key, want string }{
		{"EnableDescription", "Enable Description"},
		{"HideUninstallers", "Hide Uninstallers"},
		{"exe", "Exe"},
		{"appref-ms", "Appref-ms"},
		{"PATH", "PATH"},
	} {
		if got := settingLabel(test.key); got != test.want {
			t.Errorf("settingLabel(%q) = %q, want %q", test.key, got, test.want)
		}
	}
}
