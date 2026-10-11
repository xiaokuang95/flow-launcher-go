package core

import (
	"io/fs"
	"strings"
	"sync"
)

// PluginTranslator translates the words a plugin itself shows, from the
// language files it ships with.
//
// A plugin of the reference's has its own Languages directory beside its
// code, with one XAML file per language; a plugin here embeds the same
// files and reads them the same way:
//
//	//go:embed Languages/*.xaml
//	var languages embed.FS
//
//	translator := core.NewPluginTranslator(languages, "Languages")
//	...
//	subtitle := translator.T("flowlauncher_plugin_calculator_copy_number_to_clipboard")
//
// A plugin that shows nothing of its own needs none of this.
type PluginTranslator struct {
	files fs.FS
	dir   string

	mu     sync.Mutex
	tables map[string]map[string]string
}

// NewPluginTranslator reads a plugin's language files out of an embedded
// directory.
func NewPluginTranslator(files fs.FS, dir string) *PluginTranslator {
	return &PluginTranslator{files: files, dir: dir, tables: map[string]map[string]string{}}
}

// T is the plugin's word for a key, in the launcher's language, falling
// back to English and then to the key itself.
func (t *PluginTranslator) T(key string) string {
	if t == nil {
		return key
	}

	if value, ok := t.table(CurrentLanguage())[key]; ok {
		return value
	}
	if value, ok := t.table(DefaultLanguage)[key]; ok {
		return value
	}
	return key
}

// Tf is T with the placeholders filled, which the language files write as
// {0}, {1} and so on.
func (t *PluginTranslator) Tf(key string, args ...string) string {
	text := t.T(key)
	for i, arg := range args {
		text = strings.ReplaceAll(text, "{"+itoa(i)+"}", arg)
	}
	return text
}

// table parses one language's file, once.
func (t *PluginTranslator) table(language string) map[string]string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if table, ok := t.tables[language]; ok {
		return table
	}

	table := map[string]string{}
	// The file is named for the language, or for the language without its
	// region, as the launcher's own files are.
	for _, name := range languageFileNames(language) {
		file, err := t.files.Open(t.dir + "/" + name + ".xaml")
		if err != nil {
			continue
		}
		parsed, err := parseLanguageDictionary(file)
		file.Close()
		if err != nil {
			continue
		}
		for key, value := range parsed {
			table[key] = value
		}
	}

	t.tables[language] = table
	return table
}
