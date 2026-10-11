package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Hooks are what the launcher itself does for a plugin: the window, the
// settings, the tray. The app fills them in as it wires itself up; a plugin
// calls the API's methods rather than these.
type Hooks struct {
	ChangeQuery         func(query string, requery bool)
	RestartApp          func()
	ReloadAllPluginData func() error
	CheckForNewUpdate   func()
	SaveAppAllSettings  func()
	SavePluginSettings  func()
	ShowMainWindow      func()
	HideMainWindow      func()
	FocusQueryTextBox   func()
	IsMainWindowVisible func() bool
	OpenSettingDialog   func()
	OpenPluginSettings  func(pluginID string) bool
	ShowMsg             func(title, subTitle, iconPath string)
	ShowMsgWithButton   func(title, buttonText string, action func(), subTitle, iconPath string)
	LogDirectory        func() string
	Exit                func()
	// SetOpenAtLogin makes the launcher start when the user logs in.
	SetOpenAtLogin         func(enabled bool) error
	ShowMsgError           func(title, subTitle string)
	ShowMsgErrorWithButton func(title, buttonText string, action func(), subTitle string)
	ShowMsgBox             func(messageBoxText, caption string, buttons MessageBoxButtons, icon MessageBoxIcon) MessageBoxResult
	StartLoadingBar        func()
	StopLoadingBar         func()
	ToggleGameMode         func()
	SetGameMode            func(value bool)
	IsGameModeOn           func() bool
	ReQuery                func(reselect bool)
	BackToQueryResults     func()
	IsDarkTheme            func() bool
	GetCurrentTheme        func() ThemeData
	GetAvailableThemes     func() []ThemeData
	SetCurrentTheme        func(theme ThemeData) bool
	InstallPlugin          func(plugin UserPlugin, zipFilePath string) bool
	UninstallPlugin        func(pluginID string, removePluginSettings bool) error
	UpdatePlugin           func(pluginID string, plugin UserPlugin, zipFilePath string) error
	GetPluginManifest      func() []UserPlugin
	UpdatePluginManifest   func(usePrimaryURLOnly bool) error
	// UpdateManifest reads the plugin store again, and is what a plugin of
	// the launcher's own calls.
	UpdateManifest  func(force bool) error
	PluginModified  func(id string) bool
	OpenDirectory   func(directoryPath, fileNameOrFilePath string)
	OpenURL         func(rawURL string, inPrivate *bool)
	OpenAppURI      func(appURI string)
	ShellRun        func(cmd, filename string)
	CopyToClipboard func(text string, directCopy, showDefaultNotification bool)
}

// MessageBoxButtons and MessageBoxIcon are what a message box offers.
type MessageBoxButtons int

// The buttons a message box can show.
const (
	MessageBoxOK MessageBoxButtons = iota
	MessageBoxOKCancel
	MessageBoxYesNo
)

// MessageBoxIcon is the picture on a message box.
type MessageBoxIcon int

// The pictures a message box can show.
const (
	MessageBoxIconNone MessageBoxIcon = iota
	MessageBoxIconInformation
	MessageBoxIconWarning
	MessageBoxIconError
	MessageBoxIconQuestion
)

// MessageBoxResult is the button the user pressed.
type MessageBoxResult int

// The buttons the user can have pressed.
const (
	MessageBoxResultOK MessageBoxResult = iota
	MessageBoxResultCancel
	MessageBoxResultYes
	MessageBoxResultNo
)

// PublicAPI is what a plugin calls back into: the launcher's own window and
// settings, the machine, and the network.
type PublicAPI struct {
	// DataDir is the launcher's data directory.
	DataDir string
	// SettingsDir is where the settings files live.
	SettingsDir string
	// PluginSettingsDir is where the plugins' own settings live.
	PluginSettingsDir string
	// PluginCacheDir is where the plugins' caches live.
	PluginCacheDir string
	// Version is the launcher's version, which plugins ask about.
	Version string

	// Hooks are filled in by the app.
	Hooks Hooks

	// Logger writes the launcher's log.
	Logger *Logger

	// HTTPClient is what plugins download with; nil uses the default, with
	// the user's proxy setting.
	HTTPClient *http.Client

	manager *PluginManager
	matcher StringMatcher
	mu      sync.Mutex
	// lastError is what the query form shows when a query fails.
	keyboardCallbacks []func(vkCode, scanCode int, state SpecialKeyState) bool
}

