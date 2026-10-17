package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// version is what the launcher reports to its plugins.
const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// A second launch hands its arguments to the one already running,
	// which is how a hotkey or a file association reaches it.
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}

	// The name decides where the settings are kept, so it comes first.
	mygo.App.SetName("Flow Launcher")
	mygo.App.SetVersion(version)

	dataDir, err := dataDirectory()
	if err != nil {
		return fmt.Errorf("finding where to keep the settings failed: %w", err)
	}

	a, err := newApp(dataDir)
	if err != nil {
		return err
	}

	// Looking at the interface while it is built: a picture of the window
	// is written and the launcher quits. A query on the command line is
	// searched first, so that what the window shows can be looked at.
	if capture := capturePath(); capture != "" {
		// The window can only be made once the application is ready, so
		// the picture is taken from there.
		mygo.App.WhenReady(func() {
			if query, _ := parseArgs(os.Args[1:]); query != "" {
				loadPlugins(a)
				a.query = query
				a.searched = query
				a.results = a.engine.Search(context.Background(), query)
			}
			a.createWindow()
			a.captureTo(capture)
		})
		return mygo.App.Run()
	}

	// A query on the command line runs the launcher's own pipeline and
	// prints what it found, without a window: how the search is checked
	// while it is built, and how a script drives the launcher.
	if query, run := parseArgs(os.Args[1:]); query != "" || run {
		return a.runQueryFromCommandLine(query, run)
	}

	// The launcher is a background app with a window: no Dock icon on
	// macOS, and no taskbar button.
	if runtime.GOOS == "darwin" {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	}
	mygo.App.OnWindowAllClosed(func() {
		if runtime.GOOS != "darwin" {
			mygo.App.Quit()
		}
	})
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			a.show()
		}
	})
	// A second launch hands its query to the one already running, which is
	// how the hotkey and the command line reach it.
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		a.onSecondInstance(args, workingDir)
	})

	mygo.App.WhenReady(func() {
		if err := a.start(); err != nil {
			log.Println("the launcher failed to start:", err)
			mygo.App.Quit()
		}
	})

	return mygo.App.Run()
}

// start creates the window, the tray icon and the global hotkey, and loads
// the plugins.
func (a *app) start() error {
	a.createWindow()
	a.createTray()
	a.registerHotkey()
	a.applyStartupSetting()

	if capture := capturePath(); capture != "" {
		// Looking at the interface while building it: show the window,
		// write what it draws to a file, and quit. A query on the command
		// line is searched first, so that what the window shows can be
		// looked at.
		if query, _ := parseArgs(os.Args[1:]); query != "" {
			loadPlugins(a)
			a.query = query
			a.results = a.engine.Search(context.Background(), query)
			fmt.Printf("capture: %d results for %q\n", len(a.results), query)
		}
		a.captureTo(capture)
		return nil
	}

	go loadPlugins(a)

	// The launcher starts hidden when it is run at login, and shows itself
	// otherwise, as the reference does.
	if !a.settings.HideOnStartup || !mygo.App.WasOpenedAtLogin() {
		a.show()
	}
	return nil
}

// createWindow makes the launcher's window, which starts hidden.
func (a *app) createWindow() {
	// From here on the clipboard can be read, which is answered on the
	// thread the window lives on.
	windowUp.Store(true)
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "Flow Launcher",
		Width:             int(a.settings.WindowSize),
		Height:            int(a.settings.WindowHeightSize) + a.settings.MaxResultsToShow*resultHeight,
		MinWidth:          320,
		Frameless:         true,
		DisableResize:     true,
		DisableMaximize:   true,
		DisableMinimize:   true,
		DisableFullScreen: true,
		SkipTaskbar:       true,
		AlwaysOnTop:       a.settings.ShowAtTopmost,
		Hidden:            true,
		Content:           ui.View(a.view),
	})
	a.win.OnBlur(func() {
		// A window being captured is not one the user has left.
		if a.settings.HideWhenDeactivated && capturePath() == "" {
			a.hide()
		}
	})
}

// applyStartupSetting makes the launcher start when the user logs in, or
// stop doing so, which is what the setting says.
func (a *app) applyStartupSetting() {
	wanted := a.settings.StartFlowLauncherOnSystemStartup
	if mygo.App.OpenAtLogin() == wanted {
		return
	}
	if err := a.setOpenAtLogin(wanted); err != nil {
		a.api.LogError("launcher", "starting at login could not be set: "+err.Error())
	}
}

// show brings the launcher up, on the screen the settings ask for, with the
// query box ready for typing.
func (a *app) show() {
	if a.win == nil || a.gameMode() {
		return
	}
	// What the user is working in, before the launcher takes the front:
	// choosing a clipboard item pastes into it.
	a.rememberForeground()
	a.positionWindow()
	a.win.Show()
	a.win.Focus()
	a.focusQuery()
	a.update(func() {})
}

// hide takes the launcher down.
func (a *app) hide() {
	if a.win == nil {
		return
	}
	a.win.Hide()
	a.saveSettings()
}

// focusQuery puts the caret in the query box.
func (a *app) focusQuery() {
	a.input.Focus()
}

