package core

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

// The pinyin data the launcher ships with, which is the reference's own:
// the readings of phrases that are not the readings of their characters one
// by one, and the tables of the double pinyin keyboards.
//
//go:embed pinyin/*.json
var pinyinFiles embed.FS

// PinyinAlphabet translates Chinese into pinyin, so that a name written in
// Chinese is found by a query typed in Latin letters.
type PinyinAlphabet struct {
	// UseDoublePinyin writes each syllable the way a double pinyin keyboard
	// does: a letter for the initial and a letter for the final.
	UseDoublePinyin bool
	// Schema is which double pinyin keyboard the user knows.
	Schema string
	// UsePolyphonicPhraseOverrides reads a phrase as the word it is rather
	// than as its characters one by one, so that 重启 is chong qi and not
	// zhong qi.
	UsePolyphonicPhraseOverrides bool

	once sync.Once
	// phrases are the readings of whole phrases, and maxPhraseLength how
	// long the longest of them is.
	phrases         map[string][]string
	maxPhraseLength int
	// doublePinyin is the schema's table, from a syllable to what a double
	// pinyin keyboard writes for it.
	doublePinyin map[string]string

	// cache is what has been translated already, which is what makes the
	// check a plugin makes before scoring a name affordable: the same names
	// are compared on every keystroke.
	cacheMu sync.Mutex
	cache   map[string]cachedTranslation
}

// cachedTranslation is a translation that has been made already.
type cachedTranslation struct {
	text    string
	mapping TranslationMapping
}

// ShouldTranslate reports whether a query has to be matched against pinyin
// rather than against the text itself: a query the user typed in Chinese is
// matched as it is, and one typed in Latin letters is matched against the
// pinyin of what it is compared with.
func (a *PinyinAlphabet) ShouldTranslate(query string) bool {
	return !HasChinese(query)
}

// Translate writes text as pinyin, and says which rune of the text each
// rune of the pinyin came from.
func (a *PinyinAlphabet) Translate(text string) (string, TranslationMapping) {
	if !HasChinese(text) {
		return text, nil
	}
	a.load()

	if cached, ok := a.cached(text); ok {
		return cached.text, cached.mapping
	}

	runes := []rune(text)
	readings := make([]string, len(runes))
	for i, r := range runes {
		if isChineseCharacter(r) {
			readings[i] = syllableOf(r)
		}
	}
	a.applyPhraseOverrides(runes, readings)

	var translated strings.Builder
	translated.Grow(len(text) * 2)
	mapping := &translationMapping{}
	// written counts the runes written so far, which is what the mapping
	// is in.
	written := 0

	previousWasChinese := false
	for i, r := range runes {
		if isChineseCharacter(r) {
			reading := readings[i]
			if a.UseDoublePinyin {
				reading = a.toDoublePinyin(reading)
			}
			// Chinese is written without spaces, but each syllable is a
			// word of its own here, so that they can be told apart.
			if i > 0 && runes[i-1] != ' ' {
				translated.WriteRune(' ')
				written++
			}
			mapping.addNewIndex(written, len([]rune(reading)))
			translated.WriteString(reading)
			written += len([]rune(reading))
			previousWasChinese = true
			continue
		}

		if previousWasChinese {
			previousWasChinese = false
			if r != ' ' {
				translated.WriteRune(' ')
				written++
			}
		}
		mapping.addNewIndex(written, 1)
		translated.WriteRune(r)
		written++
	}

	translation := translated.String()
	a.remember(text, translation, mapping)
	return translation, mapping
}

// cached is a translation made before, if there is one.
func (a *PinyinAlphabet) cached(text string) (cachedTranslation, bool) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()

	cached, ok := a.cache[text]
	return cached, ok
}

// remember keeps a translation, so that the names a search compares again
// and again are not read again and again.
func (a *PinyinAlphabet) remember(text, translation string, mapping TranslationMapping) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()

	if a.cache == nil {
		a.cache = map[string]cachedTranslation{}
	}
	a.cache[text] = cachedTranslation{text: translation, mapping: mapping}
}

// load reads the data the settings ask for, once.
func (a *PinyinAlphabet) load() {
	a.once.Do(func() {
		if a.UsePolyphonicPhraseOverrides {
			a.phrases = readPhrases()
			for phrase := range a.phrases {
				if length := len([]rune(phrase)); length > a.maxPhraseLength {
					a.maxPhraseLength = length
				}
			}
		}
		if a.UseDoublePinyin {
			a.doublePinyin = readDoublePinyin(a.Schema)
		}
	})
}

