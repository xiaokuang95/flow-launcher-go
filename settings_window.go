package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"launch/core"
)

// settingsWindow is the window the launcher's settings are changed in: a
// list of pages on the left and the page itself on the right.
type settingsWindow struct {
	app *app
	win *mygo.Window

	page     int
	pageList ui.ListState

	// draft is the settings being edited, which are written out when the
	// window closes.
	draft core.Settings

	// storeFilter is what the user typed to narrow the plugin store, and
	// storeShown the plugins it lets through, kept between frames.
	storeFilter     string
	storeFilterUsed string
	storeShown      []core.UserPlugin
	storeShownFor   []core.UserPlugin
	storeList       ui.ListState
	storeDrawn      int
}

// settingsPages are the pages the settings window shows, in order.
var settingsPages = []string{
	"general", "plugins", "pluginStore", "theme", "hotkey", "proxy", "about",
}

// openSettingsWindow shows the settings, making the window the first time.
func openSettingsWindow(a *app) {
	if a.settingsWin == nil {
		window := &settingsWindow{app: a}
		window.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     core.T("", "flowlauncher_settings"),
			Width:     900,
			Height:    620,
			MinWidth:  700,
			MinHeight: 480,
			StateKey:  "settings",
			Content:   ui.View(window.view),
		})
		a.settingsWin = window
	}

	a.settingsWin.draft = *a.settings
	a.settingsWin.win.Show()
	a.settingsWin.win.Focus()
}

// view builds the settings window.
func (s *settingsWindow) view(c *ui.Context) {
	// The settings wear the theme the user is choosing, so that a theme is
	// seen as it is picked rather than after the window is closed.
	s.app.applyTheme(c)
	t := c.Theme()

	s.pageList.Selected = &s.page
	s.pageList.Label = func(row int) string { return s.pageTitle(row) }

	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(200).Padding(12).Gap(4).Background(t.Surface).Children(func() {
			ui.Text(c, core.T("", "flowlauncher_settings")).FontSize(18).Bold().Padding(0, 0, 8, 8)
			ui.List(c, &s.pageList, len(settingsPages), func(row int) {
				text := ui.Text(c, s.pageTitle(row)).Padding(8, 10)
				if row == s.page {
					text.Background(t.Selection).Radius(6)
				}
			}).Grow(1)
		})

		ui.Column(c).Grow(1).Padding(20).Gap(12).Children(func() {
			switch settingsPages[s.page] {
			case "general":
				s.generalPage(c)
			case "theme":
				s.themePage(c)
			case "hotkey":
				s.hotkeyPage(c)
			case "proxy":
				s.proxyPage(c)
			case "plugins":
				s.pluginsPage(c)
			case "pluginStore":
				s.pluginStorePage(c)
			case "about":
				s.aboutPage(c)
			}
		})
	})
}

// pageTitle is what a page is called in the user's language.
func (s *settingsWindow) pageTitle(row int) string {
	if row < 0 || row >= len(settingsPages) {
		return ""
	}
	key := map[string]string{
		"general":     "general",
		"plugins":     "plugins",
		"pluginStore": "pluginStore",
		"theme":       "theme",
		"hotkey":      "hotkey",
		"proxy":       "proxy",
		"about":       "about",
	}[settingsPages[row]]
	return core.T("", key)
}

