package url

import (
	"context"
	"embed"
	"net"
	"strconv"
	"strings"
	"unicode"

	"launch/core"
)

// The url plugin's own words.
//
//go:embed Languages/*.xaml
var languages embed.FS

// Settings is what the user can change about opening a URL.
type Settings struct {
	// UseCustomBrowser opens URLs in a browser of the user's own rather
	// than in the one the system has.
	UseCustomBrowser bool `json:"UseCustomBrowser"`
	// BrowserPath is that browser.
	BrowserPath string `json:"BrowserPath"`
	// OpenInNewBrowserWindow opens a new window rather than a new tab.
	OpenInNewBrowserWindow bool `json:"OpenInNewBrowserWindow"`
	// OpenInPrivateMode opens a private window.
	OpenInPrivateMode bool `json:"OpenInPrivateMode"`
	// PrivateModeArgument is what the browser is told to open one with,
	// such as --incognito.
	PrivateModeArgument string `json:"PrivateModeArgument"`
	// AlwaysOpenWithHttps opens a URL typed without a scheme over https.
	AlwaysOpenWithHttps bool `json:"AlwaysOpenWithHttps"`
}

// DefaultSettings are the settings a user who changed nothing gets.
func DefaultSettings() Settings {
	return Settings{OpenInNewBrowserWindow: true}
}

// Opener opens a URL. It is what the plugin does to the machine, kept
// behind a field so that a test can watch what it was asked to open.
type Opener interface {
	// Open opens a URL in the browser the system has.
	Open(rawURL string) error
	// OpenInBrowser opens one in a browser of the user's own, in a new
	// window or tab, and privately or not.
	OpenInBrowser(rawURL, browserPath string, newWindow, private bool, privateArgument string) error
}

// Plugin is the url plugin: it opens what the user types when it is a URL.
type Plugin struct {
	core.BasePlugin

	api      *core.PublicAPI
	settings Settings
	words    *core.PluginTranslator
	opener   Opener
}

// New makes the url plugin.
func New() *Plugin {
	return &Plugin{
		BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{
			ID:             "0308FD86DE0A4DEE8D62B9B535370992",
			Name:           "URL",
			Description:    "Open the typed URL from Flow Launcher",
			Author:         "qianlifeng",
			Version:        "1.0.0",
			Website:        "https://github.com/Flow-Launcher/Flow.Launcher",
			ActionKeywords: []string{"*"},
			IcoPath:        "Images/url.png",
		}},
		opener: systemOpener{},
	}
}

// Init reads the url plugin's settings.
func (p *Plugin) Init(ctx *core.PluginInitContext) error {
	p.api = ctx.API
	p.words = core.NewPluginTranslator(languages, "Languages")
	p.settings = core.LoadSettingJsonStorage[Settings](ctx.API, "URL", "Settings.json")
	return nil
}

// Query answers a URL with the one result that opens it.
func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {
	raw := strings.TrimSpace(query.Search)
	if !p.IsURL(raw) {
		return nil, nil
	}

	// An IPv6 address written without brackets is bracketed, so that it
	// can be opened as a URL.
	raw = bracketIPv6(raw)

	title := raw
	result := core.NewResult(title, p.words.Tf("flowlauncher_plugin_url_open_url", raw), "Images/url.png", 8)
	result.SetCopyText(raw)
	result.Action = func(core.ActionContext) bool {
		return p.open(raw)
	}
	return []core.Result{result}, nil
}

// open opens the URL, adding the scheme the user left out.
func (p *Plugin) open(raw string) bool {
	hasScheme := hasKnownScheme(raw)
	if !hasScheme {
		raw = p.httpPreference() + "://" + raw
	}

	var err error
	if p.settings.UseCustomBrowser && p.settings.BrowserPath != "" {
		err = p.opener.OpenInBrowser(raw, p.settings.BrowserPath,
			p.settings.OpenInNewBrowserWindow, p.settings.OpenInPrivateMode, p.settings.PrivateModeArgument)
	} else {
		err = p.opener.Open(raw)
	}

	if err != nil {
		p.api.ShowMsgError(p.words.T("flowlauncher_plugin_url_plugin_name"),
			p.words.Tf("flowlauncher_plugin_url_cannot_open_url", raw))
		return false
	}
	return true
}

// httpPreference is the scheme a URL typed without one is opened with.
func (p *Plugin) httpPreference() string {
	if p.settings.AlwaysOpenWithHttps {
		return "https"
	}
	return "http"
}

// The schemes the plugin knows, which decide what is a URL and what is a
// search.
var (
	// hostValidatedSchemes are the ones whose host must be a real one: a
	// domain with a top-level part, an address, or localhost.
	hostValidatedSchemes = []string{"http", "https"}
	// chromiumSchemes are the browsers' own, which take both forms.
	chromiumSchemes = []string{"chrome-extension", "chrome", "brave", "edge", "opera", "vivaldi"}
	// nonHostValidatedSchemes are read by their scheme alone.
	nonHostValidatedSchemes = append(append([]string{}, chromiumSchemes...), "file", "moz-extension")
	// colonOnlySchemes are written without the two slashes.
	colonOnlySchemes = append(append([]string{}, chromiumSchemes...), "about", "data")
	// allDoubleSlashSchemes are every scheme written with two slashes.
	allDoubleSlashSchemes = append(append([]string{}, hostValidatedSchemes...), nonHostValidatedSchemes...)
)

