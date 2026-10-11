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
type pluginSettingsWindow struct {
	app      *app
	win      *mygo.Window
	pluginID string

	// fields are the settings as the window shows them, in the order they
	// are shown.
	fields []core.SettingField
	// group is the group of settings being shown, which the window walks
	// through one at a time.
	group int
}

// pluginSettingsWindows are the windows that are open, one per plugin.
var pluginSettingsWindows = map[string]*pluginSettingsWindow{}

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
		window.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     strings.Replace(core.T("", "pluginSettingsWindowTitle"), "{0}", pair.Metadata.Name, 1),
			Width:     640,
			Height:    520,
			MinWidth:  480,
			MinHeight: 320,
			StateKey:  "plugin-settings-" + pluginID,
			Content:   ui.View(window.view),
		})
		pluginSettingsWindows[pluginID] = window
	}

	// The settings are read again, since a plugin may have replaced them.
	window.fields = fields
	window.group = 0
	window.win.Show()
	window.win.Focus()
	return true
}

// view builds the window: what the plugin is, and its settings.
func (w *pluginSettingsWindow) view(c *ui.Context) {
	t := c.Theme()
	metadata := core.PluginMetadata{}
	if pair, ok := w.app.manager.GetPluginForID(w.pluginID); ok {
		metadata = pair.Metadata
	}

	ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
		ui.Text(c, metadata.Name).FontSize(18).Bold()
		if metadata.Description != "" {
			ui.Text(c, metadata.Description).FontSize(12).TextColor(t.TextMuted)
		}
		ui.Text(c, metadata.Author+" · "+metadata.Version).FontSize(12).TextColor(t.TextMuted)

		if len(w.fields) == 0 {
			ui.Text(c, core.T("", "pluginSettingsWindowOpenFailed")).TextColor(t.TextMuted)
			return
		}

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				for i := range w.fields {
					w.field(c, &w.fields[i])
				}
			})
		})

	})
}

// field is one setting of the plugin, drawn as what it holds.
func (w *pluginSettingsWindow) field(c *ui.Context, field *core.SettingField) {
	switch field.Kind {
	case core.SettingSwitch:
		value, _ := field.Value().(bool)
		if ui.Checkbox(c, &value, field.Label).Changed() {
			w.change(field, value)
		}
	case core.SettingNumber:
		value, _ := field.Value().(int)
		number := float64(value)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, field.Label).Width(220)
			if ui.NumberInput(c, &number, -100000, 100000, 1).Width(140).Changed() {
				w.change(field, int(number))
			}
		})
	default:
		text, _ := field.Value().(string)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, field.Label).Width(220)
			if ui.TextInput(c, &text).Width(320).Changed() {
				w.change(field, text)
			}
		})
	}
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
