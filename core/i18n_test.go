package core

import "testing"

// The language files are the reference's, so these tests check that they
// were read rather than that they say anything in particular.

func TestTranslationReadsTheLanguageFile(t *testing.T) {
	// A key every language has, and the English text beside it.
	if got := T("en", "general"); got != "General" {
		t.Errorf("T(en, general) = %q, want %q", got, "General")
	}
	if got := T("zh-cn", "general"); got != "通用" {
		t.Errorf("T(zh-cn, general) = %q, want %q", got, "通用")
	}
}

func TestTranslationKeepsUnknownKeys(t *testing.T) {
	if got := T("en", "noSuchKeyExists"); got != "noSuchKeyExists" {
		t.Errorf("T of an unknown key = %q, want the key itself", got)
	}
}

func TestTranslationFallsBackToEnglish(t *testing.T) {
	// A key that only English has, in a language that has no translation
	// for it, must still read as English rather than as the key.
	key := "runtimePluginChooseRuntimeExecutable"
	if T("en", key) == key {
		t.Fatalf("%s is missing from the English file", key)
	}
	if got := T("cs", key); got == key {
		t.Errorf("T(cs, %s) = %q, want the English text", key, got)
	}
}

func TestTranslationFillsPlaceholders(t *testing.T) {
	got := Tf("en", "runtimePluginChooseRuntimeExecutable", "Python")
	want := "Please select the Python executable"
	if got != want {
		t.Errorf("Tf = %q, want %q", got, want)
	}
}

func TestMultilineValuesReadAsOneSentence(t *testing.T) {
	// This value is written over three lines in the file.
	got := T("en", "runtimePluginUnableToSetExecutablePath")
	if want := "Unable to set {0} executable path, please try from Flow's settings (scroll down to the bottom)."; got != want {
		t.Errorf("T = %q, want %q", got, want)
	}
}

func TestLanguageCodeForCulture(t *testing.T) {
	cases := map[string]string{
		"en-US":      "en",
		"en":         "en",
		"zh-CN":      "zh-cn",
		"zh-Hans-CN": "zh-cn",
		"zh-TW":      "zh-tw",
		"zh-Hant-TW": "zh-tw",
		"de-DE":      "de",
		"pt-BR":      "pt-br",
		"pt-PT":      "pt-pt",
		"xx-YY":      "en",
	}
	for culture, want := range cases {
		if got := LanguageCodeForCulture(culture); got != want {
			t.Errorf("LanguageCodeForCulture(%q) = %q, want %q", culture, got, want)
		}
	}
}

func TestSetLanguage(t *testing.T) {
	SetLanguage("de")
	if got := CurrentLanguage(); got != "de" {
		t.Errorf("CurrentLanguage() = %q, want de", got)
	}
	if got := T("", "general"); got == "General" {
		t.Error("T with the current language still reads English")
	}

	SetLanguage("not-a-language")
	if got := CurrentLanguage(); got != DefaultLanguage {
		t.Errorf("an unknown language left %q, want %q", got, DefaultLanguage)
	}
	SetLanguage(DefaultLanguage)
}

func TestEveryLanguageHasTheSameKeys(t *testing.T) {
	english := translationTable("en")
	if len(english) < 500 {
		t.Fatalf("the English file has only %d keys", len(english))
	}

	for _, language := range AvailableLanguages {
		table := translationTable(language.Code)
		if len(table) == 0 {
			t.Errorf("%s has no translations", language.Code)
			continue
		}

		// A translation may leave a key out, in which case English is
		// shown, and may carry a key English has dropped.
		covered := 0
		for key := range english {
			if _, ok := table[key]; ok {
				covered++
			}
		}
		if covered*100/len(english) < 90 {
			t.Errorf("%s translates %d of %d keys", language.Code, covered, len(english))
		}
	}
}

// TestVietnameseLoads records a fix rather than a copy: the reference names
// the language "vi-vn" and its file "vi.xaml", so the reference never loads
// the translation. This port looks the language up as well as the tag.
func TestVietnameseLoads(t *testing.T) {
	if got := T("vi-vn", "general"); got == "general" || got == "General" {
		t.Errorf("T(vi-vn, general) = %q, want the Vietnamese text", got)
	}
}