// IsURL reports whether what the user typed is a URL rather than a search.
func (p *Plugin) IsURL(raw string) bool { return IsURL(raw) }

// IsURL reports whether text is a URL the plugin should open.
func IsURL(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	input := strings.TrimSpace(raw)

	// A number is a number, not a URL.
	if _, err := strconv.ParseFloat(input, 64); err == nil {
		return false
	}

	// A bare address, with or without a port, path, query or fragment.
	if matched, valid := matchBareIP(input); matched {
		return valid
	}

	// A scheme written without the two slashes.
	if matched, valid := matchColonScheme(input); matched {
		return valid
	}

	// Everything else is read as a URL with the two slashes.
	urlToValidate := input
	if !startsWithAny(input, allDoubleSlashSchemes, "://") {
		urlToValidate = "http://" + input
	}

	parsed, err := parseAbsoluteURL(urlToValidate)
	if err != nil {
		return false
	}

	// A scheme the plugin does not know is left to the other plugins.
	if !contains(allDoubleSlashSchemes, parsed.scheme) {
		return false
	}

	if matched, valid := matchNonHostScheme(parsed, input); matched {
		return valid
	}

	return validateSchemeHost(parsed)
}

// matchBareIP reads an address written on its own. It reports whether the
// text looks like an address at all, and whether it is one to open: an
// address of all zeros is not.
func matchBareIP(input string) (matched, valid bool) {
	addressPart := input
	if index := strings.IndexAny(input, "/?#"); index >= 0 {
		addressPart = input[:index]
	}

	address, ok := parseEndpoint(addressPart)
	if !ok {
		return false, false
	}

	if address.To4() != nil {
		// An IPv4 address needs all four of its parts.
		if len(strings.Split(addressPart, ".")) != 4 {
			return true, false
		}
		// 0.0.0.0 is the address that means "any", and is not one to open.
		return true, !address.IsUnspecified()
	}

	// An IPv6 address with a path, query or fragment has to be bracketed,
	// or the colons cannot be told from the ones that follow it.
	if strings.ContainsAny(input, "/?#") {
		if !strings.HasPrefix(input, "[") || !strings.Contains(input, "]") {
			return true, false
		}
	}
	return true, !address.IsUnspecified()
}

// parseEndpoint reads an address, with or without a port, the way the
// reference's IPEndPoint.TryParse does: an address on its own, an address
// and a port, or a bracketed IPv6 address with a port after it.
func parseEndpoint(text string) (net.IP, bool) {
	if strings.HasPrefix(text, "[") {
		end := strings.Index(text, "]")
		if end < 0 {
			return nil, false
		}
		address := net.ParseIP(text[1:end])
		if address == nil {
			return nil, false
		}
		rest := text[end+1:]
		switch {
		case rest == "":
			return address, true
		case strings.HasPrefix(rest, ":"):
			return address, validPort(rest[1:])
		default:
			return nil, false
		}
	}

	// The whole text may be an address of its own, which is how a bare
	// IPv6 address is read: the colons in it are not a port separator.
	if address := net.ParseIP(text); address != nil {
		return address, true
	}

	// Otherwise the last colon, if there is one, separates a port. Only an
	// IPv4 address is written that way: an IPv6 address with a port has to
	// be bracketed, or its own colons cannot be told from the one before
	// the port.
	if index := strings.LastIndex(text, ":"); index > 0 {
		address := net.ParseIP(text[:index])
		if address == nil || address.To4() == nil {
			return nil, false
		}
		return address, validPort(text[index+1:])
	}
	return nil, false
}

// validPort reports whether text is a port number the system would take.
func validPort(text string) bool {
	port, err := strconv.Atoi(text)
	return err == nil && port >= 0 && port <= 65535
}

// matchColonScheme reads a scheme written without the two slashes, as
// about:blank is.
func matchColonScheme(input string) (matched, valid bool) {
	colonIndex := strings.Index(input, ":")
	if colonIndex <= 0 {
		return false, false
	}

	scheme := strings.ToLower(input[:colonIndex])
	if !contains(colonOnlySchemes, scheme) {
		return false, false
	}
	content := input[colonIndex+1:]
	if strings.HasPrefix(content, "//") {
		// That form is read as a URL with two slashes.
		return false, false
	}

	return true, content != "" && !strings.ContainsFunc(content, unicode.IsSpace)
}

// matchNonHostScheme reads a scheme that is understood by itself, without
// its host having to be a real one.
func matchNonHostScheme(parsed parsedURL, input string) (matched, valid bool) {
	if !contains(nonHostValidatedSchemes, parsed.scheme) {
		return false, false
	}
	prefix := parsed.scheme + "://"
	return true, len(input) > len(prefix)
}