// applyPhraseOverrides reads a phrase as the word it is, which is what
// tells 重启 (chong qi, to restart) from 重启 read character by character
// (zhong qi).
func (a *PinyinAlphabet) applyPhraseOverrides(runes []rune, readings []string) {
	for start := 0; start < len(runes) && a.maxPhraseLength > 0; start++ {
		longest := min(a.maxPhraseLength, len(runes)-start)
		for length := longest; length > 1; length-- {
			phrase := string(runes[start : start+length])
			override, ok := a.phrases[phrase]
			if !ok || len(override) != length || start+length > len(readings) {
				continue
			}
			copy(readings[start:start+length], override)
			start += length - 1
			break
		}
	}
}

// toDoublePinyin writes a syllable the way a double pinyin keyboard does,
// and leaves it alone when the table has nothing for it.
func (a *PinyinAlphabet) toDoublePinyin(syllable string) string {
	if written, ok := a.doublePinyin[syllable]; ok {
		return written
	}
	return syllable
}

// syllableOf is the pinyin of one Chinese character, with its first letter
// capitalized as the reference's own reader writes it.
func syllableOf(r rune) string {
	readings := pinyin.SinglePinyin(r, pinyinReader)
	if len(readings) == 0 {
		return string(r)
	}
	return capitalize(readings[0])
}

// pinyinReader reads one reading of a character, and leaves a character it
// does not know as it is.
var pinyinReader = func() pinyin.Args {
	args := pinyin.NewArgs()
	args.Fallback = func(r rune, args pinyin.Args) []string { return []string{string(r)} }
	return args
}()

// capitalize writes a word with its first letter in capitals.
func capitalize(word string) string {
	runes := []rune(word)
	if len(runes) == 0 {
		return word
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// readPhrases reads the readings of the phrases that are not read the way
// their characters are.
func readPhrases() map[string][]string {
	content, err := pinyinFiles.ReadFile("pinyin/polyphonic_pinyin.json")
	if err != nil {
		return nil
	}
	var phrases map[string][]string
	if err := json.Unmarshal(content, &phrases); err != nil {
		return nil
	}
	return phrases
}

// readDoublePinyin reads the table of one double pinyin keyboard.
func readDoublePinyin(schema string) map[string]string {
	content, err := pinyinFiles.ReadFile("pinyin/double_pinyin.json")
	if err != nil {
		return nil
	}
	var schemas map[string]map[string]string
	if err := json.Unmarshal(content, &schemas); err != nil {
		return nil
	}
	table := schemas[schema]
	if table == nil {
		table = schemas[DefaultDoublePinyinSchema]
	}
	return table
}

// DoublePinyinSchemas are the double pinyin keyboards the launcher knows,
// in the order the reference lists them.
var DoublePinyinSchemas = []string{
	"XiaoHe",
	"ZiRanMa",
	"WeiRuan",
	"ZhiNengABC",
	"ZiGuangPinYin",
	"PinYinJiaJia",
	"XingKongJianDao",
	"DaNiu",
	"XiaoLang",
}

// DefaultDoublePinyinSchema is the keyboard the reference starts with.
const DefaultDoublePinyinSchema = "XiaoHe"

// HasChinese reports whether any rune of the text is a Chinese character,
// which is what decides whether it is written in pinyin at all.
func HasChinese(text string) bool {
	for _, r := range text {
		if isChineseCharacter(r) {
			return true
		}
	}
	return false
}

// isChineseCharacter reports whether a rune is one of the Chinese
// characters a name is written with.
func isChineseCharacter(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF)
}

// translationMapping says which rune of the original text a rune of the
// translation came from.
type translationMapping struct {
	// originalToTranslated holds, for each rune of the original text, one
	// past the last rune of the translation it produced.
	originalToTranslated []int
}

// addNewIndex records that the rune of the original text being written
// produced the translation starting at translatedIndex.
func (m *translationMapping) addNewIndex(translatedIndex, length int) {
	m.originalToTranslated = append(m.originalToTranslated, translatedIndex+length)
}

// MapToOriginalIndex is the rune of the original text the rune of the
// translation at an index came from.
func (m *translationMapping) MapToOriginalIndex(translatedIndex int) int {
	at := sort.SearchInts(m.originalToTranslated, translatedIndex)
	if at < len(m.originalToTranslated) && m.originalToTranslated[at] == translatedIndex {
		return at + 1
	}
	return at
}
