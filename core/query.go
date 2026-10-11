package core

import (
	"strings"
	"unicode/utf8"
)

// TermSeparator splits a query into its terms.
const TermSeparator = " "

// GlobalPluginWildcardSign is the action keyword of a plugin that answers
// every query.
const GlobalPluginWildcardSign = "*"

// Query is what the launcher sends a plugin.
type Query struct {
	// OriginalQuery is exactly what the user typed.
	OriginalQuery string
	// TrimmedQuery is OriginalQuery without surrounding whitespace, with
	// the built-in and custom shortcuts already expanded.
	TrimmedQuery string
	// Search is the query without the action keyword: what a plugin
	// matches against.
	Search string
	// SearchTerms is Search split at its spaces.
	SearchTerms []string
	// ActionKeyword is the keyword that chose the plugin, empty for a
	// plugin that answers every query.
	ActionKeyword string
	// IsHomeQuery says the user typed nothing, so the launcher is showing
	// what it shows when it opens.
	IsHomeQuery bool
	// IsReQuery says the user asked for the query again, as Ctrl+R does,
	// so a plugin serving cached results should look them up afresh.
	IsReQuery bool
}

// FirstSearch is the first search term, empty when there is none.
func (q Query) FirstSearch() string { return q.term(0) }

// SecondSearch is the second search term, empty when there is none.
func (q Query) SecondSearch() string { return q.term(1) }

// ThirdSearch is the third search term, empty when there is none.
func (q Query) ThirdSearch() string { return q.term(2) }

// SecondToEndSearch is every search term from the second on, joined by
// spaces, empty when there is only one.
func (q Query) SecondToEndSearch() string {
	if len(q.SearchTerms) > 1 {
		return strings.Join(q.SearchTerms[1:], TermSeparator)
	}
	return ""
}

func (q Query) term(index int) string {
	if index < len(q.SearchTerms) {
		return q.SearchTerms[index]
	}
	return ""
}

// String is the trimmed query, as the reference prints it.
func (q Query) String() string { return q.TrimmedQuery }

// BuildQuery turns what the user typed into the query a plugin gets, given
// the plugins that answer to an action keyword: a leading term that is one
// of their keywords picks that plugin, and is taken off the search.
func BuildQuery(originalQuery, trimmedQuery string, nonGlobalPlugins map[string][]PluginPair) *Query {
	// An empty query is the home query: nothing was typed.
	if trimmedQuery == "" {
		return &Query{IsHomeQuery: true}
	}

	// Repeated spaces are one separator.
	terms := strings.Fields(trimmedQuery)
	if len(terms) == 0 {
		return nil
	}

	actionKeyword := ""
	search := strings.TrimSpace(trimmedQuery)
	searchTerms := terms

	if pluginPairs, ok := nonGlobalPlugins[terms[0]]; ok && hasAnyEnabledPlugin(pluginPairs) {
		actionKeyword = terms[0]
		// The search is what follows the keyword and its separator, so the
		// keyword's own characters are measured rather than assumed.
		runes := []rune(trimmedQuery)
		cut := min(utf8.RuneCountInString(actionKeyword)+1, len(runes))
		search = strings.TrimLeft(string(runes[cut:]), " \t")
		searchTerms = terms[1:]
	}

	return &Query{
		OriginalQuery: originalQuery,
		TrimmedQuery:  trimmedQuery,
		Search:        search,
		SearchTerms:   searchTerms,
		ActionKeyword: actionKeyword,
	}
}

func hasAnyEnabledPlugin(pairs []PluginPair) bool {
	for _, pair := range pairs {
		if !pair.Metadata.Disabled {
			return true
		}
	}
	return false
}
