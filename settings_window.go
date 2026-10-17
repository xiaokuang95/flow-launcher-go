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
		options := paneOptions(core.T("", "flowlauncher_settings"), 900, 700, "settings", window.view)
		options.MinWidth = 700
		options.MinHeight = 480
		window.win = mygo.NewWindow(options)
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
	s.app.systemIsDark = c.Theme().Dark
	s.app.applyTheme(c)
	applyMacLook(c)
	t := c.Theme()

	s.pageList.Selected = &s.page
	s.pageList.Label = func(row int) string { return s.pageTitle(row) }

	root := paneView(c)
	root.Children(func() {
		ui.Row(c).Fill().Padding(windowPadding).Gap(12).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Width(188).Padding(10).Gap(2).Radius(cardRadius).Background(t.Surface).Children(func() {
				ui.Text(c, core.T("", "flowlauncher_settings")).
					FontSize(14).Bold().Padding(8, 4, 8, 10)
				ui.List(c, &s.pageList, len(settingsPages), func(row int) {
					text := ui.Text(c, s.pageTitle(row)).Padding(10, 8).FillWidth()
					if row == s.page {
						text.Background(selectionTint(t)).Radius(rowRadius).TextColor(t.Text)
					}
				}).Grow(1)
			})

			// A page taller than the window scrolls, so that every setting
			// on it can be reached. The drop-downs are drawn beside the
			// window and take the wheel themselves, so they are not held up
			// by it.
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(10).Children(func() {
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

	sectionTitle(c, core.T("", "general"))

	card(c).Children(func() {
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
			dropDown(c, "language", &chosen, names, func() {
				if code := codeForName(names, codes, chosen); code != "" {
					draft.Language = code
					core.SetLanguage(draft.Language)
					s.apply()
				}
			})
		})

		s.field(c, core.T("", "PlaceholderText"), func() {
			if ui.TextInput(c, &draft.PlaceholderText).Width(220).Changed() {
				s.apply()
			}
		})

		s.checkbox(c, core.T("", "hideFlowLauncherWhenLoseFocus"), &draft.HideWhenDeactivated)
		s.checkbox(c, core.T("", "startFlowLauncherOnSystemStartup"), &draft.StartFlowLauncherOnSystemStartup)
		s.checkbox(c, core.T("", "autoUpdates"), &draft.AutoUpdates)
		s.checkbox(c, core.T("", "AlwaysPreview"), &draft.AlwaysPreview)
		s.checkbox(c, core.T("", "homePage"), &draft.ShowHomePage)
		s.checkbox(c, core.T("", "historyResultsForHomePage"), &draft.ShowHistoryResultsForHomePage)

		s.field(c, core.T("", "maxShowResults"), func() {
			value := float64(draft.MaxResultsToShow)
			if s.slider(c, &value, core.MinResultsToShow, core.MaxResultsToShowLimit) {
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
			dropDown(c, "loglevel", &chosen, names, func() {
				if level := codeForName(names, levels, chosen); level != "" {
					s.draft.LogLevel = level
					s.apply()
				}
			})
		})

		s.field(c, core.T("", "DoublePinyinSchema"), func() {
			codes := core.DoublePinyinSchemas
			names := make([]string, 0, len(codes))
			for _, code := range codes {
				names = append(names, core.T("", "DoublePinyinSchemas"+code))
			}
			chosen := nameForCode(codes, names, draft.DoublePinyinSchema)
			dropDown(c, "pinyin", &chosen, names, func() {
				if code := codeForName(names, codes, chosen); code != "" {
					draft.DoublePinyinSchema = code
					s.apply()
				}
			})
		})
	})
}

// themePage picks the theme the launcher wears.
func (s *settingsWindow) themePage(c *ui.Context) {
	sectionTitle(c, core.T("", "theme"))

	themes := core.AvailableThemes()
	names := make([]string, 0, len(themes))
	codes := make([]string, 0, len(themes))
	for _, theme := range themes {
		names = append(names, theme.Name)
		codes = append(codes, theme.FileNameWithoutExtension)
	}

	card(c).Children(func() {
		s.field(c, core.T("", "theme"), func() {
			chosen := nameForCode(codes, names, s.draft.Theme)
			dropDown(c, "theme", &chosen, names, func() {
				if code := codeForName(names, codes, chosen); code != "" {
					s.draft.Theme = code
					s.apply()
				}
			})
		})

		s.field(c, core.T("", "windowWidthSize"), func() {
			width := s.draft.WindowSize
			if s.slider(c, &width, 320, 1200) {
				s.draft.WindowSize = width
				s.apply()
			}
		})

		s.field(c, core.T("", "resultItemFont"), func() {
			size := float64(s.draft.ResultFontSize)
			if size <= 0 {
				size = 16
			}
			if s.slider(c, &size, 10, 28) {
				s.draft.ResultFontSize = int(size + 0.5)
				s.apply()
			}
		})

		s.field(c, core.T("", "queryBoxFont"), func() {
			size := float64(s.draft.QueryFontSize)
			if size <= 0 {
				size = 24
			}
			if s.slider(c, &size, 12, 40) {
				s.draft.QueryFontSize = int(size + 0.5)
				s.apply()
			}
		})

		s.field(c, core.T("", "resultSubItemFont"), func() {
			size := float64(s.draft.ResultSubItemFontSize)
			if size <= 0 {
				size = 12
			}
			if s.slider(c, &size, 8, 24) {
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
	})
}

// hotkeyPage shows the keys the launcher answers to.
func (s *settingsWindow) hotkeyPage(c *ui.Context) {
	t := c.Theme()

	sectionTitle(c, core.T("", "hotkey"))
	card(c).Children(func() {
		s.field(c, core.T("", "openFlowLauncherHotkey"), func() {
			if ui.TextInput(c, &s.draft.Hotkey).Width(220).Changed() {
				s.apply()
			}
		})
	})

	// The keys the user chose, which the reference's hotkey page offers the
	// same way: each is a setting written as "Ctrl + Tab" or "F1".
	sectionTitle(c, core.T("", "hotkeyPresets"))
	card(c).Children(func() {
		for _, binding := range changeableHotkeys(&s.draft) {
			s.field(c, core.T("", binding.description), func() {
				if ui.TextInput(c, binding.setting).Width(220).Changed() {
					s.apply()
				}
			})
		}
	})

	// The keys that cannot be changed, shown so that they can be learned.
	sectionTitle(c, core.T("", "hotkeyPresets"))
	card(c).Children(func() {
		for _, binding := range fixedHotkeys() {
			ui.Row(c).Height(settingsRowHeight).Padding(14, 0).Gap(16).AlignItems(ui.Center).Children(func() {
				ui.Text(c, binding.keys).Width(160).Font("monospace")
				ui.Text(c, core.T("", binding.description)).TextColor(t.TextMuted)
			})
		}
	})
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
	sectionTitle(c, core.T("", "proxy"))

	card(c).Children(func() {
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
	})
}

// pluginsPage lists the launcher's plugins, with a switch for each.
func (s *settingsWindow) pluginsPage(c *ui.Context) {
	sectionTitle(c, core.T("", "plugins"))

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

	ui.Column(c).Padding(cardPadding).Gap(6).Radius(cardRadius).
		Background(c.Theme().Surface).
		Border(1, c.Theme().Border.Alpha(0.45)).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, metadata.Name).Bold().Grow(1)
			ui.Switch(c, &enabled)
		})
		ui.Text(c, metadata.Description).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()

		// What the plugin answers to, and whether it is on the home page:
		// the two things the reference's list of plugins offers for each.
		if metadata.ActionKeyword != "" {
			ui.Row(c).Height(32).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, core.T("", "actionKeywords")).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
				keyword := metadata.ActionKeyword
				if ui.TextInput(c, &keyword).Width(120).Changed() {
					stored.ActionKeywords = []string{keyword}
				}
			})
		}

		ui.Row(c).Height(32).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, core.T("", "homePage")).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
			ui.Switch(c, &homeEnabled)
		})

		// A plugin with settings of its own is offered a way into them, as
		// the reference's plugin list is.
		if _, ok := pluginSettingsFor(s.app, id); ok {
			label := strings.Replace(core.T("", "pluginSettingsWindowTitle"), "{0}", metadata.Name, 1)
			ui.Row(c).Children(func() {
				if ui.Button(c, label).Clicked() {
					// The window is made after this frame: a window created
					// while one is being drawn is not drawn itself, which is
					// what made the button do nothing.
					openPluginSettingsAfterFrame(s.app, id)
				}
				ui.Box(c).Grow(1)
			})
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
	sectionTitle(c, core.T("", "pluginStore"))

	manifest := s.app.pluginManifest()
	if len(manifest) == 0 {
		ui.Text(c, core.T("", "pluginStore_None")).TextColor(c.Theme().TextMuted)
		if ui.Button(c, core.T("", "Refresh")).Clicked() {
			s.app.refreshPluginManifest()
		}
		return
	}

	card(c).Children(func() {
		// What is typed narrows the list, as it does in the reference.
		ui.Row(c).Height(settingsRowHeight).Padding(14, 0).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Text(c, core.T("", "searchplugin")).Width(settingsLabelWidth)
			if ui.TextInput(c, &s.storeFilter).Width(240).Changed() {
				s.storeList.Selected = nil
				s.storeShown = nil
			}
		})

		shown := s.storeRows(manifest)
		if len(shown) == 0 {
			ui.Text(c, core.T("", "searchplugin_Noresult_Title")).TextColor(c.Theme().TextMuted).Padding(14, 12)
			return
		}

		s.storeList.Selected = nil
		s.storeList.Label = func(row int) string {
			if row < len(shown) {
				return shown[row].Name
			}
			return ""
		}

		// The list is given a height of its own, so that it works out how
		// many rows fit and builds only those: a store of hundreds of
		// plugins drawn on every frame is what made this page slow.
		ui.List(c, &s.storeList, len(shown)).Height(storeListHeight).Rows(func(row ui.ListRow) {
			if row.Index < 0 || row.Index >= len(shown) {
				return
			}
			s.storeRow(row, shown[row.Index])
		})
	})
}

