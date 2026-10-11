package core

import (
	"embed"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// The launcher's own translations, taken from the reference's Languages
// directory. Each file is a XAML resource dictionary of string keys.
//
//go:embed languages/*.xaml
var languageFiles embed.FS

// Language is one of the languages the launcher speaks.
type Language struct {
	// Code is the language's tag, as the settings file holds it.
	Code string
	// Display is its name in its own language.
	Display string
}

// SystemLanguageCode is the setting that follows the system's language.
const SystemLanguageCode = "system"

// AvailableLanguages are the languages the launcher can be set to, in the
// order the settings window shows them.
var AvailableLanguages = []Language{
	{"en", "English"},
	{"zh-cn", "中文"},
	{"zh-tw", "中文（繁体）"},
	{"uk-UA", "Українська"},
	{"ru", "Русский"},
	{"fr", "Français"},
	{"ja", "日本語"},
	{"nl", "Dutch"},
	{"pl", "Polski"},
	{"da", "Dansk"},
	{"de", "Deutsch"},
	{"ko", "한국어"},
	{"sr", "Srpski"},
	{"sr-Cyrl-RS", "Српски"},
	{"pt-pt", "Português"},
	{"pt-br", "Português (Brasil)"},
	{"es", "Spanish"},
	{"es-419", "Spanish (Latin America)"},
	{"it", "Italiano"},
	{"nb-NO", "Norsk Bokmål"},
	{"sk", "Slovenčina"},
	{"tr", "Türkçe"},
	{"cs", "čeština"},
	{"ar", "اللغة العربية"},
	{"vi-vn", "Tiếng Việt"},
	{"he", "עברית"},
}

// systemLanguageNames is what each language calls "System", for the entry
// that follows the system's language.
var systemLanguageNames = map[string]string{
	"en": "System", "zh-cn": "系统", "zh-tw": "系統", "uk-UA": "Система",
	"ru": "Система", "fr": "Système", "ja": "システム", "nl": "Systeem",
	"pl": "System", "da": "System", "de": "System", "ko": "시스템",
	"sr": "Sistem", "sr-Cyrl-RS": "Систем", "pt-pt": "Sistema", "pt-br": "Sistema",
	"es": "Sistema", "es-419": "Sistema", "it": "Sistema", "nb-NO": "System",
	"sk": "Systém", "tr": "Sistem", "cs": "Systém", "ar": "النظام",
	"vi-vn": "Hệ thống", "he": "מערכת",
}

// DefaultLanguage is the language used when nothing else matches.
const DefaultLanguage = "en"

// SystemLanguageName is what a language calls the system language.
func SystemLanguageName(code string) string {
	if name, ok := systemLanguageNames[code]; ok {
		return name
	}
	return "System"
}

// LanguageCodeForCulture maps a culture name such as "zh-Hans-CN" to one of
// the languages the launcher speaks, as the reference matches a two-letter,
// a three-letter or the whole name.
func LanguageCodeForCulture(culture string) string {
	culture = strings.ReplaceAll(culture, "_", "-")
	parts := strings.Split(culture, "-")
	twoLetter := strings.ToLower(parts[0])
	threeLetter := ""
	if len(parts) > 1 {
		threeLetter = twoLetter + "-" + strings.ToLower(parts[1])
	}

	for _, language := range AvailableLanguages {
		code := strings.ToLower(language.Code)
		if code == twoLetter || code == threeLetter || code == strings.ToLower(culture) {
			return language.Code
		}
	}
	// Chinese is written with a script tag in some cultures.
	if twoLetter == "zh" {
		if strings.Contains(strings.ToLower(culture), "tw") || strings.Contains(strings.ToLower(culture), "hant") {
			return "zh-tw"
		}
		return "zh-cn"
	}
	return DefaultLanguage
}

// translations caches the parsed language files.
var translations = struct {
	sync.Mutex
	byLanguage map[string]map[string]string
	current    string
}{byLanguage: map[string]map[string]string{}, current: DefaultLanguage}

// SetLanguage sets the language the interface is written in.
func SetLanguage(code string) {
	if code == SystemLanguageCode || code == "" {
		code = LanguageCodeForCulture(SystemLocale())
	}
	if _, ok := languageByCode(code); !ok {
		code = DefaultLanguage
	}

	translations.Lock()
	translations.current = code
	translations.Unlock()
}

// CurrentLanguage is the language the interface is written in.
func CurrentLanguage() string {
	translations.Lock()
	defer translations.Unlock()
	return translations.current
}

func languageByCode(code string) (Language, bool) {
	for _, language := range AvailableLanguages {
		if strings.EqualFold(language.Code, code) {
			return language, true
		}
	}
	return Language{}, false
}

// T translates a key of the interface's language. An unknown key is
// returned as it is, so that a missing translation shows rather than
// disappears.
func T(language, key string) string {
	if language == "" || language == SystemLanguageCode {
		language = CurrentLanguage()
	}
	table := translationTable(language)
	if value, ok := table[key]; ok {
		return value
	}
	if language != DefaultLanguage {
		if value, ok := translationTable(DefaultLanguage)[key]; ok {
			return value
		}
	}
	return key
}

// Tf translates a key and fills its placeholders, which the reference
// writes as {0}, {1} and so on.
func Tf(language, key string, args ...string) string {
	text := T(language, key)
	for i, arg := range args {
		text = strings.ReplaceAll(text, "{"+itoa(i)+"}", arg)
	}
	return text
}

// translationTable parses a language's file once and keeps it.
func translationTable(language string) map[string]string {
	translations.Lock()
	if table, ok := translations.byLanguage[language]; ok {
		translations.Unlock()
		return table
	}
	translations.Unlock()

	table := map[string]string{}
	for _, name := range languageFileNames(language) {
		parsed, err := parseLanguageFile("languages/" + name + ".xaml")
		if err != nil || len(parsed) == 0 {
			continue
		}
		// A language may be written over two files; the later one wins.
		for key, value := range parsed {
			table[key] = value
		}
	}

	translations.Lock()
	translations.byLanguage[language] = table
	translations.Unlock()
	return table
}

// languageFileNames is the files a language is read from, in order: its
// whole tag, then the language without its region. The reference looks the
// whole tag up alone, so its Vietnamese file, which is named for the
// language rather than for the tag, never loads; looking the language up
// as well is what makes it show.
func languageFileNames(language string) []string {
	names := []string{language}
	if base, _, found := strings.Cut(language, "-"); found && base != language {
		names = append(names, base)
	}
	return names
}

// parseLanguageFile reads a XAML resource dictionary of string keys, from
// the embedded files or from a plugin's own Languages directory.
func parseLanguageFile(name string) (map[string]string, error) {
	file, err := languageFiles.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseLanguageDictionary(file)
}

// ParseLanguageDictionary reads a XAML resource dictionary of string keys
// from a reader, which is how a plugin's own language files are read.
func ParseLanguageDictionary(reader io.Reader) (map[string]string, error) {
	return parseLanguageDictionary(reader)
}

// parseLanguageDictionary reads `<system:String x:Key="...">text</...>`
// entries, whose text may run over several lines.
func parseLanguageDictionary(reader io.Reader) (map[string]string, error) {
	decoder := xml.NewDecoder(reader)
	// The files are written in UTF-8, with a BOM on some of them.
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}

	table := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return table, err
		}

		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "String" {
			continue
		}

		var key string
		for _, attribute := range start.Attr {
			if attribute.Name.Local == "Key" {
				key = attribute.Value
			}
		}
		if key == "" {
			continue
		}

		value, err := readElementText(decoder, start)
		if err != nil {
			return table, err
		}
		table[key] = value
	}

	return table, nil
}

