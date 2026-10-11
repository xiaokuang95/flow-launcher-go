package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"launch/core"
)

// siteSuggester asks a search site what it would suggest, which is what
// makes the launcher offer the rest of a search while it is typed.
type siteSuggester struct {
	// api is what the sites are asked through, which the launcher gives
	// the plugin.
	api *core.PublicAPI
	// site is which site is asked, by name.
	site string
}

// Suggestions returns what the site suggests for what was typed. The site
// is asked the way the reference asks it: each has its own address, and
// its own shape of answer.
func (s *siteSuggester) Suggestions(ctx context.Context, query string) ([]string, error) {
	if s.api == nil || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	address, err := suggestionURL(s.site, query)
	if err != nil {
		return nil, nil
	}

	body, err := s.api.HttpGetStringAsync(ctx, address)
	if err != nil {
		// A site that cannot be reached is not worth saying anything
		// about: the search itself is still offered.
		return nil, nil
	}
	return trimSuggestions(suggestionsFrom(s.site, []byte(body)), 0), nil
}

// suggestionAddresses are where each site is asked what it suggests.
var suggestionAddresses = map[string]string{
	"Google":     "https://www.google.com/complete/search?output=chrome&q=",
	"DuckDuckGo": "https://duckduckgo.com/ac/?type=list&q=",
	"Bing":       "https://api.bing.com/qsonhs.aspx?q=",
	"Baidu":      "http://suggestion.baidu.com/su?json=1&wd=",
}

// suggestionsFrom reads what a site answered. Each site answers in its own
// shape: a list whose second entry is the suggestions, a list of objects,
// or a named field.
func suggestionsFrom(site string, body []byte) []string {
	switch site {
	case "Bing":
		var answer struct {
			AS struct {
				Results []struct {
					Suggestions []struct {
						Text string `json:"Txt"`
					} `json:"Suggests"`
				} `json:"Results"`
			} `json:"AS"`
		}
		if err := json.Unmarshal(body, &answer); err != nil {
			return nil
		}
		var found []string
		for _, result := range answer.AS.Results {
			for _, suggestion := range result.Suggestions {
				if suggestion.Text != "" {
					found = append(found, suggestion.Text)
				}
			}
		}
		return found

	case "Baidu":
		var answer struct {
			Suggestions []string `json:"s"`
		}
		if err := json.Unmarshal(body, &answer); err != nil {
			return nil
		}
		return answer.Suggestions

	case "DuckDuckGo":
		// The list holds objects with the phrase in them.
		var answer []struct {
			Phrase string `json:"phrase"`
		}
		if err := json.Unmarshal(body, &answer); err == nil {
			found := make([]string, 0, len(answer))
			for _, entry := range answer {
				if entry.Phrase != "" {
					found = append(found, entry.Phrase)
				}
			}
			if len(found) > 0 {
				return found
			}
		}
		fallthrough

	default:
		// The list's second entry is the suggestions, as a list of
		// strings.
		var answer []json.RawMessage
		if err := json.Unmarshal(body, &answer); err != nil {
			return nil
		}
		if len(answer) < 2 {
			return nil
		}
		var found []string
		if err := json.Unmarshal(answer[1], &found); err != nil {
			return nil
		}
		return found
	}
}

// suggestionURL is where a site is asked what it suggests for what was
// typed.
func suggestionURL(site, query string) (string, error) {
	address, ok := suggestionAddresses[site]
	if !ok {
		return "", fmt.Errorf("%s does not offer suggestions", site)
	}
	return address + url.QueryEscape(query), nil
}

// trimSuggestions drops what the site answered that is not worth offering.
func trimSuggestions(found []string, limit int) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(found))
	for _, suggestion := range found {
		suggestion = strings.TrimSpace(suggestion)
		if suggestion == "" || seen[suggestion] {
			continue
		}
		seen[suggestion] = true
		out = append(out, suggestion)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
