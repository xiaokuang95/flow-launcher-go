package main

import (
	"bytes"
	"image/png"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// The launcher's tray icon: a magnifier, drawn as an SVG so that it is a
// few lines rather than a picture to ship.
const trayIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16">
  <circle cx="6.5" cy="6.5" r="4.5" fill="none" stroke="currentColor" stroke-width="2"/>
  <line x1="10" y1="10" x2="14.5" y2="14.5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
</svg>`

// searchIcon is the magnifier the query box shows when no single plugin is
// answering.
func searchIcon() *ui.SVG {
	return ui.MustParseSVG([]byte(trayIconSVG))
}

// createTray puts the launcher's icon in the notification area, with a menu
// that opens it and quits it.
func (a *app) createTray() {
	if a.settings.HideNotifyIcon {
		return
	}

	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIconPNG(false),
		IconIsTemplate: true,
		ToolTip:        "Flow Launcher",
		Menu:           a.trayMenu(),
	})
	if err != nil {
		a.api.LogError("launcher", "the tray icon could not be shown: "+err.Error())
		return
	}
	a.tray = tray

	// On Windows and Linux the menu shows on a click; on macOS the icon
	// opens the launcher and the menu on a right click.
	tray.OnClick(func() { a.show() })
}

// trayMenu is what the tray icon offers.
func (a *app) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: core.T("", "iconTraySettings"), Click: func(*mygo.MenuItem, *mygo.Window) { openSettingsWindow(a) }},
		{Label: core.T("", "iconTrayAbout"), Click: func(*mygo.MenuItem, *mygo.Window) { a.notify("Flow Launcher", version) }},
		mygo.Separator(),
		{Label: core.T("", "ToggleGameModeHotkey"), Type: mygo.MenuItemCheckbox, Checked: a.gameMode(), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.setGameMode(!a.gameMode())
		}},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
}

// updateTray redraws the tray icon, which shows the game mode.
func (a *app) updateTray() {
	if a.tray == nil {
		return
	}
	a.tray.SetIcon(trayIconPNG(a.gameMode()), true)
}

// trayIconPNG draws the tray icon: the magnifier, in a lighter color when
// the launcher is holding itself back, so that the two can be told apart.
func trayIconPNG(gameMode bool) []byte {
	color := ui.Hex("#1f2937")
	if gameMode {
		color = ui.Hex("#9ca3af")
	}

	rendered := ui.Render(func(c *ui.Context) {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Icon(c, searchIcon()).Size(24, 24).TextColor(color)
		})
	}, 32, 32, 2)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, rendered); err != nil {
		return nil
	}
	return buffer.Bytes()
}