// positionWindow puts the window where the settings say, and sizes it to
// what it shows.
func (a *app) positionWindow() {
	if a.win == nil {
		return
	}

	display := a.selectedDisplay()
	area := display.WorkArea
	width := int(a.settings.WindowSize)

	height := int(a.settings.WindowHeightSize)
	if shown := len(a.results); shown > 0 {
		if shown > a.settings.MaxResultsToShow {
			shown = a.settings.MaxResultsToShow
		}
		height += shown * resultHeight
	}

	x := area.X + (area.Width-width)/2
	y := area.Y + (area.Height-int(a.settings.WindowHeightSize))/4

	switch a.settings.SearchWindowAlign {
	case core.SearchWindowAlignTop:
		y = area.Y + 10
	case core.SearchWindowAlignBottom:
		y = area.Y + area.Height - height - 10
	}

	a.lastWidth, a.lastHeight = width, height
	a.win.SetBounds(mygo.Rectangle{X: x, Y: y, Width: width, Height: height})
}

// selectedDisplay is the screen the launcher opens on.
func (a *app) selectedDisplay() mygo.Display {
	switch a.settings.SearchWindowScreen {
	case core.SearchWindowScreenPrimary:
		return mygo.Screen.PrimaryDisplay()
	case core.SearchWindowScreenCustom:
		displays := mygo.Screen.Displays()
		index := a.settings.CustomScreenNumber - 1
		if index >= 0 && index < len(displays) {
			return displays[index]
		}
		return mygo.Screen.PrimaryDisplay()
	default:
		return mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	}
}

// registerHotkey binds the key that shows the launcher.
func (a *app) registerHotkey() {
	hotkey := a.settings.Hotkey
	if hotkey == "" {
		hotkey = defaultHotkey
	}
	if err := mygo.GlobalShortcut.Register(hotkey, func() {
		if a.win == nil {
			return
		}
		if a.win.IsVisible() && a.win.IsFocused() {
			a.hide()
			return
		}
		a.show()
	}); err != nil {
		a.api.LogError("launcher", fmt.Sprintf("registering %s failed: %v", hotkey, err))
	}
}

// defaultHotkey is the key that shows the launcher when the user has not
// chosen one.
const defaultHotkey = "Alt+Space"

// onSecondInstance handles a second launch, which carries a query to run.
func (a *app) onSecondInstance(args []string, workingDir string) {
	query, run := parseArgs(args)
	switch {
	case run && query != "":
		a.setQuery(query)
		a.runResult(0, core.DefaultSpecialKeyState)
	case query != "":
		a.setQuery(query)
	default:
		a.show()
	}
}

// runQueryFromCommandLine answers a query and prints the results, and
// optionally runs the first of them.
func (a *app) runQueryFromCommandLine(query string, run bool) error {
	loadPlugins(a)

	results := a.engine.Search(context.Background(), query)
	if len(results) == 0 {
		fmt.Println("no results")
		return nil
	}

	for i, result := range results {
		fmt.Printf("%d. %s | %s | plugin=%s score=%d\n", i+1, result.Title, result.SubTitle, result.PluginID, result.Score)
	}

	if run {
		fmt.Printf("--run: %s | %s\n", results[0].Title, results[0].SubTitle)
		a.runAction(results[0], core.DefaultSpecialKeyState)
	}
	return nil
}

// setOpenAtLogin makes the launcher start when the user logs in, which the
// system's own settings list and can turn off.
func (a *app) setOpenAtLogin(enabled bool) error {
	if err := mygo.App.SetOpenAtLogin(enabled); err != nil {
		a.api.LogError("launcher", "starting at login could not be set: "+err.Error())
		return err
	}
	return nil
}

// capturePath is where --capture asks for a picture of the window.
func capturePath() string {
	for i, argument := range os.Args {
		if argument == "--capture" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// captureTo shows the window, waits for it to be drawn, writes a picture of
// it, and quits. It is how the interface is looked at while it is built,
// since a window drawn on the GPU is not always what a screen capture
// shows.
func (a *app) captureTo(path string) {
	a.show()
	go func() {
		// The window needs a moment to open and draw before it can be
		// asked for a picture.
		time.Sleep(1200 * time.Millisecond)
		picture, err := a.win.CapturePage()
		if err != nil {
			log.Println("capturing the window failed:", err)
			mygo.App.Exit(1)
			return
		}
		if err := os.WriteFile(path, picture, 0o644); err != nil {
			log.Println("writing the picture failed:", err)
			mygo.App.Exit(1)
			return
		}
		fmt.Println("wrote", path, len(picture), "bytes")
		mygo.App.Exit(0)
	}()
}

// parseArgs reads the launcher's command line: a query to type, and whether
// to run what it finds.
func parseArgs(args []string) (query string, run bool) {
	for i := 0; i < len(args); i++ {
		switch strings.ToLower(args[i]) {
		case "--query", "-q":
			if i+1 < len(args) {
				query = args[i+1]
				i++
			}
		case "--run", "-r":
			run = true
		}
	}
	return query, run
}
