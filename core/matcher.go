package core

import "strings"

// SearchPrecisionScore is how good a match has to be to be shown at all.
// The values are the reference's.
type SearchPrecisionScore int

const (
	// SearchPrecisionRegular is the highest precision: a match must score 50.
	SearchPrecisionRegular SearchPrecisionScore = 50
	// SearchPrecisionLow accepts a weaker match, as a plugin asking for one.
	SearchPrecisionLow SearchPrecisionScore = 20
	// SearchPrecisionNone accepts anything that matched at all.
	SearchPrecisionNone SearchPrecisionScore = 0
)

// MatchOption says how a match compares the two strings.
type MatchOption struct {
	// IgnoreCase folds case, as the reference does by default.
	IgnoreCase bool
}

// String is the name the settings file and the settings window use.
func (p SearchPrecisionScore) String() string {
	switch p {
	case SearchPrecisionLow:
		return "Low"
	case SearchPrecisionNone:
		return "None"
	default:
		return "Regular"
	}
}

// DefaultMatchOption is the option a plugin gets when it asks for none.
func DefaultMatchOption() MatchOption { return MatchOption{IgnoreCase: true} }

// MatchResult is the outcome of matching a query against a string.
type MatchResult struct {
	// Success reports whether the query matched at all.
	Success bool
	// MatchData holds the rune indexes of stringToCompare the query matched,
	// for highlighting, in the order the reference collects them.
	MatchData []int
	// SearchPrecision is the score a match must reach to count.
	SearchPrecision SearchPrecisionScore
	// RawScore is the score before the precision filter.
	RawScore int
}

// Score is the score after the precision filter: zero when the match is too
// weak to show.
func (m MatchResult) Score() int {
	if m.IsSearchPrecisionScoreMet() {
		return m.RawScore
	}
	return 0
}

// IsSearchPrecisionScoreMet reports whether the match is good enough.
func (m MatchResult) IsSearchPrecisionScoreMet() bool {
	return m.RawScore >= int(m.SearchPrecision)
}

// StringMatcher matches a query against strings the way the reference does:
// an acronym match first, then a fuzzy search that scores a match by how
// early it starts and how tightly the characters are packed.
type StringMatcher struct {
	// UserSettingSearchPrecision is the precision from the user's settings.
	UserSettingSearchPrecision SearchPrecisionScore
	// IgnoreAccents folds accented letters to their plain ones.
	IgnoreAccents bool
	// Alphabet translates a compared string so that a query in another
	// script, pinyin for example, can match it.
	Alphabet Alphabet
}

// Alphabet translates text so a query can match it in another script.
type Alphabet interface {
	// ShouldTranslate reports whether a query needs a translation.
	ShouldTranslate(query string) bool
	// Translate returns the translated text and how its runes map back.
	Translate(text string) (string, TranslationMapping)
}

// TranslationMapping maps a rune index of translated text back to the index
// of the original text.
type TranslationMapping interface {
	MapToOriginalIndex(index int) int
}

// FuzzyMatch matches query against stringToCompare with the default option.
func (s StringMatcher) FuzzyMatch(query, stringToCompare string) MatchResult {
	return s.Match(query, stringToCompare, DefaultMatchOption())
}

