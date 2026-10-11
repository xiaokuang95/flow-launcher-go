package main

import (
	"testing"

	"launch/core"
	"launch/plugins/calculator"
	"launch/plugins/program"
)

// pluginApp is a launcher with the plugins given, without a window, which is
// enough for the settings window's own decisions.
func pluginApp(plugins ...core.Plugin) *app {
	manager := core.NewPluginManager()
	for _, plugin := range plugins {
		metadata := plugin.Metadata()
		manager.Load(core.PluginPair{Plugin: plugin, Metadata: metadata})
	}
	return &app{manager: manager}
}

func TestAPluginWithSettingsIsOfferedItsOwnWindow(t *testing.T) {
	programPlugin := program.New()
	a := pluginApp(programPlugin)

	fields, ok := pluginSettingsFor(a, programPlugin.Metadata().ID)
	if !ok {
		t.Fatalf("the program plugin has no settings the window can show")
	}

	// The switches that decide where programs are looked for are what the
	// user came for.
	found := map[string]bool{}
	for _, field := range fields {
		found[field.Key] = true
	}
	for _, want := range []string{"EnableStartMenuSource", "EnableRegistrySource", "EnablePathSource", "EnableUwpSource", "EnableDescription", "HideAppsPath"} {
		if !found[want] {
			t.Errorf("the form does not hold %q", want)
		}
	}
}

func TestAPluginWithoutSettingsIsNotOfferedOne(t *testing.T) {
	a := pluginApp(program.New())

	// The calculator has settings of its own; one that does not is not
	// offered a window it would have nothing to show in.
	if _, ok := pluginSettingsFor(a, "not-a-plugin"); ok {
		t.Errorf("a plugin that is not there was offered a settings window")
	}
}

func TestTheCalculatorHandsItsSettingsOver(t *testing.T) {
	calculatorPlugin := calculator.New()
	a := pluginApp(calculatorPlugin)

	fields, ok := pluginSettingsFor(a, calculatorPlugin.Metadata().ID)
	if !ok || len(fields) == 0 {
		t.Fatalf("the calculator has no settings the window can show")
	}
}
