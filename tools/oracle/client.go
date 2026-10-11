// Package oracle runs the reference implementation over test cases, so
// that this port can be checked against the code it copies rather than
// against a reading of it.
//
// It builds the C# project beside it, which holds the reference's own
// sources, and answers cases through it. Tests skip themselves when the
// .NET SDK is not installed.
package oracle

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// exePath is where the oracle is built, once per test run.
var (
	buildOnce sync.Once
	exePath   string
	buildErr  error
)

// Available reports whether the reference can be run here.
func Available() bool {
	_, err := exec.LookPath("dotnet")
	return err == nil
}

// Exe builds the oracle if it has not been built, and returns its path.
func Exe(t *testing.T) string {
	t.Helper()

	if !Available() {
		t.Skip("dotnet is not installed, so the reference implementation cannot be run")
	}

	buildOnce.Do(func() {
		dir, err := directory()
		if err != nil {
			buildErr = err
			return
		}

		build := exec.Command("dotnet", "build", "-v", "q", "--nologo")
		build.Dir = dir
		if out, err := build.CombinedOutput(); err != nil {
			buildErr = err
			t.Logf("building the oracle failed: %s", out)
			return
		}
		exePath = filepath.Join(dir, "bin", "Debug", "net9.0-windows", "oracle.exe")
	})

	if buildErr != nil {
		t.Fatalf("building the oracle failed: %v", buildErr)
	}
	return exePath
}

// directory is the oracle project, found from this file's own location so
// that a test may run from any directory.
func directory() (string, error) {
	_, file, _, ok := callerFile()
	if !ok {
		return filepath.Abs(".")
	}
	return filepath.Dir(file), nil
}

// Ask runs every case through the reference in one go and returns what it
// answered, one answer per case.
func Ask(t *testing.T, cases ...any) []map[string]any {
	t.Helper()

	if len(cases) == 0 {
		return nil
	}

	var input strings.Builder
	for _, c := range cases {
		line, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(line)
		input.WriteByte('\n')
	}

	cmd := exec.Command(Exe(t))
	cmd.Stdin = strings.NewReader(input.String())
	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		answered := len(strings.Split(strings.TrimSpace(string(out)), "\n"))
		if strings.TrimSpace(string(out)) == "" {
			answered = 0
		}
		t.Fatalf("running the oracle failed after %d of %d cases: %v\ncase %d: %+v\nstderr: %s",
			answered, len(cases), err, answered, at(cases, answered), stderr.String())
	}

	answers := make([]map[string]any, 0, len(cases))
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var answer map[string]any
		if err := json.Unmarshal([]byte(line), &answer); err != nil {
			t.Fatalf("the oracle answered %q: %v", line, err)
		}
		answers = append(answers, answer)
	}
	if len(answers) != len(cases) {
		t.Fatalf("the oracle answered %d of %d cases", len(answers), len(cases))
	}
	return answers
}

// String reads a string field of an answer.
func String(answer map[string]any, name string) string {
	if value, ok := answer[name].(string); ok {
		return value
	}
	return ""
}

// Number reads a number field of an answer. The reference writes its
// answers as text, so a field that is a number written out is read too.
func Number(answer map[string]any, name string) float64 {
	if value, ok := answer[name].(float64); ok {
		return value
	}
	if text, ok := answer[name].(string); ok {
		var value float64
		if err := json.Unmarshal([]byte(text), &value); err == nil {
			return value
		}
	}
	return 0
}

// Integers reads a list of numbers of an answer.
func Integers(answer map[string]any, name string) []int {
	values, ok := answer[name].([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(values))
	for _, value := range values {
		number, ok := value.(float64)
		if !ok {
			return nil
		}
		out = append(out, int(number))
	}
	return out
}

// Error is what the oracle answered when it could not run a case.
func Error(answer map[string]any) string { return String(answer, "error") }

// at returns the case at an index, or nil when there is none.
func at(cases []any, index int) any {
	if index < 0 || index >= len(cases) {
		return nil
	}
	return cases[index]
}

// callerFile is this file's path, so that the project is found beside it
// wherever the test runs from.
func callerFile() (string, string, int, bool) {
	_, file, _, ok := runtime.Caller(0)
	return file, file, 0, ok
}