// Match matches query against stringToCompare. It first tries an acronym
// match: every character of the query against the initials of the compared
// string, its upper-case letters, its digits and the characters after a
// space. Failing that it searches fuzzily, scoring a match that starts
// early and packs its characters tightly above one that does not.
func (s StringMatcher) Match(query, stringToCompare string, opt MatchOption) MatchResult {
	if stringToCompare == "" || query == "" {
		return MatchResult{SearchPrecision: s.UserSettingSearchPrecision}
	}

	query = strings.TrimSpace(query)

	var mapping TranslationMapping
	if s.Alphabet != nil && s.Alphabet.ShouldTranslate(query) {
		// A query that can be translated means the user does not want the
		// compared string translated the other way.
		stringToCompare, mapping = s.Alphabet.Translate(stringToCompare)
	}

	currentAcronymQueryIndex := 0
	var acronymMatchData []int
	// acronymsTotalCount counts the distinct acronym groups in the compared
	// string: a run of digits is one group, not one per digit.
	acronymsTotalCount := 0
	acronymsMatched := 0

	queryToCompare := query
	ignoreAccents := s.IgnoreAccents
	ignoreCase := opt.IgnoreCase

	if ignoreAccents {
		queryToCompare = NormalizeDiacritics(queryToCompare)
	} else if ignoreCase {
		queryToCompare = strings.ToLower(queryToCompare)
	}

	querySubstrings := strings.Fields(queryToCompare)
	if len(querySubstrings) == 0 {
		return MatchResult{SearchPrecision: s.UserSettingSearchPrecision}
	}
	currentQuerySubstringIndex := 0
	currentQuerySubstring := []rune(querySubstrings[currentQuerySubstringIndex])
	currentQuerySubstringCharacterIndex := 0

	firstMatchIndex := -1
	firstMatchIndexInWord := -1
	lastMatchIndex := 0
	allQuerySubstringsMatched := false
	matchFoundInPreviousLoop := false
	allSubstringsContainedInCompareString := true

	var indexList []int
	var spaceIndices []int

	compareRunes := []rune(stringToCompare)
	queryRunes := []rune(query)
	queryToCompareRunes := []rune(queryToCompare)

	for compareStringIndex := 0; compareStringIndex < len(compareRunes); compareStringIndex++ {
		// Once the acronyms are all matched, keep counting the groups that
		// were not, for the score.
		if currentAcronymQueryIndex >= len(queryRunes) && acronymsMatched == len(queryRunes) {
			if isAcronymCount(compareRunes, compareStringIndex) {
				acronymsTotalCount++
			}
			continue
		}

		if currentAcronymQueryIndex >= len(queryRunes) {
			break
		}

		compareChar := compareRunes[compareStringIndex]
		if ignoreAccents {
			compareChar = NormalizeDiacriticChar(compareChar)
		} else if ignoreCase {
			compareChar = toLower(compareChar)
		}

		// The spaces before the first match, to weigh a match that starts
		// after one. Only the first query substring looks at them.
		if compareChar == ' ' && currentQuerySubstringIndex == 0 {
			spaceIndices = append(spaceIndices, compareStringIndex)
		}

		if isAcronym(compareRunes, compareStringIndex) {
			if compareChar == queryToCompareRunes[currentAcronymQueryIndex] {
				acronymMatchData = append(acronymMatchData, compareStringIndex)
				acronymsMatched++
				currentAcronymQueryIndex++
			}
		}

		if isAcronymCount(compareRunes, compareStringIndex) {
			acronymsTotalCount++
		}

		if allQuerySubstringsMatched ||
			currentQuerySubstringCharacterIndex >= len(currentQuerySubstring) ||
			compareChar != currentQuerySubstring[currentQuerySubstringCharacterIndex] {
			matchFoundInPreviousLoop = false
			continue
		}

		if firstMatchIndex < 0 {
			// The first matched character starts the compared string.
			firstMatchIndex = compareStringIndex
		}

		if currentQuerySubstringCharacterIndex == 0 {
			// The first letter of the current word.
			matchFoundInPreviousLoop = true
			firstMatchIndexInWord = compareStringIndex
		} else if !matchFoundInPreviousLoop {
			// Not a full word: check whether the characters before this one
			// are the pattern too, which would be a better match.
			startIndexToVerify := compareStringIndex - currentQuerySubstringCharacterIndex

			if allPreviousCharsMatched(compareRunes, startIndexToVerify, currentQuerySubstringCharacterIndex,
				currentQuerySubstring, ignoreAccents, ignoreCase) {
				matchFoundInPreviousLoop = true

				// The start of the first query substring moves the start of the match.
				if currentQuerySubstringIndex == 0 {
					firstMatchIndex = startIndexToVerify
				}

				indexList = updatedIndexList(startIndexToVerify, currentQuerySubstringCharacterIndex,
					firstMatchIndexInWord, indexList)
			}
		}

		lastMatchIndex = compareStringIndex + 1
		indexList = append(indexList, compareStringIndex)

		currentQuerySubstringCharacterIndex++

		if currentQuerySubstringCharacterIndex == len(currentQuerySubstring) {
			// Any substring that was not matched means none of them count.
			allSubstringsContainedInCompareString =
				matchFoundInPreviousLoop && allSubstringsContainedInCompareString

			currentQuerySubstringIndex++

			allQuerySubstringsMatched = currentQuerySubstringIndex >= len(querySubstrings)

			if allQuerySubstringsMatched {
				continue
			}

			currentQuerySubstring = []rune(querySubstrings[currentQuerySubstringIndex])
			currentQuerySubstringCharacterIndex = 0
		}
	}

	// An acronym match needs every character of the query matched.
	if acronymsMatched > 0 && acronymsMatched == len(queryRunes) {
		matchedGroups := countDistinctAcronymGroups(acronymMatchData, compareRunes)
		acronymScore := matchedGroups * 100 / acronymsTotalCount

		if acronymScore >= int(s.UserSettingSearchPrecision) {
			return MatchResult{
				Success:         true,
				SearchPrecision: s.UserSettingSearchPrecision,
				MatchData:       mapIndices(distinct(acronymMatchData), mapping),
				RawScore:        acronymScore,
			}
		}
	}

	if allQuerySubstringsMatched {
		nearestSpaceIndex := closestSpaceIndex(spaceIndices, firstMatchIndex)

		// firstMatchIndex-nearestSpaceIndex-1 puts the start just after the
		// space, so that "world" in "hello world" and in "world hello" both
		// start at zero and the distance does not favour one.
		score := searchScore(queryRunes, compareRunes, firstMatchIndex-nearestSpaceIndex-1, spaceIndices,
			lastMatchIndex-firstMatchIndex, allSubstringsContainedInCompareString)

		return MatchResult{
			Success:         true,
			SearchPrecision: s.UserSettingSearchPrecision,
			MatchData:       mapIndices(distinct(indexList), mapping),
			RawScore:        score,
		}
	}

	return MatchResult{SearchPrecision: s.UserSettingSearchPrecision}
}

