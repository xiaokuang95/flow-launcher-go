package core

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

// The launcher shows a plugin's own settings and changes them, which is
// what the reference does with a settings window per plugin. A plugin that
// wants that hands over its settings (SettingProviderPlugin.Settings) and
// the window is built from the fields of that value: a switch for a bool, a
// field for a string, a number for an int, and a switch for each entry of a
// map of bools. What it cannot show — a list of records, a time — is left
// to the plugin to change in its own way.

// SettingFieldKind is what a settings field is changed with.
type SettingFieldKind int

const (
	// SettingSwitch is a bool, or each entry of a map of bools.
	SettingSwitch SettingFieldKind = iota
	// SettingText is a string.
	SettingText
	// SettingNumber is a number.
	SettingNumber
	// SettingChoice is one of a set of strings.
	SettingChoice
)

// SettingField is one field of a plugin's settings, as the window shows it.
type SettingField struct {
	// Group is what the field belongs to, which is the path of the field
	// inside the settings, so that the window can put them together.
	Group string
	// Key is the JSON name of the field, which is what it is saved as.
	Key string
	// Label is what the field is called.
	Label string
	// Kind is what it is changed with.
	Kind SettingFieldKind
	// Choices are the values a choice offers.
	Choices []string

	// value is the field itself, which a change is written to.
	value reflect.Value
	// mapValue and mapKey are the map and the entry a change is written to,
	// which is how the entries of a map are changed: an entry of a map
	// cannot be written to, the map itself has to be.
	mapValue reflect.Value
	mapKey   string
}

// Value is what the field holds now.
func (f SettingField) Value() any {
	switch f.value.Kind() {
	case reflect.Bool:
		return f.value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(f.value.Int())
	case reflect.Float32, reflect.Float64:
		return f.value.Float()
	case reflect.String:
		return f.value.String()
	}
	return nil
}

// Set writes a new value to the field.
func (f SettingField) Set(value any) error {
	switch f.value.Kind() {
	case reflect.Bool:
		boolean, ok := value.(bool)
		if !ok {
			return fmt.Errorf("%s is a switch", f.Key)
		}
		if f.mapValue.IsValid() {
			f.mapValue.SetMapIndex(reflect.ValueOf(f.mapKey), reflect.ValueOf(boolean))
			return nil
		}
		f.value.SetBool(boolean)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		number, ok := asInt(value)
		if !ok {
			return fmt.Errorf("%s is a number", f.Key)
		}
		f.value.SetInt(int64(number))
	case reflect.Float32, reflect.Float64:
		number, ok := asFloat(value)
		if !ok {
			return fmt.Errorf("%s is a number", f.Key)
		}
		f.value.SetFloat(number)
	case reflect.String:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s is text", f.Key)
		}
		f.value.SetString(text)
	default:
		return fmt.Errorf("%s cannot be changed from here", f.Key)
	}
	return nil
}

// asInt reads a number that may have come from a field, where it is a
// float, or from a switch, where it is not a number at all.
func asInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int64:
		return int(number), true
	case float64:
		return int(number), true
	}
	return 0, false
}

func asFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case float64:
		return number, true
	}
	return 0, false
}

// SettingFields are the fields of a plugin's settings, in the order they
// are shown.
func SettingFields(settings any) []SettingField {
	if settings == nil {
		return nil
	}
	value := reflect.ValueOf(settings)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	return fieldsOf(value, "")
}

// fieldsOf reads one struct's fields, and the fields of the structs inside
// it, which is what a plugin's settings are made of.
func fieldsOf(value reflect.Value, group string) []SettingField {
	var fields []SettingField
	kind := value.Type()

	for i := 0; i < kind.NumField(); i++ {
		field := kind.Field(i)
		if !field.IsExported() {
			continue
		}
		key := jsonName(field)
		if key == "" || key == "-" {
			continue
		}

		held := value.Field(i)
		switch held.Kind() {
		case reflect.Struct:
			// A struct of its own is a group of settings, unless it is a
			// time or something else the window cannot show.
			if _, ok := held.Interface().(interface{ IsZero() bool }); ok {
				continue
			}
			fields = append(fields, fieldsOf(held, join(group, key))...)
		case reflect.Map:
			if held.Type().Elem().Kind() != reflect.Bool {
				continue
			}
			fields = append(fields, mapFields(held, group)...)
		case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16,
			reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
			fields = append(fields, SettingField{
				Group: group,
				Key:   key,
				Label: settingLabel(key),
				Kind:  kindOf(held),
				value: held,
			})
		}
	}
	return fields
}

// mapFields are the entries of a map of switches, which is how the file
// types a program is looked for by are held. A map's entries are not
// addressable, so each field holds the map and its key instead.
func mapFields(value reflect.Value, group string) []SettingField {
	keys := make([]string, 0, value.Len())
	for _, key := range value.MapKeys() {
		if key.Kind() == reflect.String {
			keys = append(keys, key.String())
		}
	}
	sort.Strings(keys)

	fields := make([]SettingField, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, SettingField{
			Group: group,
			Key:   key,
			Label: settingLabel(key),
			Kind:  SettingSwitch,
			value: value.MapIndex(reflect.ValueOf(key)),
			// The map and the key are what a change needs, since the entry
			// itself cannot be written to.
			mapValue: value,
			mapKey:   key,
		})
	}
	return fields
}

// jsonName is the name a field is saved under.
func jsonName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

// kindOf is what a field of a settings value is changed with.
func kindOf(value reflect.Value) SettingFieldKind {
	switch value.Kind() {
	case reflect.Bool:
		return SettingSwitch
	case reflect.String:
		return SettingText
	default:
		return SettingNumber
	}
}

// settingLabel is what a field is called: the words of its name, with the
// capitals as the spaces between them. A plugin that wants to say it in the
// user's own language gives the field a name that is a translation key.
func settingLabel(key string) string {
	runes := []rune(key)

	var words []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if !unicode.IsUpper(runes[i]) {
			continue
		}
		// A capital starts a word when a small letter is before it, or
		// when a small letter follows a run of capitals, as in PATHValue.
		afterSmall := unicode.IsLower(runes[i-1])
		beforeSmall := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		if afterSmall || beforeSmall {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	words = append(words, string(runes[start:]))

	for i, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// join is the path of a group inside the settings.
func join(group, key string) string {
	if group == "" {
		return key
	}
	return group + "." + key
}
