package core

import "path/filepath"

// LastQueryMode is what the query box holds when the launcher opens.
type LastQueryMode string

// What the query box can hold when the launcher opens.
const (
	// LastQuerySelected restores the whole query with its text chosen.
	LastQuerySelected LastQueryMode = "Selected"
	// LastQueryEmpty opens with an empty query box.
	LastQueryEmpty LastQueryMode = "Empty"
	// LastQueryPreserved restores the whole query, with the caret at its end.
	LastQueryPreserved LastQueryMode = "Preserved"
	// LastQueryActionKeywordPreserved restores only the action keyword.
	LastQueryActionKeywordPreserved LastQueryMode = "ActionKeywordPreserved"
	// LastQueryActionKeywordSelected restores only the action keyword, and
	// chooses it.
	LastQueryActionKeywordSelected LastQueryMode = "ActionKeywordSelected"
)

// HistoryStyle is what the home page lists.
type HistoryStyle string

// What the home page can list.
const (
	// HistoryStyleQuery lists the queries the user ran.
	HistoryStyleQuery HistoryStyle = "Query"
	// HistoryStyleLastOpened lists the results they opened.
	HistoryStyleLastOpened HistoryStyle = "LastOpened"
)

// SearchWindowScreens is the screen the launcher opens on.
type SearchWindowScreens string

// The screens the launcher can open on.
const (
	SearchWindowScreenCursor  SearchWindowScreens = "Cursor"
	SearchWindowScreenPrimary SearchWindowScreens = "Primary"
	SearchWindowScreenCustom  SearchWindowScreens = "Custom"
)

// SearchWindowAligns is where on that screen it opens.
type SearchWindowAligns string

// Where the launcher can open on its screen.
const (
	SearchWindowAlignCenter SearchWindowAligns = "Center"
	SearchWindowAlignTop    SearchWindowAligns = "Top"
	SearchWindowAlignBottom SearchWindowAligns = "Bottom"
)

// BackdropTypes is what the window shows behind the launcher.
type BackdropTypes string

// What the window can show behind the launcher.
const (
	BackdropNone    BackdropTypes = "None"
	BackdropMica    BackdropTypes = "Mica"
	BackdropAcrylic BackdropTypes = "Acrylic"
)

// HttpProxy is the proxy the launcher and its plugins download through.
type HttpProxy struct {
	Enabled  bool   `json:"Enabled"`
	Server   string `json:"Server"`
	Port     int    `json:"Port"`
	UserName string `json:"UserName"`
	Password string `json:"Password"`
}

// CustomShortcut expands a word the user types into something longer.
type CustomShortcut struct {
	Key      string `json:"Key"`
	Expanded string `json:"Expanded"`
}