// NewPublicAPI makes the API plugins are given, over the directories the
// launcher keeps its data in.
func NewPublicAPI(manager *PluginManager, dataDir, version string) *PublicAPI {
	settingsDir := filepath.Join(dataDir, "Settings")
	api := &PublicAPI{
		DataDir:           dataDir,
		SettingsDir:       settingsDir,
		PluginSettingsDir: filepath.Join(settingsDir, "Plugins"),
		PluginCacheDir:    filepath.Join(dataDir, "Cache", "Plugins"),
		Version:           version,
		Logger:            NewLogger(filepath.Join(dataDir, "Logs")),
		manager:           manager,
		matcher:           StringMatcher{UserSettingSearchPrecision: SearchPrecisionRegular},
	}
	_ = os.MkdirAll(api.PluginSettingsDir, 0o755)
	_ = os.MkdirAll(api.PluginCacheDir, 0o755)
	return api
}

// SetMatcher gives the API the matcher plugins ask for fuzzy searches with.
func (a *PublicAPI) SetMatcher(matcher StringMatcher) { a.matcher = matcher }

// Matcher is the matcher plugins ask for fuzzy searches with.
func (a *PublicAPI) Matcher() StringMatcher { return a.matcher }

// FuzzySearch matches a query against a string, as the reference exposes it.
func (a *PublicAPI) FuzzySearch(query, stringToCompare string) MatchResult {
	return a.matcher.FuzzyMatch(query, stringToCompare)
}

// --- The window ---

// ChangeQuery puts text in the query box, optionally searching again.
func (a *PublicAPI) ChangeQuery(query string, requery bool) {
	if a.Hooks.ChangeQuery != nil {
		a.Hooks.ChangeQuery(query, requery)
	}
}

// ReQuery searches the same text again.
func (a *PublicAPI) ReQuery(reselect bool) {
	if a.Hooks.ReQuery != nil {
		a.Hooks.ReQuery(reselect)
	}
}

// BackToQueryResults leaves the context menu and shows the results again.
func (a *PublicAPI) BackToQueryResults() {
	if a.Hooks.BackToQueryResults != nil {
		a.Hooks.BackToQueryResults()
	}
}

// ShowMainWindow shows the launcher.
func (a *PublicAPI) ShowMainWindow() {
	if a.Hooks.ShowMainWindow != nil {
		a.Hooks.ShowMainWindow()
	}
}

// HideMainWindow hides it.
func (a *PublicAPI) HideMainWindow() {
	if a.Hooks.HideMainWindow != nil {
		a.Hooks.HideMainWindow()
	}
}

// FocusQueryTextBox puts the caret in the query box.
func (a *PublicAPI) FocusQueryTextBox() {
	if a.Hooks.FocusQueryTextBox != nil {
		a.Hooks.FocusQueryTextBox()
	}
}

// IsMainWindowVisible reports whether the launcher is showing.
func (a *PublicAPI) IsMainWindowVisible() bool {
	return a.Hooks.IsMainWindowVisible != nil && a.Hooks.IsMainWindowVisible()
}

// StartLoadingBar shows the launcher is working.
func (a *PublicAPI) StartLoadingBar() {
	if a.Hooks.StartLoadingBar != nil {
		a.Hooks.StartLoadingBar()
	}
}

// StopLoadingBar hides it again.
func (a *PublicAPI) StopLoadingBar() {
	if a.Hooks.StopLoadingBar != nil {
		a.Hooks.StopLoadingBar()
	}
}

// --- Settings ---

// OpenSettingDialog opens the settings window.
func (a *PublicAPI) OpenSettingDialog() {
	if a.Hooks.OpenSettingDialog != nil {
		a.Hooks.OpenSettingDialog()
	}
}

// OpenPluginSettingsWindow opens a plugin's settings page.
func (a *PublicAPI) OpenPluginSettingsWindow(pluginID string) bool {
	return a.Hooks.OpenPluginSettings != nil && a.Hooks.OpenPluginSettings(pluginID)
}