// generalPage is the settings most people change.
func (s *settingsWindow) generalPage(c *ui.Context) {
	draft := &s.draft

	ui.Text(c, core.T("", "general")).FontSize(20).Bold()

	s.field(c, core.T("", "language"), func() {
		names := make([]string, 0, len(core.AvailableLanguages)+1)
		codes := make([]string, 0, len(core.AvailableLanguages)+1)
		names = append(names, core.SystemLanguageName(draft.Language))
		codes = append(codes, core.SystemLanguageCode)
		for _, language := range core.AvailableLanguages {
			names = append(names, language.Display)
			codes = append(codes, language.Code)
		}

		// The drop-down holds names, so the code is found from the name.
		chosen := nameForCode(codes, names, draft.Language)
		if ui.Select(c, &chosen, names).Width(220).Changed() {
			if code := codeForName(names, codes, chosen); code != "" {
				draft.Language = code
				core.SetLanguage(draft.Language)
				s.apply()
			}
		}
	})

	s.field(c, core.T("", "PlaceholderText"), func() {
		if ui.TextInput(c, &draft.PlaceholderText).Width(220).Changed() {
			s.apply()
		}
	})

	s.checkbox(c, core.T("", "HideWhenDeactivated"), &draft.HideWhenDeactivated)
	s.checkbox(c, core.T("", "startFlowLauncherOnSystemStartup"), &draft.StartFlowLauncherOnSystemStartup)
	s.checkbox(c, core.T("", "autoUpdates"), &draft.AutoUpdates)
	s.checkbox(c, core.T("", "AlwaysPreview"), &draft.AlwaysPreview)
	s.checkbox(c, core.T("", "homePage"), &draft.ShowHomePage)
	s.checkbox(c, core.T("", "historyResultsForHomePage"), &draft.ShowHistoryResultsForHomePage)

	s.field(c, core.T("", "maxShowResults"), func() {
		value := float64(draft.MaxResultsToShow)
		if ui.Slider(c, &value, core.MinResultsToShow, core.MaxResultsToShowLimit).Width(220).Changed() {
			draft.MaxResultsToShow = int(value + 0.5)
			s.apply()
		}
	})

	s.field(c, core.T("", "querySearchPrecision"), func() {
		options := []string{"Regular", "Low", "None"}
		chosen := draft.QuerySearchPrecision.String()
		if ui.Select(c, &chosen, options).Width(220).Changed() {
			switch chosen {
			case "Low":
				draft.QuerySearchPrecision = core.SearchPrecisionLow
			case "None":
				draft.QuerySearchPrecision = core.SearchPrecisionNone
			default:
				draft.QuerySearchPrecision = core.SearchPrecisionRegular
			}
			s.apply()
		}
	})

	s.checkbox(c, core.T("", "ignoreAccents"), &draft.IgnoreAccents)

	// A name written in Chinese is found by its pinyin, which is what the
	// reference's own settings offer the same way.
	s.checkbox(c, core.T("", "ShouldUsePinyin"), &draft.ShouldUsePinyin)
	s.checkbox(c, core.T("", "UsePolyphonicPhraseOverrides"), &draft.UsePolyphonicPhraseOverrides)
	s.checkbox(c, core.T("", "ShouldUseDoublePinyin"), &draft.UseDoublePinyin)

	s.field(c, core.T("", "logLevel"), func() {
		levels := []string{core.LogLevelDEBUG, core.LogLevelINFO, core.LogLevelERROR, core.LogLevelNONE}
		names := make([]string, 0, len(levels))
		for _, level := range levels {
			names = append(names, core.T("", "LogLevel"+level))
		}
		chosen := nameForCode(levels, names, s.draft.LogLevel)
		if ui.Select(c, &chosen, names).Width(220).Changed() {
			if level := codeForName(names, levels, chosen); level != "" {
				s.draft.LogLevel = level
				s.apply()
			}
		}
	})

	s.field(c, core.T("", "DoublePinyinSchema"), func() {
		codes := core.DoublePinyinSchemas
		names := make([]string, 0, len(codes))
		for _, code := range codes {
			names = append(names, core.T("", "DoublePinyinSchemas"+code))
		}
		chosen := nameForCode(codes, names, draft.DoublePinyinSchema)
		if ui.Select(c, &chosen, names).Width(220).Changed() {
			if code := codeForName(names, codes, chosen); code != "" {
				draft.DoublePinyinSchema = code
				s.apply()
			}
		}
	})
}

