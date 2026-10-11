package url

import (
	"context"
	"testing"

	"launch/core"
	"launch/tools/oracle"
)

// The cases in this file are the reference's own, from
// Flow.Launcher.Test/Plugins/UrlPluginTest.cs: they are what the plugin
// counts as a URL and what it leaves to the other plugins.

func TestValidUrls(t *testing.T) {
	valid := []string{
		"http://www.google.com", "https://www.google.com", "http://google.com",
		"www.google.com", "google.com",
		"http://localhost", "https://localhost", "http://localhost:80", "https://localhost:80",
		"localhost", "localhost:8080",
		"http://110.10.10.10", "110.10.10.10", "110.10.10.10:8080",
		"192.168.1.1", "192.168.1.1:3000",
		"[2001:db8::1]", "[2001:db8::1]:8080", "http://[2001:db8::1]", "https://[2001:db8::1]:8080",
		"[::1]", "[::1]:8080",
		"2001:db8::1", "fe80:1:2::3:4", "::1",
		"HTTP://EXAMPLE.COM", "HTTPS://EXAMPLE.COM", "EXAMPLE.COM", "LOCALHOST",
		"Http://Example.Com", "hTTps://ExAmPlE.CoM", "LocalHost",
		"example.com/path", "example.com/path/to/resource",
		"http://example.com/path", "https://example.com/path?query=1",
		"192.168.1.1/path/to/resource", "192.168.1.1/path/to/resource?query=1",
		"localhost:8080/api/endpoint", "http://localhost/path",
		"[::1]/path", "[2001:db8::1]/path?query=1",
		"192.168.1.1?query=value", "192.168.1.1#fragment",
		"localhost:8080?test=123", "example.com#fragment",
		// Schemes read by their name alone.
		"chrome://settings", "edge://about", "brave://settings", "opera://history",
		"vivaldi://bookmarks", "chrome-extension://abc123def456/", "moz-extension://abc123def456/",
		"file:///C:/path/to/file.txt",
		// Schemes written without the two slashes.
		"about:blank", "about:config", "data:text/plain,hello", "data:,Hello%2C%20World%21",
		"chrome:settings", "chrome-extension:settings",
	}

	for _, raw := range valid {
		if !IsURL(raw) {
			t.Errorf("%q should be a URL", raw)
		}
	}
}

func TestInvalidUrls(t *testing.T) {
	invalid := []string{
		"2001:db8::1/path",
		"wwww",
		"wwww.c",
		"not a url",
		"just text",
		"http://",
		"://example.com",
		"0.0.0.0",
		"256.1.1.1",
		"example",
		"example..com",
		"example .com",
		"..example.com",
		".com",
		"http://.com",
		"2001:db8:::1",
		"about: blank",
		"about:",
		"chrome:",
		"chrome://",
		"moz-extension://",
		"1", "1/", "1/2", "1.2", "1.2:80", "1.2/", "1.2/3",
		"1.2.3", "1.2.3:80", "1.2.3/4",
		"192.168", "192.168:80", "192.168/", "192.168/2",
	}

	for _, raw := range invalid {
		if IsURL(raw) {
			t.Errorf("%q should not be a URL", raw)
		}
	}
}

// fakeOpener stands in for the browser, so that a test watches what the
// plugin asked to open rather than opening it.
type fakeOpener struct {
	opened     []string
	inBrowser  []string
	newWindows []bool
	privates   []bool
}

func (f *fakeOpener) Open(rawURL string) error {
	f.opened = append(f.opened, rawURL)
	return nil
}

func (f *fakeOpener) OpenInBrowser(rawURL, browserPath string, newWindow, private bool, privateArgument string) error {
	f.inBrowser = append(f.inBrowser, rawURL)
	f.newWindows = append(f.newWindows, newWindow)
	f.privates = append(f.privates, private)
	return nil
}

func newTestPlugin(t *testing.T) (*Plugin, *fakeOpener) {
	t.Helper()

	plugin := New()
	opener := &fakeOpener{}
	plugin.opener = opener
	plugin.api = core.NewPublicAPI(nil, t.TempDir(), "test")
	plugin.words = core.NewPluginTranslator(languages, "Languages")
	plugin.settings = DefaultSettings()
	return plugin, opener
}

func TestQueryOpensAUrl(t *testing.T) {
	plugin, opener := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{Search: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want one", len(results))
	}
	if results[0].Title != "example.com" {
		t.Errorf("the title is %q", results[0].Title)
	}
	if results[0].Score != 8 {
		t.Errorf("the result scores %d, want 8", results[0].Score)
	}

	results[0].Action(core.ActionContext{})
	if len(opener.opened) != 1 {
		t.Fatalf("the browser was asked to open %d URLs, want 1", len(opener.opened))
	}
	// A URL typed without a scheme is opened over http, as the reference
	// does.
	if opener.opened[0] != "http://example.com" {
		t.Errorf("the browser was asked to open %q, want http://example.com", opener.opened[0])
	}
}

