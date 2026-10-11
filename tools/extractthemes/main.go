package main

// extractthemes reads the reference's theme files and writes the colours of
// each into a Go table, so that this launcher wears the same themes with the
// same colours.
//
// The reference's themes are XAML resource dictionaries: they name a colour
// for each part of the window, and a style template says what to do with
// it. Only the colours are portable, so only they are taken: the shape of
// the window is drawn by this launcher's own view.
//
// Usage: go run ./tools/extractthemes <themes directory> <output file>

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: extractthemes <themes directory> <output file>")
		os.Exit(2)
	}

	directory, output := os.Args[1], os.Args[2]
	themes, err := extract(directory)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	file, err := os.Create(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()

	write(file, themes, filepath.Base(directory))
	fmt.Printf("wrote %d themes to %s\n", len(themes), output)
}

// theme is one theme's colours.
type theme struct {
	// FileName is the theme's file name without its extension, which the
	// settings file holds.
	FileName string
	// Name is what the settings window shows.
	Name string
	// IsDark says whether the theme is dark, and HasBlur whether it blurs
	// what is behind the window.
	IsDark  bool
	HasBlur bool
	// Colors are the theme's own colours, by the name the reference gives
	// them.
	Colors map[string]string
}

// extract reads every theme file in a directory.
func extract(directory string) ([]theme, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	var themes []theme
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xaml") {
			continue
		}
		// The base file holds the shapes rather than a theme's colours.
		if entry.Name() == "Base.xaml" {
			continue
		}

		path := filepath.Join(directory, entry.Name())
		parsed, err := parseTheme(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			continue
		}
		parsed.FileName = strings.TrimSuffix(entry.Name(), ".xaml")
		themes = append(themes, parsed)
	}

	sort.Slice(themes, func(i, j int) bool { return themes[i].Name < themes[j].Name })
	return themes, nil
}

// parseTheme reads one theme file: the comment at its top says what it is
// called and how it behaves, and its styles name the colours.
func parseTheme(path string) (theme, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return theme{}, err
	}

	result := theme{Colors: map[string]string{}}
	readMetadata(string(content), &result)
	if result.Name == "" {
		result.Name = strings.TrimSuffix(filepath.Base(path), ".xaml")
	}
	if err := readColors(content, &result); err != nil {
		return result, err
	}
	return result, nil
}

// readMetadata reads the comment the reference puts at the top of a theme:
//
//	<!--
//	    Name: Blur Black
//	    IsDark: False
//	    HasBlur: True
//	-->
func readMetadata(content string, result *theme) {
	start := strings.Index(content, "<!--")
	end := strings.Index(content, "-->")
	if start < 0 || end < 0 || end < start {
		return
	}

	result.Name = result.FileName
	for _, line := range strings.Split(content[start+4:end], "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(strings.ToLower(line), "name:"):
			result.Name = strings.TrimSpace(line[len("name:"):])
		case strings.HasPrefix(strings.ToLower(line), "isdark:"):
			result.IsDark = strings.EqualFold(strings.TrimSpace(line[len("isdark:"):]), "true")
		case strings.HasPrefix(strings.ToLower(line), "hasblur:"):
			result.HasBlur = strings.EqualFold(strings.TrimSpace(line[len("hasblur:"):]), "true")
		}
	}
}

// readColors takes the colour of each part of the window out of the theme's
// styles.
func readColors(content []byte, result *theme) error {
	decoder := xml.NewDecoder(strings.NewReader(string(content)))
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	// currentKey is the style being read, and currentProperty the property
	// of it, so that a colour knows what it colours.
	currentKey := ""
	currentProperty := ""

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// A theme that cannot be read whole still gives what was read.
			return nil
		}

		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "Style":
				currentKey = attribute(element, "Key")
			case "SolidColorBrush":
				key := attribute(element, "Key")
				if key != "" {
					currentKey = key
				}
			case "Setter":
				currentProperty = attribute(element, "Property")
				if value := attribute(element, "Value"); value != "" {
					record(result, currentKey, currentProperty, value)
				}
			}
		case xml.CharData:
			// A colour written as the text of a brush rather than as a
			// property value.
			if currentKey != "" && currentProperty == "" {
				text := strings.TrimSpace(string(element))
				if isColor(text) {
					record(result, currentKey, "", text)
				}
			}
		case xml.EndElement:
			if element.Name.Local == "Style" {
				currentKey = ""
			}
			if element.Name.Local == "Setter" {
				currentProperty = ""
			}
		}
	}
	return nil
}

