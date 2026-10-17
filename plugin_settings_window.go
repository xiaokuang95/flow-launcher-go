package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// pluginSettingsWindow is the window a plugin's own settings are changed
// in, built from the fields the plugin hands over. The reference opens one
// per plugin the same way.
//
// It is drawn as a settings pane: what the plugin is at the top, then its
// settings in one card, a row each, with the label on the left and what
// changes it on the right. A list of bare fields with a checkbox beside
// each is what a generated form looks like, and what this is not.
type pluginSettingsWindow struct {
	app      *app
	win      *mygo.Window
	pluginID string

	// fields are the settings as the window shows them, in the order they
	// are shown.
	fields []core.SettingField
}

// pluginSettingsWindows are the windows that are open, one per plugin.
var pluginSettingsWindows = map[string]*pluginSettingsWindow{}

// The shape of the window, which is what makes it read as a pane rather
// than as a form.
const (
	// settingsCardRadius is how round the card holding the settings is.
	settingsCardRadius float32 = 10
)

// pluginSettingsFor is a plugin's own settings as a form, and whether it
// has any the window can show.
func pluginSettingsFor(a *app, pluginID string) ([]core.SettingField, bool) {
	pair, ok := a.manager.GetPluginForID(pluginID)
	if !ok {
		return nil, false
	}
	provider, ok := pair.Plugin.(core.SettingProviderPlugin)
	if !ok {
		return nil, false
	}

	fields := core.SettingFields(provider.Settings())
	if len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

// openPluginSettingsAfterFrame opens a plugin's settings once the frame
// being drawn is done. A window made while a frame is being built is not
// drawn: mygo builds the frame it is in the middle of, and the window made
// during it is left behind.
func openPluginSettingsAfterFrame(a *app, pluginID string) {
	if openPluginSettingsWindowHook != nil {
		openPluginSettingsWindowHook(a, pluginID)
		return
	}
	go func() {
		// From another goroutine the call waits for the main thread, which
		// is done with the frame by then.
		mygo.RunOnMain(func() { openPluginSettingsWindow(a, pluginID) })
	}()
}

// openPluginSettingsWindow shows a plugin's own settings, making the window
// the first time. It reports whether the plugin has any.
func openPluginSettingsWindow(a *app, pluginID string) bool {
	fields, ok := pluginSettingsFor(a, pluginID)
	if !ok {
		return false
	}
	pair, _ := a.manager.GetPluginForID(pluginID)

	window := pluginSettingsWindows[pluginID]
	if window == nil {
		window = &pluginSettingsWindow{app: a, pluginID: pluginID}
		options := paneOptions(
			strings.Replace(core.T("", "pluginSettingsWindowTitle"), "{0}", pair.Metadata.Name, 1),
			640, 560, "plugin-settings-"+pluginID, window.view)
		options.MinWidth = 480
		options.MinHeight = 320
		window.win = mygo.NewWindow(options)
		pluginSettingsWindows[pluginID] = window
	}

	// The settings are read again, since a plugin may have replaced them.
	window.fields = fields
	window.win.Show()
	window.win.Focus()
	return true
}

// view builds the window: what the plugin is, then its settings in a card.
func (w *pluginSettingsWindow) view(c *ui.Context) {
	w.app.applyTheme(c)
	applyMacLook(c)
	t := c.Theme()

	metadata := core.PluginMetadata{}
	if pair, ok := w.app.manager.GetPluginForID(w.pluginID); ok {
		metadata = pair.Metadata
	}

	root := paneView(c)
	root.Children(func() {
		w.header(c, metadata)

		if len(w.fields) == 0 {
			ui.Text(c, core.T("", "pluginSettingsWindowOpenFailed")).
				TextColor(t.TextMuted).Padding(24, 12)
			return
		}

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(24, 0, 24, 20).Gap(0).Children(func() {
				w.card(c)
			})
		})
	})
}

// header says what the plugin is, with the picture the launcher shows it
// by: a window that says only "settings" does not say whose.
func (w *pluginSettingsWindow) header(c *ui.Context, metadata core.PluginMetadata) {
	t := c.Theme()

	ui.Row(c).Padding(24, 20).Gap(14).AlignItems(ui.Center).Children(func() {
		bitmap := w.app.bitmapFor(w.iconOf(metadata))
		iconFrame(c, bitmap)
		if bitmap == nil {
			ui.Box(c).Size(lookIconSize, lookIconSize)
		}

		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, metadata.Name).FontSize(17).Bold()
			if metadata.Description != "" {
				ui.Text(c, metadata.Description).FontSize(12).TextColor(t.TextMuted).SingleLine()
			}
			ui.Text(c, metadata.Author+" · "+metadata.Version).FontSize(11).TextColor(t.TextMuted)
		})
	})
}

// iconOf is the picture the plugin is drawn with.
func (w *pluginSettingsWindow) iconOf(metadata core.PluginMetadata) string {
	if metadata.IcoPath == "" {
		return ""
	}
	return metadata.IcoPath
}

// card is the settings themselves: one row each, a hairline between them,
// in one rounded panel.
func (w *pluginSettingsWindow) card(c *ui.Context) {
	t := c.Theme()

	panel := ui.Column(c).Radius(settingsCardRadius).Border(1, t.Border.Alpha(0.45)).Background(t.Surface)
	panel.Children(func() {
		for i := range w.fields {
			if i > 0 {
				// A hairline between the rows, which is what says where one
				// setting ends and the next begins.
				ui.Box(c).Height(1).Background(t.Border.Alpha(0.45))
			}
			w.row(c, &w.fields[i])
		}
	})
}

// row is one setting: what it is called on the left, and what changes it on
// the right.
func (w *pluginSettingsWindow) row(c *ui.Context, field *core.SettingField) {
	settingsRow(c, field.Label, func() {
		switch field.Kind {
		case core.SettingSwitch:
			value, _ := field.Value().(bool)
			if ui.Switch(c, &value).Changed() {
				w.change(field, value)
			}
		case core.SettingNumber:
			value, _ := field.Value().(int)
			number := float64(value)
			if ui.NumberInput(c, &number, -100000, 100000, 1).Width(120).Changed() {
				w.change(field, int(number))
			}
		default:
			text, _ := field.Value().(string)
			if ui.TextInput(c, &text).Width(260).Changed() {
				w.change(field, text)
			}
		}
	})
}

// change writes a setting the user changed, and tells the plugin, so that
// what it holds and what is saved are the same.
func (w *pluginSettingsWindow) change(field *core.SettingField, value any) {
	if err := field.Set(value); err != nil {
		w.app.api.LogError("launcher", "changing a plugin setting failed: "+err.Error())
		return
	}

	if pair, ok := w.app.manager.GetPluginForID(w.pluginID); ok {
		if savable, ok := pair.Plugin.(core.SavablePlugin); ok {
			savable.Save()
		}
	}
	w.app.requery()
}

// openPluginSettingsWindowHook, when set, is what the button of a plugin's
// settings calls, so that a test watches the click without a window being
// made.
var openPluginSettingsWindowHook func(a *app, pluginID string) bool
