//go:build windows

package program

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// The Windows apps installed from the store are not programs on disk: the
// shell runs one by its application user model id, and what it is called,
// what it looks like and whether it may run as an administrator is in the
// package it came in.
//
// What each app is called comes from the shell's own list of the apps the
// Start menu shows, because a package names its apps with a reference to a
// resource ("ms-resource:AppName") rather than with a name. The rest is
// read from the package: where it is installed, its logo, and whether it
// asked to be able to run elevated.

// packagesKey is where the packages installed for this user are listed.
const packagesKey = `Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\Repository\Packages`

// uwpPrograms are the Windows apps installed from the store, as programs.
func uwpPrograms() []Program {
	locations := uwpPackageLocations()

	var programs []Program
	for _, app := range startApps() {
		family, appID, found := strings.Cut(app.id, "!")
		if !found {
			continue
		}
		location := locations[family]
		if location == "" {
			continue
		}

		logo, description, canElevate := uwpManifest(location, appID)
		programs = append(programs, Program{
			Name:             app.name,
			Description:      description,
			IcoPath:          logo,
			FullPath:         location,
			ParentDirectory:  location,
			UniqueIdentifier: app.id,
			AppUserModelID:   app.id,
			CanRunElevated:   canElevate,
			Enabled:          true,
		})
	}
	return programs
}

// startApp is one app the Start menu shows.
type startApp struct {
	// id is the application user model id the shell runs it by.
	id string
	// name is what the Start menu calls it, in the user's own language.
	name string
}

// startApps are the apps the Start menu shows, asked of the shell. A
// package's own manifest names its apps with a resource reference rather
// than with a name, so the shell is what knows what to call them.
func startApps() []startApp {
	// The output is read as UTF-8, whatever the machine's code page is.
	script := "[Console]::OutputEncoding = [Text.Encoding]::UTF8; " +
		"Get-StartApps | Where-Object { $_.AppID -like '*!*' } | " +
		"ForEach-Object { \"$($_.AppID)`t$($_.Name)\" }"

	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	// The launcher has no console of its own, so the shell is given none.
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}

	output, err := command.Output()
	if err != nil {
		return nil
	}

	var apps []startApp
	for _, line := range strings.Split(string(output), "\n") {
		id, name, found := strings.Cut(strings.TrimRight(line, "\r"), "\t")
		if !found || id == "" || name == "" {
			continue
		}
		apps = append(apps, startApp{id: id, name: name})
	}
	return apps
}

// createNoWindow keeps a program that is run from having a console window
// of its own.
const createNoWindow = 0x08000000

// uwpPackageLocations are where the packages are installed, by the family
// name an application user model id starts with.
func uwpPackageLocations() map[string]string {
	key, err := registry.OpenKey(registry.CURRENT_USER, packagesKey, registry.READ)
	if err != nil {
		return nil
	}
	defer key.Close()

	names, err := key.ReadSubKeyNames(0)
	if err != nil {
		return nil
	}

	locations := map[string]string{}
	for _, name := range names {
		family := packageFamilyName(name)
		if family == "" {
			continue
		}

		sub, err := registry.OpenKey(key, name, registry.READ)
		if err != nil {
			continue
		}
		location, _, _ := sub.GetStringValue("PackageRootFolder")
		framework, _, _ := sub.GetIntegerValue("Framework")
		sub.Close()

		// A package that is a framework is a library for other packages,
		// not an app of its own.
		if location == "" || framework != 0 {
			continue
		}
		if _, err := os.Stat(filepath.Join(location, "AppxManifest.xml")); err != nil {
			continue
		}
		locations[family] = location
	}
	return locations
}

// packageFamilyName is the name a package is known by, which is its full
// name without the version, the architecture and the resource id:
// Microsoft.WindowsCalculator_8wekyb3d8bbwe for
// Microsoft.WindowsCalculator_11.2210.0.0_x64__8wekyb3d8bbwe.
func packageFamilyName(fullName string) string {
	parts := strings.Split(fullName, "_")
	if len(parts) < 5 {
		return ""
	}
	return parts[0] + "_" + parts[len(parts)-1]
}

// The manifest of a package, as much of it as is read here.
type appxManifest struct {
	Applications struct {
		Application []struct {
			ID         string `xml:"Id,attr"`
			EntryPoint string `xml:"EntryPoint,attr"`
			TrustLevel string `xml:"TrustLevel,attr"`
			Visual     struct {
				Description       string `xml:"Description,attr"`
				Square44x44Logo   string `xml:"Square44x44Logo,attr"`
				Square150x150Logo string `xml:"Square150x150Logo,attr"`
			} `xml:"VisualElements"`
		} `xml:"Application"`
	} `xml:"Applications"`
}

// uwpManifest reads what the package says about one of its apps: the
// picture to show it by, what it does, and whether it may run elevated.
func uwpManifest(location, appID string) (logo, description string, canElevate bool) {
	content, err := os.ReadFile(filepath.Join(location, "AppxManifest.xml"))
	if err != nil {
		return "", "", false
	}

	var manifest appxManifest
	if err := xml.Unmarshal(content, &manifest); err != nil {
		return "", "", false
	}

	for _, app := range manifest.Applications.Application {
		if !strings.EqualFold(app.ID, appID) {
			continue
		}
		// A package may say that an app can be elevated, or that the app
		// is a program of its own that runs with the user's own rights.
		canElevate = app.EntryPoint == "Windows.FullTrustApplication" ||
			strings.EqualFold(app.TrustLevel, "mediumIL")

		description = readableResource(app.Visual.Description)
		logo = uwpLogo(location, app.Visual.Square44x44Logo, app.Visual.Square150x150Logo)
		return logo, description, canElevate
	}
	return "", "", false
}

// readableResource is a manifest string that is a string: a package may
// name a description with a reference to a resource
// ("ms-resource:AppxManifest_Description"), which nothing here can look
// up, and a reference is not worth showing.
func readableResource(value string) string {
	if strings.HasPrefix(value, "ms-resource:") || strings.HasPrefix(value, "@{") {
		return ""
	}
	return value
}

// uwpLogo is the picture to show an app by. A package names a logo
// ("Assets\Square44x44Logo.png") and then ships it once for each screen
// scale ("Square44x44Logo.scale-100.png"), so what is on disk is not
// always what is named.
func uwpLogo(location string, names ...string) string {
	for _, name := range names {
		if name == "" {
			continue
		}
		path := filepath.Join(location, filepath.FromSlash(strings.ReplaceAll(name, `\`, "/")))
		if fileExists(path) {
			return path
		}
		if match := closestLogo(path); match != "" {
			return match
		}
	}
	return ""
}

// closestLogo is the file a named logo stands for: the same name with the
// size Windows added to it, the plainest one first.
func closestLogo(path string) string {
	extension := filepath.Ext(path)
	prefix := strings.TrimSuffix(filepath.Base(path), extension)

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), prefix+"*"+extension))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return logoRank(matches[i]) < logoRank(matches[j])
	})
	return matches[0]
}

// logoRank is how plain a logo's file name is, which is the one to show
// when the package ships several: one drawn for a screen scale of 100 is
// what the name asked for.
func logoRank(path string) int {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(name, "scale-100"):
		return 0
	case strings.Contains(name, "targetsize-44"):
		return 1
	case strings.Contains(name, "targetsize-48"):
		return 2
	}
	return 3
}

// fileExists reports whether there is a file at a path.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