// storeListHeight is how tall the list of the store's plugins is, after
// which it scrolls.
const storeListHeight = 320

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
	t := c.Theme()

	sectionTitle(c, core.T("", "about"))
	card(c).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Flow Launcher "+version).Bold()
			ui.Text(c, core.T("", "aboutDescription")).TextColor(t.TextMuted)
			ui.Textf(c, "%s: %s", core.T("", "plugins"), fmt.Sprint(len(s.app.manager.AllLoadedPlugins())))
			ui.Textf(c, "%s: %s", core.T("", "pluginDirectory"), s.app.dataDir).Selectable().FontSize(12)
			ui.Text(c, core.Tf("", "about_activate_times", fmt.Sprint(s.app.settings.ActivateTimes))).FontSize(12)
		})
	})
}

// checkbox is a setting that is on or off.
func (s *settingsWindow) checkbox(c *ui.Context, label string, value *bool) {
	settingsRow(c, label, func() {
		if ui.Switch(c, value).Changed() {
			s.apply()
		}
	})
}

// field is a setting with its label beside it.
func (s *settingsWindow) field(c *ui.Context, label string, build func()) {
	settingsRow(c, label, build)
}

// slider is a number the user sets by dragging, with the number it is on
// shown beside it: a track alone does not say what is chosen.
func (s *settingsWindow) slider(c *ui.Context, value *float64, lo, hi float64) bool {
	changed := ui.Slider(c, value, lo, hi).Width(180).Changed()
	ui.Text(c, itoa(int(*value+0.5))).Width(36).TextColor(c.Theme().TextMuted)
	return changed
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