// validateSchemeHost reports whether the host of a URL is one to open: a
// name with a top-level part, an address that is not "any", or localhost.
func validateSchemeHost(parsed parsedURL) bool {
	host := parsed.host
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if address := net.ParseIP(host); address != nil {
		return !address.IsUnspecified()
	}

	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
	}

	// A host of digits and dots only is read as an address, and one that is
	// not an address of four parts is not one to open: "1.2.3" is not a
	// URL, and neither is "192.168", which is how the reference's engine
	// reads them. Its first three parts must each fit in a byte, while the
	// last may be any number, as an address may be written.
	//
	// A last part larger than a byte also lifts that check on the three
	// before it, which is what the reference's engine does: it takes
	// 1.999.999.999 and 256.256.256.256, and refuses 256.1.1.1. The
	// oddity is copied rather than corrected, since what counts as a URL
	// decides what the launcher opens.
	if allNumeric(parts) {
		if len(parts) != 4 {
			return false
		}
		last, err := strconv.ParseUint(parts[3], 10, 64)
		if err != nil {
			return false
		}
		if last > 255 {
			return true
		}
		for _, part := range parts[:3] {
			value, err := strconv.Atoi(part)
			if err != nil || value < 0 || value > 255 {
				return false
			}
		}
		return true
	}

	topLevel := parts[len(parts)-1]
	if len(topLevel) < 2 {
		return false
	}
	for _, r := range topLevel {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// allNumeric reports whether every part of a host is made of digits.
func allNumeric(parts []string) bool {
	for _, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// bracketIPv6 puts brackets around an IPv6 address written without them,
// which is what makes it a URL.
func bracketIPv6(raw string) string {
	if strings.HasPrefix(raw, "[") {
		return raw
	}
	host, rest := raw, ""
	if index := strings.IndexAny(raw, "/?#"); index >= 0 {
		host, rest = raw[:index], raw[index:]
	}
	if strings.Count(host, ":") < 2 {
		return raw
	}
	if address := net.ParseIP(host); address == nil || address.To4() != nil {
		return raw
	}
	return "[" + host + "]" + rest
}

// hasKnownScheme reports whether a URL already names its scheme.
func hasKnownScheme(raw string) bool {
	if startsWithAny(raw, allDoubleSlashSchemes, "://") {
		return true
	}
	for _, scheme := range colonOnlySchemes {
		if len(raw) > len(scheme)+1 && strings.EqualFold(raw[:len(scheme)+1], scheme+":") {
			return true
		}
	}
	return false
}

// parsedURL is the part of a URL the plugin reads.
type parsedURL struct {
	scheme string
	host   string
}

// parseAbsoluteURL reads a URL the way the reference does: it must have a
// scheme, and nothing but a scheme, before its two slashes.
func parseAbsoluteURL(raw string) (parsedURL, error) {
	index := strings.Index(raw, "://")
	if index <= 0 {
		return parsedURL{}, errNotAURL
	}

	scheme := strings.ToLower(raw[:index])
	for i, r := range scheme {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		case (r == '+' || r == '-' || r == '.') && i > 0:
		default:
			return parsedURL{}, errNotAURL
		}
	}

	rest := raw[index+3:]
	if strings.ContainsAny(rest, " \t") {
		return parsedURL{}, errNotAURL
	}
	authority := rest
	if cut := strings.IndexAny(rest, "/?#"); cut >= 0 {
		authority = rest[:cut]
	}

	host, port := splitHostPort(authority)
	// A port the system would not take is not a URL, as the reference's
	// engine reads it.
	if port != "" && !validPort(port) {
		return parsedURL{}, errNotAURL
	}
	// An address in a URL is written in brackets, so that its own colons
	// cannot be taken for the one before a port. A host with a colon in it
	// that is not bracketed is not a host.
	if !strings.HasPrefix(authority, "[") && strings.Contains(host, ":") {
		return parsedURL{}, errNotAURL
	}

	return parsedURL{scheme: scheme, host: host}, nil
}

// splitHostPort separates the host from the port of an authority, keeping
// an IPv6 address in brackets whole.
func splitHostPort(authority string) (host, port string) {
	if strings.HasPrefix(authority, "[") {
		end := strings.Index(authority, "]")
		if end < 0 {
			return authority, ""
		}
		host = authority[1:end]
		if rest := authority[end+1:]; strings.HasPrefix(rest, ":") {
			port = rest[1:]
		}
		return host, port
	}
	if index := strings.LastIndex(authority, ":"); index >= 0 {
		return authority[:index], authority[index+1:]
	}
	return authority, ""
}

func startsWithAny(value string, prefixes []string, suffix string) bool {
	for _, prefix := range prefixes {
		if len(value) >= len(prefix)+len(suffix) && strings.EqualFold(value[:len(prefix)+len(suffix)], prefix+suffix) {
			return true
		}
	}
	return false
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// errNotAURL is what parsing answers when the text is not a URL.
var errNotAURL = errorString("not a URL")

type errorString string

func (e errorString) Error() string { return string(e) }