// themePage picks the theme the launcher wears.
func (s *settingsWindow) themePage(c *ui.Context) {
	ui.Text(c, core.T("", "theme")).FontSize(20).Bold()

	themes := core.AvailableThemes()
	names := make([]string, 0, len(themes))
	codes := make([]string, 0, len(themes))
	for _, theme := range themes {
		names = append(names, theme.Name)
		codes = append(codes, theme.FileNameWithoutExtension)
	}

	s.field(c, core.T("", "theme"), func() {
		chosen := nameForCode(codes, names, s.draft.Theme)
		if ui.Select(c, &chosen, names).Width(220).Changed() {
			if code := codeForName(names, codes, chosen); code != "" {
				s.draft.Theme = code
				s.apply()
			}
		}
	})

	s.field(c, core.T("", "windowWidthSize"), func() {
		width := s.draft.WindowSize
		if ui.Slider(c, &width, 320, 1200).Width(220).Changed() {
			s.draft.WindowSize = width
			s.apply()
		}
	})

	s.field(c, core.T("", "resultItemFont"), func() {
		size := float64(s.draft.ResultFontSize)
		if size <= 0 {
			size = 16
		}
		if ui.Slider(c, &size, 10, 28).Width(220).Changed() {
			s.draft.ResultFontSize = int(size + 0.5)
			s.apply()
		}
	})

	s.field(c, core.T("", "queryBoxFont"), func() {
		size := float64(s.draft.QueryFontSize)
		if size <= 0 {
			size = 24
		}
		if ui.Slider(c, &size, 12, 40).Width(220).Changed() {
			s.draft.QueryFontSize = int(size + 0.5)
			s.apply()
		}
	})

	s.field(c, core.T("", "resultSubItemFont"), func() {
		size := float64(s.draft.ResultSubItemFontSize)
		if size <= 0 {
			size = 12
		}
		if ui.Slider(c, &size, 8, 24).Width(220).Changed() {
			s.draft.ResultSubItemFontSize = int(size + 0.5)
			s.apply()
		}
	})

	// The faces, which the reference offers as free text: what the desktop
	// knows by that name is what is used.
	s.field(c, core.T("", "queryBoxFont"), func() {
		if ui.TextInput(c, &s.draft.QueryBoxFont).Width(220).Changed() {
			s.apply()
		}
	})
	s.field(c, core.T("", "resultItemFont"), func() {
		if ui.TextInput(c, &s.draft.ResultFont).Width(220).Changed() {
			s.apply()
		}
	})

	s.checkbox(c, core.T("", "useGlyphUI"), &s.draft.UseGlyphIcons)
	s.checkbox(c, core.T("", "showBadges"), &s.draft.ShowBadges)
	s.checkbox(c, core.T("", "showBadgesGlobalOnly"), &s.draft.ShowBadgesGlobalOnly)
}

// hotkeyPage shows the keys the launcher answers to.
func (s *settingsWindow) hotkeyPage(c *ui.Context) {
	ui.Text(c, core.T("", "hotkey")).FontSize(20).Bold()

	s.field(c, core.T("", "openFlowLauncherHotkey"), func() {
		if ui.TextInput(c, &s.draft.Hotkey).Width(220).Changed() {
			s.apply()
		}
	})

	// The keys the user chose, which the reference's hotkey page offers the
	// same way: each is a setting written as "Ctrl + Tab" or "F1".
	ui.Text(c, core.T("", "hotkeyPresets")).FontSize(16).Bold().Padding(0, 8, 0, 0)
	for _, binding := range changeableHotkeys(&s.draft) {
		s.field(c, core.T("", binding.description), func() {
			if ui.TextInput(c, binding.setting).Width(220).Changed() {
				s.apply()
			}
		})
	}

	// The keys that cannot be changed, shown so that they can be learned.
	ui.Text(c, core.T("", "hotkeyPresets")).FontSize(16).Bold().Padding(0, 8, 0, 0)
	for _, binding := range fixedHotkeys() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Text(c, binding.keys).Width(160).Font("monospace")
			ui.Text(c, core.T("", binding.description)).TextColor(c.Theme().TextMuted)
		})
	}
}

