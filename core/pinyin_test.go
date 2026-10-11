package core

import "testing"

// The cases here are the reference's own: the readings it expects of the
// phrases its test names, and the index mapping its TranslationMapping test
// walks through.
func TestChineseIsWrittenAsPinyin(t *testing.T) {
	withOverrides := &PinyinAlphabet{UsePolyphonicPhraseOverrides: true}
	withoutOverrides := &PinyinAlphabet{}

	for _, test := range []struct {
		alphabet *PinyinAlphabet
		text     string
		want     string
	}{
		// A phrase is read as the word it is.
		{withOverrides, "重启", "Chong Qi"},
		{withOverrides, "重启 Flow Launcher", "Chong Qi Flow Launcher"},
		{withOverrides, "重庆", "Chong Qing"},
		{withOverrides, "核查", "He Cha"},
		// And character by character when the user does not want that.
		{withoutOverrides, "重启", "Zhong Qi"},
	} {
		got, _ := test.alphabet.Translate(test.text)
		if got != test.want {
			t.Errorf("Translate(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestDoublePinyinWritesEachSyllableInTwoLetters(t *testing.T) {
	alphabet := &PinyinAlphabet{
		UseDoublePinyin:              true,
		Schema:                       DefaultDoublePinyinSchema,
		UsePolyphonicPhraseOverrides: true,
	}

	got, _ := alphabet.Translate("重启")
	if got != "is qi" {
		t.Errorf("Translate(重启) = %q, want %q", got, "is qi")
	}
}

func TestTextWithoutChineseIsLeftAlone(t *testing.T) {
	alphabet := &PinyinAlphabet{UsePolyphonicPhraseOverrides: true}

	got, mapping := alphabet.Translate("Flow Launcher")
	if got != "Flow Launcher" {
		t.Errorf("Translate = %q, want the text itself", got)
	}
	if mapping != nil {
		t.Errorf("the mapping is %v, want none for text that was not translated", mapping)
	}
}

func TestShouldTranslateOnlyAQueryWithoutChinese(t *testing.T) {
	alphabet := &PinyinAlphabet{}

	if !alphabet.ShouldTranslate("chongqi") {
		t.Errorf("a query typed in Latin letters is matched against pinyin")
	}
	if alphabet.ShouldTranslate("重启") {
		t.Errorf("a query typed in Chinese is matched as it is")
	}
}

func TestAPinyinIndexMapsBackToTheCharacter(t *testing.T) {
	// 用 and 户 are two characters; the mapping says which of them a rune
	// of the pinyin came from.
	mapping := &translationMapping{}
	mapping.addNewIndex(0, 1)  // F
	mapping.addNewIndex(1, 1)  // l
	mapping.addNewIndex(2, 1)  // o
	mapping.addNewIndex(3, 1)  // w
	mapping.addNewIndex(4, 1)  // ' '
	mapping.addNewIndex(5, 4)  // 用 -> Yong
	mapping.addNewIndex(10, 2) // 户 -> Hu

	for _, test := range []struct{ translated, original int }{
		{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4},
		{5, 5}, {6, 5}, {7, 5}, {8, 5},
		{10, 6}, {11, 6},
	} {
		if got := mapping.MapToOriginalIndex(test.translated); got != test.original {
			t.Errorf("MapToOriginalIndex(%d) = %d, want %d", test.translated, got, test.original)
		}
	}
}

func TestTheMappingOfARealTranslation(t *testing.T) {
	alphabet := &PinyinAlphabet{UsePolyphonicPhraseOverrides: true}

	// 微信 becomes "Wei Xin": the first rune of the pinyin is the first
	// character, and the fifth is the second.
	translated, mapping := alphabet.Translate("微信")
	if translated != "Wei Xin" {
		t.Fatalf("Translate(微信) = %q, want %q", translated, "Wei Xin")
	}
	if got := mapping.MapToOriginalIndex(0); got != 0 {
		t.Errorf("the first rune of the pinyin came from the rune %d, want 0", got)
	}
	if got := mapping.MapToOriginalIndex(4); got != 1 {
		t.Errorf("the fifth rune of the pinyin came from the rune %d, want 1", got)
	}
}

func TestAPinyinQueryFindsAChineseName(t *testing.T) {
	matcher := StringMatcher{
		UserSettingSearchPrecision: SearchPrecisionRegular,
		Alphabet:                   &PinyinAlphabet{UsePolyphonicPhraseOverrides: true},
	}

	match := matcher.FuzzyMatch("weixin", "微信")
	if !match.Success || !match.IsSearchPrecisionScoreMet() {
		t.Errorf("weixin did not find 微信: %+v", match)
	}

	// The highlight is in the characters, not in the pinyin.
	for _, index := range match.MatchData {
		if index < 0 || index > 1 {
			t.Errorf("the highlight at %d is not in the two characters", index)
		}
	}
}

func TestADoublePinyinQueryFindsAPhrase(t *testing.T) {
	matcher := StringMatcher{
		UserSettingSearchPrecision: SearchPrecisionRegular,
		Alphabet: &PinyinAlphabet{
			UseDoublePinyin:              true,
			Schema:                       DefaultDoublePinyinSchema,
			UsePolyphonicPhraseOverrides: true,
		},
	}

	if match := matcher.FuzzyMatch("isqi", "重启"); !match.Success {
		t.Errorf("isqi did not find 重启: %+v", match)
	}
	// The other reading of the same characters is not what the phrase says.
	if match := matcher.FuzzyMatch("vsqi", "重启"); match.Success && match.IsSearchPrecisionScoreMet() {
		t.Errorf("vsqi found 重启, want the phrase's own reading: %+v", match)
	}
}
