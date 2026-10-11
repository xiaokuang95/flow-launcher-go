package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// echoRunner stands in for a plugin's program, so that the protocol is
// tested without running anything.
type echoRunner struct {
	requests [][]byte
	answer   string
	err      error
}

func (r *echoRunner) Run(ctx context.Context, directory string, request []byte) ([]byte, error) {
	r.requests = append(r.requests, request)
	if r.err != nil {
		return nil, r.err
	}
	return []byte(r.answer), nil
}

func TestJSONRPCQuery(t *testing.T) {
	runner := &echoRunner{answer: `{"id":1,"result":[
		{"title":"From Python","subTitle":"a plugin of its own","icoPath":"Images/x.png","score":42,
		 "titleHighlightData":[0,1],
		 "jsonRPCAction":{"id":7,"method":"do_it","parameters":["a",1]}}
	]}`}

	plugin := NewJSONRPCPlugin(PluginMetadata{ID: "py", Name: "Python Plugin"}, t.TempDir(), runner)
	results, err := plugin.Query(context.Background(), Query{Search: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want one", len(results))
	}
	if results[0].Title != "From Python" || results[0].Score != 42 {
		t.Errorf("the result is %+v", results[0])
	}
	if len(results[0].TitleHighlightData) != 2 {
		t.Errorf("the highlight is %v", results[0].TitleHighlightData)
	}

	// The request the plugin was asked is the reference's own shape.
	var request JSONRPCRequest
	if err := json.Unmarshal(runner.requests[0], &request); err != nil {
		t.Fatal(err)
	}
	if request.Method != "query" {
		t.Errorf("the method is %q, want query", request.Method)
	}
	if len(request.Parameters) != 1 || request.Parameters[0] != "hello" {
		t.Errorf("the parameters are %v, want what was typed", request.Parameters)
	}
}

func TestJSONRPCActionRunsThePluginsMethod(t *testing.T) {
	runner := &echoRunner{answer: `{"id":1,"result":[{"title":"Go","jsonRPCAction":{"id":7,"method":"run_it","parameters":["x"]}}]}`}
	plugin := NewJSONRPCPlugin(PluginMetadata{ID: "py", Name: "Python Plugin"}, t.TempDir(), runner)

	results, err := plugin.Query(context.Background(), Query{Search: "go"})
	if err != nil {
		t.Fatal(err)
	}
	hide := results[0].Execute(context.Background(), ActionContext{})
	if !hide {
		t.Error("the launcher did not hide after the action")
	}

	if len(runner.requests) != 2 {
		t.Fatalf("the plugin was asked %d times, want the query and the action", len(runner.requests))
	}
	var action JSONRPCRequest
	if err := json.Unmarshal(runner.requests[1], &action); err != nil {
		t.Fatal(err)
	}
	if action.Method != "run_it" || action.ID != 7 {
		t.Errorf("the action asked was %+v", action)
	}
}

func TestJSONRPCDontHideAfterAction(t *testing.T) {
	runner := &echoRunner{answer: `{"id":1,"result":[{"title":"Go","jsonRPCAction":{"id":7,"method":"run_it","dontHideAfterAction":true}}]}`}
	plugin := NewJSONRPCPlugin(PluginMetadata{ID: "py"}, t.TempDir(), runner)

	results, _ := plugin.Query(context.Background(), Query{Search: "go"})
	if hide := results[0].Execute(context.Background(), ActionContext{}); hide {
		t.Error("the launcher hid although the plugin asked it to stay")
	}
}

func TestJSONRPCLoadExternalPlugins(t *testing.T) {
	directory := t.TempDir()
	pluginDirectory := filepath.Join(directory, "MyPlugin")
	if err := os.MkdirAll(pluginDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	described := `{"ID":"my-plugin","Name":"My Plugin","Language":"python","ActionKeyword":"mp","ExecuteFileName":"main.py"}`
	if err := os.WriteFile(filepath.Join(pluginDirectory, "plugin.json"), []byte(described), 0o644); err != nil {
		t.Fatal(err)
	}
	// A plugin written in a language the launcher has no host for.
	dotnetDirectory := filepath.Join(directory, "DotNetPlugin")
	if err := os.MkdirAll(dotnetDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	describedDotNet := `{"ID":"dotnet","Name":"DotNet Plugin","Language":"csharp","ExecuteFileName":"x.dll"}`
	if err := os.WriteFile(filepath.Join(dotnetDirectory, "plugin.json"), []byte(describedDotNet), 0o644); err != nil {
		t.Fatal(err)
	}

	settings := &PluginsSettings{PythonExecutablePath: "python.exe", NodeExecutablePath: "node.exe"}
	pairs, unsupported := LoadExternalPlugins(directory, settings)

	if len(pairs) != 1 {
		t.Fatalf("loaded %d plugins, want the one written in Python", len(pairs))
	}
	if pairs[0].Metadata.Name != "My Plugin" {
		t.Errorf("the plugin is %q", pairs[0].Metadata.Name)
	}
	if pairs[0].Metadata.ActionKeyword != "mp" {
		t.Errorf("the keyword is %q, want mp", pairs[0].Metadata.ActionKeyword)
	}
	if len(unsupported) != 1 {
		t.Fatalf("the unsupported plugins are %v, want the .NET one", unsupported)
	}
}

func TestJSONRPCMetadataDefaultsToTheWildcard(t *testing.T) {
	description := PluginDescription{ID: "x", Name: "X", Language: "python"}
	metadata := description.Metadata("/plugins/x")
	if len(metadata.ActionKeywords) != 1 || metadata.ActionKeywords[0] != GlobalPluginWildcardSign {
		t.Errorf("the keywords are %v, want the wildcard", metadata.ActionKeywords)
	}
	if metadata.IcoPath != "" {
		t.Errorf("the icon is %q, want none", metadata.IcoPath)
	}
}
