package windowssettings

import (
	"embed"
	"encoding/xml"
	"io"
	"strings"
	"sync"

	"launch/core"
)

// The words of the settings pages, which the reference keeps in the
// resources of its own assembly: one file for each language, and the file
// without a language in its name holding English.
//
//go:embed Resources/*.resx
var resources embed.FS

// translator reads the words of a settings page in the language the
// launcher is set to.
type translator struct {
	mu     sync.Mutex
	tables map[string]map[string]string
}

// newTranslator makes a translator with nothing read yet.
func newTranslator() *translator {
	return &translator{tables: map[string]map[string]string{}}
}

// t is the word for a key, in the launcher's language, falling back to
// English and then to the key itself.
func (t *translator) t(key string) string {
	if key == "" {
		return ""
	}
	if value, ok := t.table(core.CurrentLanguage())[key]; ok && value != "" {
		return value
	}
	if value, ok := t.table("en")[key]; ok && value != "" {
		return value
	}
	return key
}

// table reads one language's file, once.
func (t *translator) table(language string) map[string]string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if table, ok := t.tables[language]; ok {
		return table
	}

	// The language's own words are read first, and English fills in only
	// what it does not say: the reference's files are not all complete,
	// and a word missing from one is shown in English rather than as its
	// own key.
	table := readResourceFiles(resourceFileNames(language))
	for key, value := range readResourceFiles(englishResourceFile) {
		if _, said := table[key]; !said {
			table[key] = value
		}
	}

	t.tables[language] = table
	return table
}

// englishResourceFile holds the words every other language falls back to.
var englishResourceFile = []string{"Resources.resx"}

// readResourceFiles reads the files named, each filling in what the ones
// before it did not say.
func readResourceFiles(names []string) map[string]string {
	table := map[string]string{}
	for _, name := range names {
		file, err := resources.Open("Resources/" + name)
		if err != nil {
			continue
		}
		parsed, err := parseResourceFile(file)
		file.Close()
		if err != nil {
			continue
		}
		for key, value := range parsed {
			table[key] = value
		}
	}
	return table
}

// resourceFileNames are the files a language is read from, in order: the
// one named for the language, and the one named for it as .NET writes a
// region.
func resourceFileNames(language string) []string {
	names := []string{}
	lower := strings.ToLower(language)

	for _, candidate := range []string{language, lower, base(lower)} {
		if candidate == "" {
			continue
		}
		for _, form := range regionForms(candidate) {
			names = append(names, "Resources."+form+".resx")
		}
	}
	return names
}

// regionForms are the ways a language and its region are written in a file
// name.
func regionForms(language string) []string {
	forms := []string{language}
	if code, region, found := strings.Cut(language, "-"); found && region != "" {
		forms = append(forms,
			code+"-"+strings.ToUpper(region),
			code+"-"+strings.ToUpper(region[:1])+region[1:])
	}
	return forms
}

// base is a language without its region.
func base(language string) string {
	code, _, _ := strings.Cut(language, "-")
	return code
}

// parseResourceFile reads a .resx file, which is XML holding one value for
// each name.
func parseResourceFile(reader io.Reader) (map[string]string, error) {
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	table := map[string]string{}
	currentName := ""

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return table, err
		}

		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "data":
				currentName = attributeOf(element, "name")
			case "value":
				if currentName == "" {
					continue
				}
				text, err := readElementText(decoder, element)
				if err != nil {
					return table, err
				}
				// The file explains its own format with examples, which
				// are not words of the settings.
				if isSampleName(currentName) {
					continue
				}
				table[currentName] = text
			}
		case xml.EndElement:
			if element.Name.Local == "data" {
				currentName = ""
			}
		}
	}
	return table, nil
}

// isSampleName reports whether a name is one of the examples a .resx file
// explains its own format with.
func isSampleName(name string) bool {
	switch name {
	case "Name1", "Bitmap1", "Icon1", "Comment1", "String1", "String2", "Text1":
		return true
	default:
		return false
	}
}

func attributeOf(element xml.StartElement, name string) string {
	for _, attribute := range element.Attr {
		if attribute.Name.Local == name {
			return attribute.Value
		}
	}
	return ""
}

// readElementText reads the text of an element to its end.
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
			if _, err := readElementText(decoder, element); err != nil {
				return text.String(), err
			}
		case xml.EndElement:
			if element.Name == start.Name {
				return strings.TrimSpace(text.String()), nil
			}
		}
	}
}