// isAcronym reports whether the rune at index counts as an initial: it is
// upper case, it starts the string, or a space comes before it.
func isAcronym(s []rune, i int) bool {
	return isAcronymChar(s, i) || isAsciiDigit(s[i])
}

// isAcronymChar reports whether the rune at index is an initial rather than
// a digit.
func isAcronymChar(s []rune, i int) bool {
	return isUpper(s[i]) || i == 0 || isSpace(s[i-1])
}

// isAcronymCount reports whether the rune at index adds a group to the count
// of acronyms: a run of digits is one group, so only its first digit counts.
func isAcronymCount(s []rune, i int) bool {
	if isAcronymChar(s, i) {
		return true
	}
	if isAsciiDigit(s[i]) {
		return i == 0 || !isAsciiDigit(s[i-1])
	}
	return false
}

// countDistinctAcronymGroups counts the groups the matched indexes belong to.
// Every matched non-digit index is a group of its own; the digits of one run
// are a single group however many of them matched.
func countDistinctAcronymGroups(matchedIndices []int, stringToCompare []rune) int {
	groups := 0
	processed := make(map[int]bool, len(matchedIndices))

	for _, matchedIndex := range matchedIndices {
		if processed[matchedIndex] {
			continue
		}
		processed[matchedIndex] = true

		groups++

		if !isAsciiDigit(stringToCompare[matchedIndex]) {
			continue
		}

		// The rest of the digit run belongs to this group.
		digitRunEnd := matchedIndex
		for digitRunEnd < len(stringToCompare)-1 && isAsciiDigit(stringToCompare[digitRunEnd+1]) {
			digitRunEnd++
			processed[digitRunEnd] = true
		}
	}

	return groups
}

// closestSpaceIndex returns the index of the last space before firstMatchIndex.
func closestSpaceIndex(spaceIndices []int, firstMatchIndex int) int {
	closest := -1
	for _, index := range spaceIndices {
		if index < firstMatchIndex {
			closest = index
		} else {
			break
		}
	}
	return closest
}

// allPreviousCharsMatched reports whether the characters before the current
// one are the beginning of the query substring.
func allPreviousCharsMatched(s []rune, startIndexToVerify, currentQuerySubstringCharacterIndex int,
	currentQuerySubstring []rune, ignoreAccents, ignoreCase bool) bool {
	for indexToCheck := 0; indexToCheck < currentQuerySubstringCharacterIndex; indexToCheck++ {
		c := s[startIndexToVerify+indexToCheck]
		if ignoreAccents {
			c = NormalizeDiacriticChar(c)
		} else if ignoreCase {
			c = toLower(c)
		}

		if c != currentQuerySubstring[indexToCheck] {
			return false
		}
	}
	return true
}

// updatedIndexList replaces the indexes of a partial word match with the ones
// that start the word.
func updatedIndexList(startIndexToVerify, currentQuerySubstringCharacterIndex, firstMatchIndexInWord int,
	indexList []int) []int {
	kept := indexList[:0]
	for _, index := range indexList {
		if index < firstMatchIndexInWord {
			kept = append(kept, index)
		}
	}

	updated := kept
	for indexToCheck := 0; indexToCheck < currentQuerySubstringCharacterIndex; indexToCheck++ {
		updated = append(updated, startIndexToVerify+indexToCheck)
	}
	return updated
}

// searchScore weighs a match: a match near the start of the string scores
// higher than one near its end, and one whose characters are close together
// higher than one whose are spread out.
func searchScore(query, stringToCompare []rune, firstIndex int, spaceIndices []int, matchLen int,
	allSubstringsContainedInCompareString bool) int {
	score := 100 * (len(query) + 1) / ((1 + firstIndex) + (matchLen + 1))

	// "world" matches "world hello" and "hello world" equally by distance;
	// the extra space before it makes the first the better match.
	if firstIndex == 0 && allSubstringsContainedInCompareString {
		score -= len(spaceIndices)
	}

	// A match that uses up more of the compared string is worth more.
	switch {
	case len(stringToCompare)-len(query) < 5:
		score += 20
	case len(stringToCompare)-len(query) < 10:
		score += 10
	}

	if allSubstringsContainedInCompareString {
		count := 0
		for _, c := range query {
			if !isSpace(c) {
				count++
			}
		}

		// Ten per character is too much for long queries, which would then
		// override the other results, so only the first four count fully.
		const threshold = 4
		if count <= threshold {
			score += count * 10
		} else {
			score += threshold*10 + (count-threshold)*5
		}
	}

	return score
}

// distinct returns the indexes without duplicates, in order.
func distinct(indices []int) []int {
	out := make([]int, 0, len(indices))
	seen := make(map[int]bool, len(indices))
	for _, index := range indices {
		if seen[index] {
			continue
		}
		seen[index] = true
		out = append(out, index)
	}
	return out
}

// mapIndices translates indexes back to the original text.
func mapIndices(indices []int, mapping TranslationMapping) []int {
	if mapping == nil {
		return indices
	}
	out := make([]int, 0, len(indices))
	for _, index := range indices {
		out = append(out, mapping.MapToOriginalIndex(index))
	}
	return out
}
