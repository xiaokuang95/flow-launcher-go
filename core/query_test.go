package core

import "testing"

// The cases in this file are the reference's own, from
// Flow.Launcher.Test/QueryBuilderTest.cs.

func exclusivePlugins(disabled bool) map[string][]PluginPair {
	return map[string][]PluginPair{
		">": {{Metadata: PluginMetadata{ActionKeywords: []string{">"}, Disabled: disabled}}},
	}
}

func TestExclusivePluginQuery(t *testing.T) {
	query := BuildQuery(">   ping    google.com   -n 20  -6", ">   ping    google.com   -n 20  -6", exclusivePlugins(false))

	if query.TrimmedQuery != ">   ping    google.com   -n 20  -6" {
		t.Errorf("TrimmedQuery = %q", query.TrimmedQuery)
	}
	if query.Search != "ping    google.com   -n 20  -6" {
		t.Errorf("Search = %q, want it without the action keyword", query.Search)
	}
	if query.ActionKeyword != ">" {
		t.Errorf("ActionKeyword = %q, want >", query.ActionKeyword)
	}
	if len(query.SearchTerms) != 5 {
		t.Errorf("SearchTerms = %v, want 5 terms", query.SearchTerms)
	}
	if query.FirstSearch() != "ping" {
		t.Errorf("FirstSearch = %q", query.FirstSearch())
	}
	if query.SecondSearch() != "google.com" {
		t.Errorf("SecondSearch = %q", query.SecondSearch())
	}
	if query.ThirdSearch() != "-n" {
		t.Errorf("ThirdSearch = %q", query.ThirdSearch())
	}
	if query.SecondToEndSearch() != "google.com -n 20 -6" {
		t.Errorf("SecondToEndSearch = %q, want the repeated spaces folded", query.SecondToEndSearch())
	}
}

func TestExclusivePluginQueryIgnoresDisabledPlugins(t *testing.T) {
	query := BuildQuery(">   ping    google.com   -n 20  -6", ">   ping    google.com   -n 20  -6", exclusivePlugins(true))

	if query.Search != ">   ping    google.com   -n 20  -6" {
		t.Errorf("Search = %q", query.Search)
	}
	if query.Search != query.TrimmedQuery {
		t.Errorf("Search = %q, TrimmedQuery = %q, want them equal", query.Search, query.TrimmedQuery)
	}
	if len(query.SearchTerms) != 6 {
		t.Errorf("SearchTerms = %v, want 6 terms", query.SearchTerms)
	}
	if query.ActionKeyword == ">" {
		t.Error("the keyword of a disabled plugin was used")
	}
	if query.SecondToEndSearch() != "ping google.com -n 20 -6" {
		t.Errorf("SecondToEndSearch = %q", query.SecondToEndSearch())
	}
}

func TestSharedKeywordWithOneDisabledPlugin(t *testing.T) {
	plugins := map[string][]PluginPair{
		">": {
			{Metadata: PluginMetadata{ActionKeywords: []string{">"}, Disabled: true}},
			{Metadata: PluginMetadata{ActionKeywords: []string{">"}}},
		},
	}
	query := BuildQuery(">   ping    google.com   -n 20  -6", ">   ping    google.com   -n 20  -6", plugins)

	if query.Search != "ping    google.com   -n 20  -6" {
		t.Errorf("Search = %q", query.Search)
	}
	if query.ActionKeyword != ">" {
		t.Errorf("ActionKeyword = %q, want > because an enabled plugin shares it", query.ActionKeyword)
	}
	if len(query.SearchTerms) != 5 {
		t.Errorf("SearchTerms = %v, want 5 terms", query.SearchTerms)
	}
}

func TestGenericPluginQuery(t *testing.T) {
	query := BuildQuery("file.txt file2 file3", "file.txt file2 file3", nil)

	if query.Search != "file.txt file2 file3" {
		t.Errorf("Search = %q", query.Search)
	}
	if query.ActionKeyword != "" {
		t.Errorf("ActionKeyword = %q, want none", query.ActionKeyword)
	}
	if query.FirstSearch() != "file.txt" || query.SecondSearch() != "file2" || query.ThirdSearch() != "file3" {
		t.Errorf("terms = %q %q %q", query.FirstSearch(), query.SecondSearch(), query.ThirdSearch())
	}
	if query.SecondToEndSearch() != "file2 file3" {
		t.Errorf("SecondToEndSearch = %q", query.SecondToEndSearch())
	}
}

func TestEmptyQueryIsTheHomeQuery(t *testing.T) {
	query := BuildQuery("", "", nil)
	if query == nil {
		t.Fatal("an empty query built nothing")
	}
	if !query.IsHomeQuery {
		t.Error("an empty query is not the home query")
	}
	if query.Search != "" || query.ActionKeyword != "" {
		t.Errorf("the home query is %+v", query)
	}
}

// TestKeywordWithNonASCIICharacters covers a keyword the reference measures
// in UTF-16 units: this port measures runes, and both cut the keyword off
// at the same place for text that is not astral.
func TestKeywordWithNonASCIICharacters(t *testing.T) {
	plugins := map[string][]PluginPair{
		"搜索": {{Metadata: PluginMetadata{ActionKeywords: []string{"搜索"}}}},
	}
	query := BuildQuery("搜索 关键词", "搜索 关键词", plugins)

	if query.ActionKeyword != "搜索" {
		t.Errorf("ActionKeyword = %q", query.ActionKeyword)
	}
	if query.Search != "关键词" {
		t.Errorf("Search = %q, want the text after the keyword", query.Search)
	}
}