// Settings is the launcher's own settings, as Settings.json holds them.
type Settings struct {
	// --- Startup ---
	StartFlowLauncherOnSystemStartup bool `json:"StartFlowLauncherOnSystemStartup"`
	UseLogonTaskForStartup           bool `json:"UseLogonTaskForStartup"`
	HideOnStartup                    bool `json:"HideOnStartup"`
	HideNotifyIcon                   bool `json:"HideNotifyIcon"`

	// --- General ---
	Language            string `json:"Language"`
	AlwaysStartEn       bool   `json:"AlwaysStartEn"`
	StartWithSystem     bool   `json:"StartWithSystem"`
	AutoUpdates         bool   `json:"AutoUpdates"`
	AutoUpdatePlugins   bool   `json:"AutoUpdatePlugins"`
	DontPromptUpdateMsg bool   `json:"DontPromptUpdateMsg"`
	EnableUpdateLog     bool   `json:"EnableUpdateLog"`
	ReleaseNotesVersion string `json:"ReleaseNotesVersion"`

	// --- The window ---
	WindowSize                float64             `json:"WindowSize"`
	WindowHeightSize          float64             `json:"WindowHeightSize"`
	ItemHeightSize            float64             `json:"ItemHeightSize"`
	MaxResultsToShow          int                 `json:"MaxResultsToShow"`
	KeepMaxResults            bool                `json:"KeepMaxResults"`
	HideWhenDeactivated       bool                `json:"HideWhenDeactivated"`
	ShowTaskbarWhenInvoked    bool                `json:"ShowTaskbarWhenInvoked"`
	ShowAtTopmost             bool                `json:"ShowAtTopmost"`
	SearchWindowScreen        SearchWindowScreens `json:"SearchWindowScreen"`
	SearchWindowAlign         SearchWindowAligns  `json:"SearchWindowAlign"`
	CustomScreenNumber        int                 `json:"CustomScreenNumber"`
	IgnoreHotkeysOnFullscreen bool                `json:"IgnoreHotkeysOnFullscreen"`
	LastQueryMode             LastQueryMode       `json:"LastQueryMode"`
	HistoryStyle              HistoryStyle        `json:"HistoryStyle"`

	// --- Appearance ---
	Theme               string        `json:"Theme"`
	UseDropShadowEffect bool          `json:"UseDropShadowEffect"`
	BackdropType        BackdropTypes `json:"BackdropType"`
	ColorScheme         string        `json:"ColorScheme"`
	ResultFontSize      int           `json:"ResultFontSize"`
	QueryFontSize       int           `json:"QueryFontSize"`
	CustomTheme         bool          `json:"CustomTheme"`

	// --- Searching ---
	QuerySearchPrecision               SearchPrecisionScore `json:"QuerySearchPrecision"`
	IgnoreAccents                      bool                 `json:"IgnoreAccents"`
	SearchQueryResultsWithDelay        bool                 `json:"SearchQueryResultsWithDelay"`
	SearchDelayTime                    int                  `json:"SearchDelayTime"`
	ShowHomePage                       bool                 `json:"ShowHomePage"`
	ShowHistoryResultsForHomePage      bool                 `json:"ShowHistoryResultsForHomePage"`
	MaxHistoryResultsToShowForHomePage int                  `json:"MaxHistoryResultsToShowForHomePage"`
	ShowUnknownSourceWarning           bool                 `json:"ShowUnknownSourceWarning"`

	// --- The query box ---
	PlaceholderText          string `json:"PlaceholderText"`
	AutoRestartAfterChanging bool   `json:"AutoRestartAfterChanging"`

	// --- Preview ---
	AlwaysPreview      bool   `json:"AlwaysPreview"`
	PreviewHotkey      string `json:"PreviewHotkey"`
	CodeHighlightTheme string `json:"CodeHighlightTheme"`

	// --- Hotkeys ---
	// Hotkey shows the launcher from anywhere.
	Hotkey                 string `json:"Hotkey"`
	OpenResultModifiers    string `json:"OpenResultModifiers"`
	AutoCompleteHotkey     string `json:"AutoCompleteHotkey"`
	AutoCompleteHotkey2    string `json:"AutoCompleteHotkey2"`
	SelectNextItemHotkey   string `json:"SelectNextItemHotkey"`
	SelectNextItemHotkey2  string `json:"SelectNextItemHotkey2"`
	SelectPrevItemHotkey   string `json:"SelectPrevItemHotkey"`
	SelectPrevItemHotkey2  string `json:"SelectPrevItemHotkey2"`
	SelectNextPageHotkey   string `json:"SelectNextPageHotkey"`
	SelectPrevPageHotkey   string `json:"SelectPrevPageHotkey"`
	OpenContextMenuHotkey  string `json:"OpenContextMenuHotkey"`
	SettingWindowHotkey    string `json:"SettingWindowHotkey"`
	OpenHistoryHotkey      string `json:"OpenHistoryHotkey"`
	CycleHistoryUpHotkey   string `json:"CycleHistoryUpHotkey"`
	CycleHistoryDownHotkey string `json:"CycleHistoryDownHotkey"`
	// DialogJumpHotkey opens the jump to a folder that is already open in
	// the file manager. The launcher does not do this yet.
	DialogJumpHotkey    string               `json:"DialogJumpHotkey"`
	CustomPluginHotkeys []CustomPluginHotkey `json:"CustomPluginHotkeys"`
	CustomShortcuts     []CustomShortcut     `json:"CustomShortcuts"`

	// --- Sound ---
	SoundEnabled bool `json:"SoundEnabled"`
	SoundVolume  int  `json:"SoundVolume"`

	// --- Clock and date, shown when the query box is empty ---
	UseClock    bool   `json:"UseClock"`
	ClockFormat string `json:"ClockFormat"`
	UseDate     bool   `json:"UseDate"`
	DateFormat  string `json:"DateFormat"`

	// --- Proxy ---
	Proxy HttpProxy `json:"Proxy"`

	// --- Game mode ---
	GameMode bool `json:"GameMode"`

	// --- Files ---
	CustomExplorerIndex    int    `json:"CustomExplorerIndex"`
	CustomBrowserIndex     int    `json:"CustomBrowserIndex"`
	LastCustomExplorerPath string `json:"LastCustomExplorerPath"`

	// --- Plugin store ---
	PluginsModified map[string]bool `json:"PluginsModified"`

	path string `json:"-"`
}

// CustomPluginHotkey runs a plugin's query when a global hotkey is pressed.
type CustomPluginHotkey struct {
	Hotkey        string `json:"Hotkey"`
	ActionKeyword string `json:"ActionKeyword"`
}