// SaveAppAllSettings writes the launcher's settings out.
func (a *PublicAPI) SaveAppAllSettings() {
	if a.Hooks.SaveAppAllSettings != nil {
		a.Hooks.SaveAppAllSettings()
	}
}

// SavePluginSettings writes the plugins' settings out.
func (a *PublicAPI) SavePluginSettings() {
	if a.Hooks.SavePluginSettings != nil {
		a.Hooks.SavePluginSettings()
	}
}

// ReloadAllPluginData asks every plugin to look its data up again.
func (a *PublicAPI) ReloadAllPluginData() error {
	if a.Hooks.ReloadAllPluginData != nil {
		return a.Hooks.ReloadAllPluginData()
	}
	return nil
}

// CheckForNewUpdate looks for a new version of the launcher.
func (a *PublicAPI) CheckForNewUpdate() {
	if a.Hooks.CheckForNewUpdate != nil {
		a.Hooks.CheckForNewUpdate()
	}
}

// RestartApp starts the launcher again.
func (a *PublicAPI) RestartApp() {
	if a.Hooks.RestartApp != nil {
		a.Hooks.RestartApp()
	}
}

// GetDataDirectory is the launcher's data directory.
func (a *PublicAPI) GetDataDirectory() string { return a.DataDir }

// LogDirectory is where the launcher writes its log.
func (a *PublicAPI) LogDirectory() string {
	if a.Hooks.LogDirectory != nil {
		return a.Hooks.LogDirectory()
	}
	return filepath.Join(a.DataDir, "Logs")
}

// Exit quits the launcher.
func (a *PublicAPI) Exit() {
	if a.Hooks.Exit != nil {
		a.Hooks.Exit()
	}
}

// SetOpenAtLogin makes the launcher start when the user logs in.
func (a *PublicAPI) SetOpenAtLogin(enabled bool) error {
	if a.Hooks.SetOpenAtLogin == nil {
		return nil
	}
	return a.Hooks.SetOpenAtLogin(enabled)
}

// GetTranslation translates a key of the launcher's own language files.
func (a *PublicAPI) GetTranslation(key string) string { return T(CurrentLanguage(), key) }

// --- Plugins ---

// GetAllPlugins returns every plugin the launcher knows.
func (a *PublicAPI) GetAllPlugins() []PluginPair {
	if a.manager == nil {
		return nil
	}
	return a.manager.AllLoadedPlugins()
}

// GetAllInitializedPlugins returns the plugins that initialized.
func (a *PublicAPI) GetAllInitializedPlugins(includeFailed bool) []PluginPair {
	if a.manager == nil {
		return nil
	}
	return a.manager.AllInitializedPlugins(includeFailed)
}

// AddActionKeyword gives a plugin another keyword.
func (a *PublicAPI) AddActionKeyword(pluginID, keyword string) {
	if a.manager != nil {
		a.manager.AddActionKeyword(pluginID, keyword)
	}
}

// RemoveActionKeyword takes one away.
func (a *PublicAPI) RemoveActionKeyword(pluginID, keyword string) {
	if a.manager != nil {
		a.manager.RemoveActionKeyword(pluginID, keyword)
	}
}

// ActionKeywordAssigned reports whether a keyword is already taken.
func (a *PublicAPI) ActionKeywordAssigned(keyword string) bool {
	if a.manager == nil {
		return false
	}
	if keyword == GlobalPluginWildcardSign {
		return len(a.manager.GlobalPlugins()) > 0
	}
	return len(a.manager.NonGlobalPlugins()[keyword]) > 0
}

// PluginModified reports whether the user changed a plugin's files, which
// the launcher leaves out of queries until it is reloaded.
func (a *PublicAPI) PluginModified(id string) bool {
	return a.Hooks.PluginModified != nil && a.Hooks.PluginModified(id)
}

// InstallPlugin installs a plugin from a downloaded zip file.
func (a *PublicAPI) InstallPlugin(plugin UserPlugin, zipFilePath string) bool {
	return a.Hooks.InstallPlugin != nil && a.Hooks.InstallPlugin(plugin, zipFilePath)
}