// readElementText reads the text of an element to its end, and returns it
// without the indentation the resource files wrap long strings in.
func readElementText(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return text.String(), err
		}
		switch element := token.(type) {
		case xml.CharData:
			text.Write([]byte(element))
		case xml.StartElement:
			// A nested element's text is not part of the value.
			if _, err := readElementText(decoder, element); err != nil {
				return text.String(), err
			}
		case xml.EndElement:
			if element.Name == start.Name {
				return normalizeLanguageValue(text.String()), nil
			}
		}
	}
}

// normalizeLanguageValue trims the wrapping whitespace the resource files
// add around a value, and folds the runs of spaces inside it, so that a
// sentence written over several lines reads as one.
func normalizeLanguageValue(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	// The first line's indentation is the value's own, the rest is the
	// file's, so only the first line decides how much to trim.
	joined := ""
	for i, line := range lines {
		if i == 0 {
			joined = strings.TrimLeft(line, " \t")
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if joined != "" && !strings.HasSuffix(joined, " ") {
			joined += " "
		}
		joined += strings.TrimSpace(line)
	}
	return strings.TrimSpace(joined)
}

// LoadPluginLanguage reads a plugin's own language file for a language,
// falling back to English and then to the plugin's default file.
func LoadPluginLanguage(pluginDirectory, language string) map[string]string {
	for _, candidate := range []string{language, DefaultLanguage, "en-US"} {
		path := filepath.Join(pluginDirectory, "Languages", candidate+".xaml")
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		table, err := parseLanguageDictionary(file)
		file.Close()
		if err == nil && len(table) > 0 {
			return table
		}
	}
	return map[string]string{}
}

// PluginT translates a key with a plugin's own language file.
func PluginT(pluginDirectory, language, key string) string {
	table := LoadPluginLanguage(pluginDirectory, language)
	if value, ok := table[key]; ok {
		return value
	}
	return key
}

// itoa is strconv.Itoa without the import, for the placeholder helper.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

// LanguageKeys are the keys a language translates, sorted.
func LanguageKeys(language string) []string {
	table := translationTable(language)
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	return keys
}