// record keeps the colours the window is drawn with, which are the ones
// this launcher can wear.
func record(result *theme, key, property, value string) {
	if !isColor(value) {
		return
	}
	switch key {
	case "ItemSelectedBackgroundColor":
		result.Colors["selectedBackground"] = value
	case "HighlightStyle":
		result.Colors["highlight"] = value
	default:
		if property == "" {
			return
		}
		// The style keys the reference uses, and what they colour here.
		names := map[string]string{
			"QueryBoxStyle":             "queryForeground",
			"WindowBorderStyle":         "background",
			"ItemTitleStyle":            "title",
			"ItemSubTitleStyle":         "subtitle",
			"ItemTitleSelectedStyle":    "titleSelected",
			"ItemSubTitleSelectedStyle": "subtitleSelected",
			"PreviewItemTitleStyle":     "previewTitle",
			"PreviewItemSubTitleStyle":  "previewSubtitle",
			"ThumbStyle":                "scrollbar",
		}
		name, ok := names[key]
		if !ok {
			return
		}
		if property != "Foreground" && property != "Background" && property != "BorderBrush" && property != "CaretBrush" && property != "SelectionBrush" {
			return
		}
		if name == "background" && property != "Background" {
			// The border colour is kept apart from the window's own.
			if property == "BorderBrush" {
				result.Colors["border"] = value
			}
			return
		}
		if property != "Foreground" && property != "Background" {
			return
		}
		result.Colors[name] = value
	}
}

func attribute(element xml.StartElement, name string) string {
	for _, attribute := range element.Attr {
		if attribute.Name.Local == name {
			return attribute.Value
		}
	}
	return ""
}

// isColor reports whether a value is a colour the launcher can wear.
func isColor(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return true
	}
	// A gradient is not a colour, and neither is a brush name.
	return false
}

// write writes the Go table.
func write(output io.Writer, themes []theme, source string) {
	fmt.Fprintf(output, `package core

// Code generated by tools/extractthemes from the reference's themes. DO NOT EDIT.
//
// Source: %s of github.com/Flow-Launcher/Flow.Launcher

// ThemeColors are the colours of one of the reference's themes, by the part
// of the window they colour. A theme need not name every one of them: what
// it leaves out is taken from the theme the launcher draws with.
type ThemeColors struct {
`, source)

	names := []string{
		"background", "border", "title", "subtitle", "titleSelected", "subtitleSelected",
		"selectedBackground", "queryForeground", "highlight", "scrollbar",
		"previewTitle", "previewSubtitle",
	}
	for _, name := range names {
		fmt.Fprintf(output, "\t%s string\n", exported(name))
	}

	fmt.Fprintf(output, `}

// ReferenceThemes are the reference's themes, in the order the settings
// window shows them.
var ReferenceThemes = []ReferenceTheme{
`)

	for _, t := range themes {
		fmt.Fprintf(output, "\t{\n")
		fmt.Fprintf(output, "\t\tFileName: %q,\n", t.FileName)
		fmt.Fprintf(output, "\t\tName:     %q,\n", t.Name)
		fmt.Fprintf(output, "\t\tIsDark:   %v,\n", t.IsDark)
		fmt.Fprintf(output, "\t\tHasBlur:  %v,\n", t.HasBlur)
		fmt.Fprintf(output, "\t\tColors: ThemeColors{\n")
		for _, name := range names {
			if value, ok := t.Colors[name]; ok {
				fmt.Fprintf(output, "\t\t\t%s: %q,\n", exported(name), normalise(value))
			}
		}
		fmt.Fprintf(output, "\t\t},\n\t},\n")
	}

	fmt.Fprintf(output, "}\n")
}

// normalise writes a colour the way Go's own colour parser reads it: the
// reference writes some of them with an alpha channel first, as #AARRGGBB,
// where the rest of the world writes #RRGGBBAA.
func normalise(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "#") {
		return value
	}
	digits := value[1:]
	if len(digits) == 8 {
		// #AARRGGBB becomes #RRGGBBAA.
		return "#" + digits[2:] + digits[:2]
	}
	return value
}

func exported(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
}