// UninstallPlugin removes a plugin.
func (a *PublicAPI) UninstallPlugin(pluginID string, removePluginSettings bool) error {
	if a.Hooks.UninstallPlugin == nil {
		return fmt.Errorf("the launcher is not ready to uninstall plugins")
	}
	return a.Hooks.UninstallPlugin(pluginID, removePluginSettings)
}

// UpdatePlugin replaces a plugin with a newer version.
func (a *PublicAPI) UpdatePlugin(pluginID string, plugin UserPlugin, zipFilePath string) error {
	if a.Hooks.UpdatePlugin == nil {
		return fmt.Errorf("the launcher is not ready to update plugins")
	}
	return a.Hooks.UpdatePlugin(pluginID, plugin, zipFilePath)
}

// GetPluginManifest is the list of plugins the launcher can install.
func (a *PublicAPI) GetPluginManifest() []UserPlugin {
	if a.Hooks.GetPluginManifest == nil {
		return nil
	}
	return a.Hooks.GetPluginManifest()
}

// UpdatePluginManifest downloads that list again.
func (a *PublicAPI) UpdatePluginManifest(usePrimaryURLOnly bool) error {
	if a.Hooks.UpdateManifest != nil {
		return a.Hooks.UpdateManifest(usePrimaryURLOnly)
	}
	if a.Hooks.UpdatePluginManifest == nil {
		return fmt.Errorf("the launcher is not ready to update the plugin list")
	}
	return a.Hooks.UpdatePluginManifest(usePrimaryURLOnly)
}

// RegisterGlobalKeyboardCallback is told about every key press, and reports
// whether it took the key.
func (a *PublicAPI) RegisterGlobalKeyboardCallback(callback func(vkCode, scanCode int, state SpecialKeyState) bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keyboardCallbacks = append(a.keyboardCallbacks, callback)
}

// RemoveGlobalKeyboardCallback stops telling it.
func (a *PublicAPI) RemoveGlobalKeyboardCallback(callback func(vkCode, scanCode int, state SpecialKeyState) bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	kept := a.keyboardCallbacks[:0]
	for _, candidate := range a.keyboardCallbacks {
		if fmt.Sprintf("%p", candidate) != fmt.Sprintf("%p", callback) {
			kept = append(kept, candidate)
		}
	}
	a.keyboardCallbacks = kept
}

// KeyboardCallbacks returns the callbacks a key press goes to.
func (a *PublicAPI) KeyboardCallbacks() []func(vkCode, scanCode int, state SpecialKeyState) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]func(vkCode, scanCode int, state SpecialKeyState) bool(nil), a.keyboardCallbacks...)
}

// --- The machine ---

// OpenDirectory shows a directory, optionally with a file inside it
// selected.
func (a *PublicAPI) OpenDirectory(directoryPath, fileNameOrFilePath string) {
	if a.Hooks.OpenDirectory != nil {
		a.Hooks.OpenDirectory(directoryPath, fileNameOrFilePath)
	}
}

// OpenWebUrl opens a URL in the user's browser.
func (a *PublicAPI) OpenWebUrl(rawURL string, inPrivate *bool) {
	if a.Hooks.OpenURL != nil {
		a.Hooks.OpenURL(rawURL, inPrivate)
	}
}

// OpenUrl opens a URL, which may be of an app rather than of the web.
func (a *PublicAPI) OpenUrl(rawURL string, inPrivate *bool) { a.OpenWebUrl(rawURL, inPrivate) }

// OpenAppUri opens an app's URI, as Windows settings pages use.
func (a *PublicAPI) OpenAppUri(appURI string) {
	if a.Hooks.OpenAppURI != nil {
		a.Hooks.OpenAppURI(appURI)
	}
}

// ShellRun runs a command line.
func (a *PublicAPI) ShellRun(cmd, filename string) {
	if a.Hooks.ShellRun != nil {
		a.Hooks.ShellRun(cmd, filename)
	}
}

// CopyToClipboard puts text, or the file at a path, on the clipboard.
func (a *PublicAPI) CopyToClipboard(text string, directCopy, showDefaultNotification bool) {
	if a.Hooks.CopyToClipboard != nil {
		a.Hooks.CopyToClipboard(text, directCopy, showDefaultNotification)
	}
}