// changeableHotkeys are the keys the settings decide, with the setting each
// is written in.
func changeableHotkeys(settings *core.Settings) []struct {
	setting     *string
	description string
} {
	return []struct {
		setting     *string
		description string
	}{
		{&settings.PreviewHotkey, "previewHotkey"},
		{&settings.AutoCompleteHotkey, "autoCompleteHotkey"},
		{&settings.AutoCompleteHotkey2, "autoCompleteHotkey"},
		{&settings.SelectNextItemHotkey, "SelectNextItemHotkey"},
		{&settings.SelectNextItemHotkey2, "SelectNextItemHotkey"},
		{&settings.SelectPrevItemHotkey, "SelectPrevItemHotkey"},
		{&settings.SelectPrevItemHotkey2, "SelectPrevItemHotkey"},
		{&settings.SelectNextPageHotkey, "SelectNextPageHotkey"},
		{&settings.SelectPrevPageHotkey, "SelectPrevPageHotkey"},
		{&settings.OpenContextMenuHotkey, "OpenContextMenuHotkey"},
		{&settings.SettingWindowHotkey, "SettingWindowHotkey"},
		{&settings.OpenHistoryHotkey, "ToggleHistoryHotkey"},
		{&settings.CycleHistoryUpHotkey, "CycleHistoryUpHotkey"},
		{&settings.CycleHistoryDownHotkey, "CycleHistoryDownHotkey"},
		{&settings.OpenResultModifiers, "openResultModifiers"},
	}
}

// proxyPage is the proxy the launcher and its plugins download through.
func (s *settingsWindow) proxyPage(c *ui.Context) {
	ui.Text(c, core.T("", "proxy")).FontSize(20).Bold()

	s.checkbox(c, core.T("", "enableProxy"), &s.draft.Proxy.Enabled)
	s.field(c, core.T("", "server"), func() {
		if ui.TextInput(c, &s.draft.Proxy.Server).Width(220).Changed() {
			s.apply()
		}
	})
	s.field(c, core.T("", "port"), func() {
		port := float64(s.draft.Proxy.Port)
		if ui.NumberInput(c, &port, 0, 65535, 1).Width(120).Changed() {
			s.draft.Proxy.Port = int(port)
			s.apply()
		}
	})
	s.field(c, core.T("", "userName"), func() {
		if ui.TextInput(c, &s.draft.Proxy.UserName).Width(220).Changed() {
			s.apply()
		}
	})
	s.field(c, core.T("", "password"), func() {
		if ui.TextInput(c, &s.draft.Proxy.Password).Password().Width(220).Changed() {
			s.apply()
		}
	})
}

// pluginsPage lists the launcher's plugins, with a switch for each.
func (s *settingsWindow) pluginsPage(c *ui.Context) {
	ui.Text(c, core.T("", "plugins")).FontSize(20).Bold()

	pairs := s.app.manager.AllLoadedPlugins()
	if len(pairs) == 0 {
		ui.Text(c, core.T("", "searchplugin_Noresult_Title")).TextColor(c.Theme().TextMuted)
		return
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			for _, pair := range pairs {
				id := pair.Metadata.ID
				s.pluginRow(c, id, pair.Metadata)
			}
		})
	})
}

