package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"launch/core"
	"launch/plugins/program"
)

func TestTheLogLevelSaysHowMuchIsWritten(t *testing.T) {
	directory := t.TempDir()
	logger := core.NewLogger(directory)

	logger.SetLevel(core.LogLevelERROR)
	logger.Info("launcher", "something happened")
	logger.Error("launcher", "something failed")

	log := readLog(t, directory)
	if strings.Contains(log, "something happened") {
		t.Errorf("the log holds what the level leaves out: %q", log)
	}
	if !strings.Contains(log, "something failed") {
		t.Errorf("the log does not hold the failure: %q", log)
	}

	// And the level is the user's to change.
	logger.SetLevel(core.LogLevelDEBUG)
	logger.Debug("launcher", "the details")
	if log := readLog(t, directory); !strings.Contains(log, "the details") {
		t.Errorf("the log does not hold the details: %q", log)
	}
}

func TestALevelOfNoneWritesNothing(t *testing.T) {
	directory := t.TempDir()
	logger := core.NewLogger(directory)

	logger.SetLevel(core.LogLevelNONE)
	logger.Error("launcher", "something failed")

	if entries, err := os.ReadDir(directory); err == nil && len(entries) > 0 {
		t.Errorf("a log file was written at a level that writes nothing: %v", entries)
	}
}

// readLog is everything that has been written to the log.
func readLog(t *testing.T, directory string) string {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		return ""
	}

	var written strings.Builder
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		written.Write(content)
	}
	return written.String()
}

func TestBadgesAreDrawnUnlessTheUserSaysOtherwise(t *testing.T) {
	programPlugin := program.New()
	a := pluginApp(programPlugin)
	a.settings = core.DefaultSettings()

	// The history's badge is drawn for a result of a plugin that answers
	// every query, and the setting can say otherwise.
	result := core.Result{PluginID: programPlugin.Metadata().ID}
	if !a.settings.ShowBadges {
		t.Errorf("badges start off, want them on as the reference has them")
	}
	if !a.badgeShows(result) {
		t.Errorf("a badge of a plugin that answers every query is not drawn")
	}

	a.settings.ShowBadgesGlobalOnly = true
	other := core.Result{PluginID: "not-a-plugin"}
	if a.badgeShows(other) {
		t.Errorf("a badge of a plugin that is not there is drawn")
	}
}
