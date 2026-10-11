package program

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"launch/core"
)

// systemFinder looks for the programs the machine has, and runs them.
type systemFinder struct{}

// Find returns the programs the settings ask for: what the Start Menu
// holds, what the registry lists, and what is on the PATH.
func (f systemFinder) Find(settings Settings) ([]Program, error) {
	suffixes := settings.Suffixes()
	protocols := settings.Protocols()

	var programs []Program
	seen := map[string]bool{}

	add := func(program Program) {
		if program.UniqueIdentifier == "" || seen[program.UniqueIdentifier] {
			return
		}
		seen[program.UniqueIdentifier] = true
		program.Enabled = true
		programs = append(programs, program)
	}

	if settings.EnableStartMenuSource {
		for _, directory := range startMenuDirectories() {
			for _, path := range findFiles(directory, suffixes) {
				add(programFromPath(path, protocols))
			}
		}
	}

	if settings.EnableRegistrySource {
		for _, path := range appPathsPrograms() {
			add(programFromPath(path, protocols))
		}
	}

	if settings.EnablePathSource {
		for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
			for _, path := range findFiles(directory, suffixes) {
				add(programFromPath(path, protocols))
			}
		}
	}

	// The directories the user added are looked in as well, and what they
	// hold is listed even when it is not a program.
	for _, source := range settings.ProgramSources {
		if !source.Enabled || !core.DirectoryExists(source.Location) {
			continue
		}
		for _, path := range findFiles(source.Location, suffixes) {
			add(programFromPath(path, protocols))
		}
	}

	sort.SliceStable(programs, func(i, j int) bool { return programs[i].Name < programs[j].Name })
	return programs, nil
}

// Start runs a program.
func (systemFinder) Start(program Program, arguments string, runAsAdmin bool) error {
	return startProgram(program.ProgramFile(), program.ParentDirectory, arguments, runAsAdmin)
}

// StartAsDifferentUser runs one as another user.
func (systemFinder) StartAsDifferentUser(program Program, arguments string) error {
	return startAsDifferentUser(program.ProgramFile(), program.ParentDirectory, arguments)
}

// programFromPath makes a program out of a file, reading what the file says
// about itself.
func programFromPath(path string, protocols []string) Program {
	program := Program{
		Name:             filepath.Base(path),
		FullPath:         path,
		UniqueIdentifier: path,
		ParentDirectory:  filepath.Dir(path),
		IcoPath:          path,
		Enabled:          true,
	}
	if extension := filepath.Ext(path); extension != "" {
		program.Name = strings.TrimSuffix(program.Name, extension)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".lnk":
		target, description, arguments := core.ReadShortcut(path)
		if target != "" && core.FileExists(target) {
			program.LnkResolvedPath = target
			program.ExecutableName = strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))
			program.Args = arguments
		}
		program.Description = description
		if program.Description == "" {
			program.Description = fileDescription(target)
		}
		program.LocalizedName = core.ShortcutDescription(path)
	case ".url":
		url, icon := internetShortcut(path)
		if !hasProtocol(url, protocols) {
			// A shortcut to somewhere the user did not ask to search is
			// not a program.
			return Program{}
		}
		program.LnkResolvedPath = url
		program.Description = url
		if icon != "" {
			program.IcoPath = icon
		}
	case ".exe":
		program.Description = fileDescription(path)
	}
	return program
}

// hasProtocol reports whether a URL is one the plugin was asked to look
// for.
func hasProtocol(url string, protocols []string) bool {
	if url == "" {
		return false
	}
	for _, protocol := range protocols {
		if strings.HasPrefix(strings.ToLower(url), strings.ToLower(protocol)) {
			return true
		}
	}
	return false
}

// findFiles lists the files in a directory whose extension is one of the
// suffixes, looking inside its subdirectories too.
func findFiles(directory string, suffixes []string) []string {
	if directory == "" || !core.DirectoryExists(directory) {
		return nil
	}

	wanted := map[string]bool{}
	for _, suffix := range suffixes {
		wanted["."+strings.ToLower(strings.TrimPrefix(suffix, "."))] = true
	}

	var files []string
	_ = filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			// A directory the user may not look into is left alone.
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if wanted[strings.ToLower(filepath.Ext(path))] {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// startMenuDirectories are where Windows keeps its shortcuts.
func startMenuDirectories() []string {
	var directories []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		directories = append(directories,
			filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if programData := os.Getenv("ProgramData"); programData != "" {
		directories = append(directories,
			filepath.Join(programData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	return directories
}

// internetShortcut reads the URL and the icon of a .url file, which is an
// INI file with one section.
func internetShortcut(path string) (url, icon string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "url":
			url = strings.TrimSpace(value)
		case "iconfile":
			icon = strings.TrimSpace(value)
		}
	}
	return url, icon
}

// writeFile writes a file, for the tests that need one to read.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