// pluginRow is one plugin in the list, with what it does and a switch.
func (s *settingsWindow) pluginRow(c *ui.Context, id string, metadata core.PluginMetadata) {
	stored, ok := s.app.plugins.GetPluginSettings(id)
	if !ok {
		stored = core.PluginSettings{ID: id, Name: metadata.Name}
	}
	enabled := !stored.Disabled
	homeEnabled := !stored.HomeDisabled

	ui.Column(c).Padding(10).Gap(4).Radius(8).Border(1, c.Theme().Border).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, metadata.Name).Bold().Grow(1)
			ui.Switch(c, &enabled)
		})
		ui.Text(c, metadata.Description).FontSize(12).TextColor(c.Theme().TextMuted)

		if metadata.ActionKeyword != "" {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, core.T("", "actionKeywords")).FontSize(12).Width(120)
				keyword := metadata.ActionKeyword
				if ui.TextInput(c, &keyword).Width(120).Changed() {
					stored.ActionKeywords = []string{keyword}
				}
			})
		}

		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, core.T("", "homePage")).FontSize(12).Width(120)
			ui.Switch(c, &homeEnabled)
		})

		// A plugin with settings of its own is offered a way into them, as
		// the reference's plugin list is.
		if _, ok := pluginSettingsFor(s.app, id); ok {
			label := strings.Replace(core.T("", "pluginSettingsWindowTitle"), "{0}", metadata.Name, 1)
			if ui.Button(c, label).Clicked() {
				openPluginSettingsWindow(s.app, id)
			}
		}
	})

	if enabled != !stored.Disabled || homeEnabled != !stored.HomeDisabled {
		stored.Disabled = !enabled
		stored.HomeDisabled = !homeEnabled
		s.app.plugins.SetPluginSettings(id, stored)
		s.app.manager.UpdateMetadata(id, func(metadata *core.PluginMetadata) {
			metadata.Disabled = stored.Disabled
			metadata.HomeDisabled = stored.HomeDisabled
			if len(stored.ActionKeywords) > 0 {
				metadata.ActionKeywords = stored.ActionKeywords
				metadata.ActionKeyword = stored.ActionKeywords[0]
			}
		})
		_ = s.app.plugins.Save()
	}
}

// pluginStorePage is the list of plugins the launcher can install. Only the
// rows on the screen are drawn, since the store holds hundreds of plugins
// and every row built is work done on every frame.
func (s *settingsWindow) pluginStorePage(c *ui.Context) {
	s.storeDrawn = 0
	ui.Text(c, core.T("", "pluginStore")).FontSize(20).Bold()

	manifest := s.app.pluginManifest()
	if len(manifest) == 0 {
		ui.Text(c, core.T("", "pluginStore_None")).TextColor(c.Theme().TextMuted)
		if ui.Button(c, core.T("", "Refresh")).Clicked() {
			s.app.refreshPluginManifest()
		}
		return
	}

	// What is typed narrows the list, as it does in the reference.
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, core.T("", "searchplugin")).Width(200)
		if ui.TextInput(c, &s.storeFilter).Width(240).Changed() {
			s.storeList.Selected = nil
			s.storeShown = nil
		}
	})

	shown := s.storeRows(manifest)
	if len(shown) == 0 {
		ui.Text(c, core.T("", "searchplugin_Noresult_Title")).TextColor(c.Theme().TextMuted)
		return
	}

	s.storeList.Selected = nil
	s.storeList.Label = func(row int) string {
		if row < len(shown) {
			return shown[row].Name
		}
		return ""
	}

	// The list is given the room that is left in the window: it works out
	// how many rows that is and builds only those, which is what keeps a
	// store of hundreds of plugins from being drawn on every frame.
	ui.Column(c).Grow(1).Children(func() {
		ui.List(c, &s.storeList, len(shown)).Grow(1).Rows(func(row ui.ListRow) {
			if row.Index < 0 || row.Index >= len(shown) {
				return
			}
			s.storeRow(row, shown[row.Index])
		})
	})
}

// storeRowsDrawn is how many rows the store page drew in the frame it drew
// last, which a test reads to see that only what is on the screen is built.
func (s *settingsWindow) storeRowsDrawn() int { return s.storeDrawn }

// storeRow is one plugin of the store.
func (s *settingsWindow) storeRow(row ui.ListRow, plugin core.UserPlugin) {
	s.storeDrawn++
	c := row.Context
	ui.Column(c).Padding(8).Gap(2).Children(func() {
		ui.Text(c, plugin.Name).Bold()
		if plugin.Description != "" {
			ui.Text(c, plugin.Description).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()
		}
	})
}