// DefaultSettings are the settings of a launcher the user has not changed.
func DefaultSettings() *Settings {
	return &Settings{
		Language:                           SystemLanguageCode,
		AutoUpdates:                        true,
		AutoUpdatePlugins:                  true,
		WindowSize:                         580,
		WindowHeightSize:                   42,
		ItemHeightSize:                     58,
		MaxResultsToShow:                   5,
		HideWhenDeactivated:                true,
		ShowAtTopmost:                      false,
		SearchWindowScreen:                 SearchWindowScreenCursor,
		SearchWindowAlign:                  SearchWindowAlignCenter,
		CustomScreenNumber:                 1,
		LastQueryMode:                      LastQuerySelected,
		HistoryStyle:                       HistoryStyleQuery,
		Theme:                              DefaultThemeName,
		UseDropShadowEffect:                true,
		BackdropType:                       BackdropNone,
		ColorScheme:                        "System",
		QuerySearchPrecision:               SearchPrecisionRegular,
		SearchDelayTime:                    150,
		ShowHomePage:                       true,
		MaxHistoryResultsToShowForHomePage: 5,
		ShowUnknownSourceWarning:           true,
		PreviewHotkey:                      "F1",
		OpenResultModifiers:                "Alt",
		AutoCompleteHotkey:                 "Ctrl + Tab",
		SelectNextItemHotkey:               "Tab",
		SelectPrevItemHotkey:               "Shift + Tab",
		SelectNextPageHotkey:               "PageUp",
		SelectPrevPageHotkey:               "PageDown",
		OpenContextMenuHotkey:              "Ctrl+O",
		SettingWindowHotkey:                "Ctrl+I",
		OpenHistoryHotkey:                  "Ctrl+H",
		CycleHistoryUpHotkey:               "Alt + Up",
		CycleHistoryDownHotkey:             "Alt + Down",
		DialogJumpHotkey:                   "Alt + G",
		SoundVolume:                        100,
		UseClock:                           true,
		ClockFormat:                        "HH:mm",
		UseDate:                            false,
		DateFormat:                         "yyyy-MM-dd",
		CodeHighlightTheme:                 "Auto",
	}
}

// DefaultThemeName is the theme a launcher the user has not changed wears.
const DefaultThemeName = "System"

// LoadSettings reads the settings, or makes the defaults.
func LoadSettings(path string) *Settings {
	settings := DefaultSettings()

	storage := JSONStorage[*Settings]{Path: path}
	if stored := storage.Load(nil); stored != nil {
		*settings = *stored
	}
	settings.path = path
	settings.applyDefaults()
	return settings
}

// applyDefaults fills the settings that an older file did not have.
func (s *Settings) applyDefaults() {
	defaults := DefaultSettings()

	if s.Language == "" {
		s.Language = defaults.Language
	}
	if s.Theme == "" {
		s.Theme = defaults.Theme
	}
	if s.ColorScheme == "" {
		s.ColorScheme = defaults.ColorScheme
	}
	if s.WindowSize <= 0 {
		s.WindowSize = defaults.WindowSize
	}
	if s.WindowHeightSize <= 0 {
		s.WindowHeightSize = defaults.WindowHeightSize
	}
	if s.ItemHeightSize <= 0 {
		s.ItemHeightSize = defaults.ItemHeightSize
	}
	if s.MaxResultsToShow < MinResultsToShow || s.MaxResultsToShow > MaxResultsToShowLimit {
		s.MaxResultsToShow = defaults.MaxResultsToShow
	}
	if s.SearchDelayTime <= 0 {
		s.SearchDelayTime = defaults.SearchDelayTime
	}
	if s.MaxHistoryResultsToShowForHomePage <= 0 {
		s.MaxHistoryResultsToShowForHomePage = defaults.MaxHistoryResultsToShowForHomePage
	}
	if s.Hotkey == "" {
		s.Hotkey = defaults.Hotkey
	}
	if s.PreviewHotkey == "" {
		s.PreviewHotkey = defaults.PreviewHotkey
	}
	if s.OpenResultModifiers == "" {
		s.OpenResultModifiers = defaults.OpenResultModifiers
	}
	if s.SearchWindowScreen == "" {
		s.SearchWindowScreen = defaults.SearchWindowScreen
	}
	if s.SearchWindowAlign == "" {
		s.SearchWindowAlign = defaults.SearchWindowAlign
	}
	if s.LastQueryMode == "" {
		s.LastQueryMode = defaults.LastQueryMode
	}
	if s.HistoryStyle == "" {
		s.HistoryStyle = defaults.HistoryStyle
	}
	if s.BackdropType == "" {
		s.BackdropType = defaults.BackdropType
	}
	if s.SoundVolume == 0 {
		s.SoundVolume = defaults.SoundVolume
	}
}

// The range the settings window offers for the number of results.
const (
	MinResultsToShow      = 2
	MaxResultsToShowLimit = 17
)

// Save writes the settings out.
func (s *Settings) Save() error {
	if s.path == "" {
		return nil
	}
	return JSONStorage[*Settings]{Path: s.path}.Save(s)
}

// Path is where the settings are written.
func (s *Settings) Path() string { return s.path }

// SetPath says where the settings are written.
func (s *Settings) SetPath(path string) { s.path = path }

// SearchPrecisionScoreFromString reads a precision from the settings file,
// where it is written by name.
func SearchPrecisionScoreFromString(value string) SearchPrecisionScore {
	switch value {
	case "Regular":
		return SearchPrecisionRegular
	case "Low":
		return SearchPrecisionLow
	default:
		return SearchPrecisionNone
	}
}

// SettingsPath is where the launcher's settings live, under its data
// directory.
func SettingsPath(dataDir string) string {
	return filepath.Join(dataDir, "Settings", "Settings.json")
}