// ToggleGameMode holds the launcher back while the user plays.
func (a *PublicAPI) ToggleGameMode() {
	if a.Hooks.ToggleGameMode != nil {
		a.Hooks.ToggleGameMode()
	}
}

// SetGameMode sets it.
func (a *PublicAPI) SetGameMode(value bool) {
	if a.Hooks.SetGameMode != nil {
		a.Hooks.SetGameMode(value)
	}
}

// IsGameModeOn reports whether it is on.
func (a *PublicAPI) IsGameModeOn() bool {
	return a.Hooks.IsGameModeOn != nil && a.Hooks.IsGameModeOn()
}

// IsApplicationDarkTheme reports whether the launcher is drawn dark.
func (a *PublicAPI) IsApplicationDarkTheme() bool {
	return a.Hooks.IsDarkTheme != nil && a.Hooks.IsDarkTheme()
}

// GetAvailableThemes returns the themes the launcher can wear.
func (a *PublicAPI) GetAvailableThemes() []ThemeData {
	if a.Hooks.GetAvailableThemes == nil {
		return nil
	}
	return a.Hooks.GetAvailableThemes()
}

// GetCurrentTheme returns the theme it is wearing.
func (a *PublicAPI) GetCurrentTheme() ThemeData {
	if a.Hooks.GetCurrentTheme == nil {
		return ThemeData{}
	}
	return a.Hooks.GetCurrentTheme()
}

// SetCurrentTheme puts on a theme, and reports whether it could.
func (a *PublicAPI) SetCurrentTheme(theme ThemeData) bool {
	return a.Hooks.SetCurrentTheme != nil && a.Hooks.SetCurrentTheme(theme)
}

// --- Messages ---

// ShowMsg shows a notification.
func (a *PublicAPI) ShowMsg(title, subTitle, iconPath string) {
	if a.Hooks.ShowMsg != nil {
		a.Hooks.ShowMsg(title, subTitle, iconPath)
	}
}

// ShowMsgWithButton shows one with a button that runs an action.
func (a *PublicAPI) ShowMsgWithButton(title, buttonText string, action func(), subTitle, iconPath string) {
	if a.Hooks.ShowMsgWithButton != nil {
		a.Hooks.ShowMsgWithButton(title, buttonText, action, subTitle, iconPath)
	}
}

// ShowMsgError shows a notification saying something failed.
func (a *PublicAPI) ShowMsgError(title, subTitle string) {
	if a.Hooks.ShowMsgError != nil {
		a.Hooks.ShowMsgError(title, subTitle)
	}
}

// ShowMsgErrorWithButton shows one with a button that runs an action.
func (a *PublicAPI) ShowMsgErrorWithButton(title, buttonText string, action func(), subTitle string) {
	if a.Hooks.ShowMsgErrorWithButton != nil {
		a.Hooks.ShowMsgErrorWithButton(title, buttonText, action, subTitle)
	}
}

// ShowMsgBox asks the user a question, and waits for the answer.
func (a *PublicAPI) ShowMsgBox(text, caption string, buttons MessageBoxButtons, icon MessageBoxIcon) MessageBoxResult {
	if a.Hooks.ShowMsgBox == nil {
		return MessageBoxResultOK
	}
	return a.Hooks.ShowMsgBox(text, caption, buttons, icon)
}

// --- The network ---

// httpClient is the client plugins download with.
func (a *PublicAPI) httpClient() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return http.DefaultClient
}

// HttpGetStringAsync downloads a URL as text.
func (a *PublicAPI) HttpGetStringAsync(ctx context.Context, rawURL string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	response, err := a.httpClient().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("GET %s answered %s", rawURL, response.Status)
	}
	return string(body), nil
}

// HttpGetStreamAsync downloads a URL as a stream.
func (a *PublicAPI) HttpGetStreamAsync(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := a.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		response.Body.Close()
		return nil, fmt.Errorf("GET %s answered %s", rawURL, response.Status)
	}
	return response.Body, nil
}