func TestSchemeIsKept(t *testing.T) {
	plugin, opener := newTestPlugin(t)

	results, _ := plugin.Query(context.Background(), core.Query{Search: "https://example.com"})
	results[0].Action(core.ActionContext{})

	if len(opener.opened) != 1 || opener.opened[0] != "https://example.com" {
		t.Errorf("the browser was asked to open %v, want https://example.com", opener.opened)
	}
}

func TestHttpsPreference(t *testing.T) {
	plugin, opener := newTestPlugin(t)
	plugin.settings.AlwaysOpenWithHttps = true

	results, _ := plugin.Query(context.Background(), core.Query{Search: "example.com"})
	results[0].Action(core.ActionContext{})

	if len(opener.opened) != 1 || opener.opened[0] != "https://example.com" {
		t.Errorf("the browser was asked to open %v, want https://example.com", opener.opened)
	}
}

func TestQueryLeavesASearchAlone(t *testing.T) {
	plugin, _ := newTestPlugin(t)

	results, err := plugin.Query(context.Background(), core.Query{Search: "how to write a launcher"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results for a search, want none", len(results))
	}
}

func TestCustomBrowserIsUsed(t *testing.T) {
	plugin, opener := newTestPlugin(t)
	plugin.settings.UseCustomBrowser = true
	plugin.settings.BrowserPath = "C:/browsers/other.exe"
	plugin.settings.OpenInPrivateMode = true
	plugin.settings.PrivateModeArgument = "--incognito"

	results, _ := plugin.Query(context.Background(), core.Query{Search: "example.com"})
	results[0].Action(core.ActionContext{})

	if len(opener.inBrowser) != 1 {
		t.Fatalf("the custom browser was asked to open %d URLs, want 1", len(opener.inBrowser))
	}
	if len(opener.opened) != 0 {
		t.Error("the system browser was used although a custom one is set")
	}
	if !opener.privates[0] {
		t.Error("the URL was not opened privately")
	}
}

func TestBracketedIPv6(t *testing.T) {
	cases := map[string]string{
		"2001:db8::1":      "[2001:db8::1]",
		"[2001:db8::1]":    "[2001:db8::1]",
		"::1":              "[::1]",
		"2001:db8::1/path": "[2001:db8::1]/path",
		"example.com":      "example.com",
		"192.168.1.1":      "192.168.1.1",
	}
	for input, want := range cases {
		if got := bracketIPv6(input); got != want {
			t.Errorf("bracketIPv6(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestURLValidationMatchesTheReference checks what counts as a URL against
// the reference's own code, which is copied into the oracle unchanged, over
// the cases its tests cover and the corners around them.
func TestURLValidationMatchesTheReference(t *testing.T) {
	inputs := []string{
		// The reference's own cases.
		"http://www.google.com", "https://www.google.com", "www.google.com", "google.com",
		"localhost", "localhost:8080", "110.10.10.10", "[::1]", "[2001:db8::1]:8080",
		"2001:db8::1", "fe80:1:2::3:4", "::1", "EXAMPLE.COM", "example.com/path",
		"192.168.1.1/path/to/resource?query=1", "chrome://settings", "about:blank",
		"data:text/plain,hello", "chrome:settings", "file:///C:/path/to/file.txt",
		"wwww", "not a url", "http://", "://example.com", "0.0.0.0", "256.1.1.1",
		"example", "example..com", "example .com", ".com", "2001:db8:::1",
		"about: blank", "about:", "chrome://", "1", "1.2", "1.2.3", "192.168",
		"192.168:80", "192.168/2", "2001:db8::1/path",
		// The corners around them.
		"1.2.3.4", "1.2.3.4:80", "1.2.3.4:0", "1.2.3.4:99999", "example.com:99999",
		"255.255.255.255", "1.1.1.256", "01.2.3.4", "123.456", "123.com", "1.example",
		"1.2.3.example", "1.2.3.4.5", "1.2.3.4.5.6", "999.999.999.999", "256.256.256.256",
		"2001:db8::1:8080", "[2001:db8::1]", "[2001:db8::1]/path", "1.2345", "42",
		"moz-extension://abc123/", "moz-extension://", "edge://about", "opera://history",
		"", "   ", "http://.com", "https://localhost:80", "HTTP://EXAMPLE.COM",
		"data:,Hello%2C%20World%21", "chrome-extension:settings", "chrome-extension://abc/",
	}

	cases := make([]any, 0, len(inputs))
	for _, input := range inputs {
		cases = append(cases, map[string]any{"kind": "isURL", "input": input})
	}

	answers := oracle.Ask(t, cases...)
	for i, input := range inputs {
		want, _ := answers[i]["isURL"].(bool)
		if got := IsURL(input); got != want {
			t.Errorf("IsURL(%q) = %v, the reference says %v", input, got, want)
		}
	}
}