// storeRows are the plugins of the store the filter lets through, which is
// worked out again only when the list or the filter changed: a search over
// hundreds of names on every frame is work the window does not need.
func (s *settingsWindow) storeRows(manifest []core.UserPlugin) []core.UserPlugin {
	if s.storeShown != nil && s.storeFilter == s.storeFilterUsed && len(s.storeShownFor) == len(manifest) {
		return s.storeShown
	}

	filter := strings.ToLower(strings.TrimSpace(s.storeFilter))
	shown := make([]core.UserPlugin, 0, len(manifest))
	for _, plugin := range manifest {
		if filter != "" &&
			!strings.Contains(strings.ToLower(plugin.Name), filter) &&
			!strings.Contains(strings.ToLower(plugin.Author), filter) &&
			!strings.Contains(strings.ToLower(plugin.Description), filter) {
			continue
		}
		shown = append(shown, plugin)
	}

	s.storeShown = shown
	s.storeFilterUsed = s.storeFilter
	s.storeShownFor = manifest
	return shown
}

// aboutPage says what this is.
func (s *settingsWindow) aboutPage(c *ui.Context) {
	ui.Text(c, core.T("", "about")).FontSize(20).Bold()
	ui.Text(c, "Flow Launcher "+version).Bold()
	ui.Text(c, core.T("", "aboutDescription")).TextColor(c.Theme().TextMuted)
	ui.Textf(c, "%s: %s", core.T("", "plugins"), fmt.Sprint(len(s.app.manager.AllLoadedPlugins())))
	ui.Textf(c, "%s: %s", core.T("", "pluginDirectory"), s.app.dataDir).Selectable().FontSize(12)
	ui.Text(c, core.Tf("", "about_activate_times", fmt.Sprint(s.app.settings.ActivateTimes))).FontSize(12)
}

// checkbox is a setting that is on or off.
func (s *settingsWindow) checkbox(c *ui.Context, label string, value *bool) {
	if ui.Checkbox(c, value, label).Changed() {
		s.apply()
	}
}

// field is a setting with its label beside it.
func (s *settingsWindow) field(c *ui.Context, label string, build func()) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).Width(200)
		build()
	})
}

// apply writes the draft settings out and tells the launcher about them.
func (s *settingsWindow) apply() {
	*s.app.settings = s.draft
	s.app.saveSettings()
	s.app.applySettings()
}

// nameForCode is the name shown for a code, and codeForName the code
// behind a name, which is how a drop-down of names chooses a value.
func nameForCode(codes, names []string, code string) string {
	for i, candidate := range codes {
		if candidate == code && i < len(names) {
			return names[i]
		}
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func codeForName(names, codes []string, name string) string {
	for i, candidate := range names {
		if candidate == name && i < len(codes) {
			return codes[i]
		}
	}
	return ""
}

// fixedHotkeys are the keys the launcher answers to while it is up, which
// the reference shows in its settings but does not let the user change.
func fixedHotkeys() []struct{ keys, description string } {
	return []struct{ keys, description string }{
		{"Up, Down", "HotkeyLeftRightDesc"},
		{"Escape", "HotkeyESCDesc"},
		{"F5", "ReloadPluginHotkey"},
		{"Ctrl+R", "HotkeyRequery"},
		{"Enter", "HotkeyRunDesc"},
		{"Ctrl+Enter", "OpenContainFolderHotkey"},
		{"Shift+Enter", "OpenContextMenuHotkey"},
		{"Alt+Enter", "HotkeyOpenResult"},
		{"Ctrl+F12", "ToggleGameModeHotkey"},
		{"Ctrl+=, Ctrl+-", "QuickHeightHotkey"},
		{"Ctrl+], Ctrl+[", "QuickWidthHotkey"},
	}
}