// HttpDownloadAsync downloads a URL to a file, reporting how far it got from
// 0 to 1.
func (a *PublicAPI) HttpDownloadAsync(ctx context.Context, rawURL, filePath string, reportProgress func(float64)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	response, err := a.httpClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("GET %s answered %s", rawURL, response.Status)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	total := response.ContentLength
	written := int64(0)
	buffer := make([]byte, 64*1024)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err := file.Write(buffer[:n]); err != nil {
				return err
			}
			written += int64(n)
			if reportProgress != nil && total > 0 {
				reportProgress(float64(written) / float64(total))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if reportProgress != nil {
		reportProgress(1)
	}
	return nil
}

// --- Logging ---

// LogDebug writes a line to the log.
func (a *PublicAPI) LogDebug(className, message string) { a.Logger.Debug(className, message) }

// LogInfo writes a line to the log.
func (a *PublicAPI) LogInfo(className, message string) { a.Logger.Info(className, message) }

// LogWarn writes a line to the log.
func (a *PublicAPI) LogWarn(className, message string) { a.Logger.Warn(className, message) }

// LogError writes a line to the log.
func (a *PublicAPI) LogError(className, message string) { a.Logger.Error(className, message) }

// LogException writes a failure to the log.
func (a *PublicAPI) LogException(className, message string, err error) {
	a.Logger.Error(className, message+": "+err.Error())
}

// Logger writes the launcher's log file. The file is opened for each line
// and closed again, so that a launcher that logs nothing makes no file, and
// so that a test that logs does not hold a file open under a directory it
// is about to remove.
type Logger struct {
	dir string

	mu sync.Mutex
}

// NewLogger makes a logger writing under a directory.
func NewLogger(dir string) *Logger { return &Logger{dir: dir} }

func (l *Logger) write(level, className, message string) {
	if l == nil {
		return
	}
	line := fmt.Sprintf("%s|%s|%s|%s\n", time.Now().Format("2006-01-02 15:04:05.000"), level, className, message)

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(l.dir, "Log_"+time.Now().Format("2006-01-02")+".txt")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = file.WriteString(line)
}

// Debug writes a line to the log.
func (l *Logger) Debug(className, message string) { l.write("DEBUG", className, message) }

// Info writes a line to the log.
func (l *Logger) Info(className, message string) { l.write("INFO", className, message) }

// Warn writes a line to the log.
func (l *Logger) Warn(className, message string) { l.write("WARN", className, message) }

// Error writes a line to the log.
func (l *Logger) Error(className, message string) { l.write("ERROR", className, message) }

// --- Plugin settings storage ---

// LoadSettingJsonStorage reads a plugin's settings, or makes new ones.
func LoadSettingJsonStorage[T any](api *PublicAPI, pluginName, file string) T {
	var zero T
	path := pluginSettingsPath(api.PluginSettingsDir, pluginName, file)
	return JSONStorage[T]{Path: path}.Load(zero)
}

// SaveSettingJsonStorage writes a plugin's settings out.
func SaveSettingJsonStorage[T any](api *PublicAPI, pluginName, file string, value T) error {
	path := pluginSettingsPath(api.PluginSettingsDir, pluginName, file)
	storage := JSONStorage[T]{Path: path}
	if err := storage.Save(value); err != nil {
		api.LogError("JsonStorage", fmt.Sprintf("failed to save %s: %v", path, err))
		return err
	}
	return nil
}

// LoadCacheBinaryStorage reads a plugin's cache of any JSON value.
func LoadCacheBinaryStorage[T any](api *PublicAPI, pluginName, cacheName string) (T, bool) {
	var zero T
	path := filepath.Join(api.PluginCacheDir, pluginName, cacheName+".json")
	serialized, err := os.ReadFile(path)
	if err != nil {
		return zero, false
	}
	var value T
	if err := json.Unmarshal(serialized, &value); err != nil {
		return zero, false
	}
	return value, true
}

// SaveCacheBinaryStorage writes a plugin's cache.
func SaveCacheBinaryStorage[T any](api *PublicAPI, pluginName, cacheName string, value T) error {
	path := filepath.Join(api.PluginCacheDir, pluginName, cacheName+".json")
	return JSONStorage[T]{Path: path}.Save(value)
}

// Logf writes to the standard logger, for the launcher's own diagnostics.
func Logf(format string, args ...any) { log.Printf(format, args...) }

// TrimPath shortens a path for a log line.
func TrimPath(p string) string { return strings.TrimSpace(p) }
