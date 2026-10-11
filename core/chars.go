package core

import (
	"strings"
	"unicode"
)

// accentMap folds the accented letters the reference folds. It is keyed by
// the lower-case letter, as the reference's lookup is.
var accentMap = map[rune]rune{
	'á': 'a', 'à': 'a', 'ã': 'a', 'â': 'a', 'ä': 'a', 'å': 'a', 'ā': 'a', 'ă': 'a', 'ą': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e', 'ĕ': 'e', 'ė': 'e', 'ę': 'e', 'ě': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i', 'ĭ': 'i', 'į': 'i', 'ı': 'i',
	'ó': 'o', 'ò': 'o', 'õ': 'o', 'ô': 'o', 'ö': 'o', 'ø': 'o', 'ō': 'o', 'ŏ': 'o', 'ő': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u', 'ŭ': 'u', 'ů': 'u', 'ű': 'u', 'ų': 'u',
	'ç': 'c', 'ć': 'c', 'ĉ': 'c', 'ċ': 'c', 'č': 'c',
	'ñ': 'n', 'ń': 'n', 'ņ': 'n', 'ň': 'n', 'ŋ': 'n',
	'ý': 'y', 'ÿ': 'y', 'ŷ': 'y',
	'ś': 's', 'ŝ': 's', 'ş': 's', 'š': 's', 'ß': 's',
	'ź': 'z', 'ż': 'z', 'ž': 'z',
	'ł': 'l',
	'ď': 'd', 'đ': 'd',
	'ĝ': 'g', 'ğ': 'g', 'ġ': 'g', 'ģ': 'g',
	'ĥ': 'h', 'ħ': 'h',
	'ĵ': 'j',
	'ķ': 'k',
	'ŕ': 'r', 'ř': 'r',
	'ţ': 't', 'ť': 't', 'ŧ': 't',
	'æ': 'a',
	'œ': 'o',
}

// NormalizeDiacriticChar lower-cases a rune and folds an accented letter to
// the plain one under it.
func NormalizeDiacriticChar(c rune) rune {
	c = toLower(c)
	if mapped, ok := accentMap[c]; ok {
		return mapped
	}
	return c
}

// NormalizeDiacritics folds every accented letter of the text.
func NormalizeDiacritics(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, c := range value {
		b.WriteRune(NormalizeDiacriticChar(c))
	}
	return b.String()
}

// toLower is the invariant lower case of the reference.
func toLower(c rune) rune { return unicode.ToLower(c) }

// isUpper is the reference's char.IsUpper.
func isUpper(c rune) bool { return unicode.IsUpper(c) }

// isSpace is the reference's char.IsWhiteSpace.
func isSpace(c rune) bool { return unicode.IsSpace(c) }

// isAsciiDigit is the reference's char.IsAsciiDigit.
func isAsciiDigit(c rune) bool { return c >= '0' && c <= '9' }
