package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file checks the port against the reference's own implementation
// rather than against a reading of it. tools/oracle runs the reference's
// StringMatcher, unchanged, over cases given to it on stdin; this test
// generates a corpus, runs both, and fails on the first difference.
//
// The test skips itself when the .NET SDK is not installed.

type oracleCase struct {
	Query         string `json:"query"`
	Compare       string `json:"compare"`
	Precision     int    `json:"precision,omitempty"`
	IgnoreAccents bool   `json:"ignoreAccents,omitempty"`
}

type oracleResult struct {
	Index     int    `json:"index"`
	Success   bool   `json:"success"`
	RawScore  int    `json:"rawScore"`
	Score     int    `json:"score"`
	Met       bool   `json:"met"`
	MatchData []int  `json:"matchData"`
	Error     string `json:"error"`
}

// oracleExe builds the oracle project if needed and returns its path.
func oracleExe(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("dotnet"); err != nil {
		t.Skip("dotnet is not installed, so the reference implementation cannot be run")
	}

	dir, err := filepath.Abs(filepath.Join("..", "tools", "oracle"))
	if err != nil {
		t.Fatal(err)
	}

	// The oracle targets net9.0-windows for the reference's WPF types, so
	// its output lives under that name.
	exe := filepath.Join(dir, "bin", "Debug", "net9.0-windows", "oracle.exe")
	if _, err := os.Stat(exe); err == nil {
		return exe
	}

	build := exec.Command("dotnet", "build", "-v", "q", "--nologo")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the oracle failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("the oracle did not build: %v", err)
	}
	return exe
}

// runOracle runs every case through the reference implementation.
func runOracle(t *testing.T, exe string, cases []oracleCase) []oracleResult {
	t.Helper()

	var in strings.Builder
	for _, c := range cases {
		line, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		in.Write(line)
		in.WriteByte('\n')
	}

	cmd := exec.Command(exe)
	cmd.Stdin = strings.NewReader(in.String())
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		// A crash leaves the cases it never reached without an answer, so
		// name the one that killed it.
		answered := len(strings.Split(strings.TrimSpace(string(out)), "\n"))
		if strings.TrimSpace(string(out)) == "" {
			answered = 0
		}
		t.Fatalf("running the oracle failed after %d of %d cases: %v\ncase %d: %+v\nstderr: %s",
			answered, len(cases), err, answered, at(cases, answered), stderr.String())
	}

	var results []oracleResult
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var r oracleResult
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatalf("the oracle answered %q: %v", scanner.Text(), err)
		}
		results = append(results, r)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(results) != len(cases) {
		t.Fatalf("the oracle answered %d of %d cases", len(results), len(cases))
	}
	return results
}

// runMine runs every case through this port.
func runMine(cases []oracleCase) []oracleResult {
	results := make([]oracleResult, 0, len(cases))
	for _, c := range cases {
		m := StringMatcher{
			UserSettingSearchPrecision: SearchPrecisionScore(c.Precision),
			IgnoreAccents:              c.IgnoreAccents,
		}
		if c.Precision == 0 {
			m.UserSettingSearchPrecision = SearchPrecisionRegular
		}
		r := m.Match(c.Query, c.Compare, DefaultMatchOption())
		results = append(results, oracleResult{
			Success:   r.Success,
			RawScore:  r.RawScore,
			Score:     r.Score(),
			Met:       r.IsSearchPrecisionScoreMet(),
			MatchData: r.MatchData,
		})
	}
	return results
}

func TestMatchesTheReferenceImplementation(t *testing.T) {
	exe := oracleExe(t)

	cases := matcherCorpus()
	got := runMine(cases)
	want := runOracle(t, exe, cases)

	failures := 0
	crashes := 0
	for i := range cases {
		if want[i].Error != "" {
			// The reference throws on a query that is nothing but whitespace:
			// it trims the query, then indexes the empty list of substrings.
			// This port answers that there was no match instead of crashing,
			// which is the one deliberate difference between the two.
			if !strings.Contains(want[i].Error, "IndexOutOfRangeException") || got[i].Success {
				t.Fatalf("the oracle failed on %+v: %s", cases[i], want[i].Error)
			}
			crashes++
			continue
		}
		if got[i].Success != want[i].Success || got[i].RawScore != want[i].RawScore ||
			got[i].Score != want[i].Score || got[i].Met != want[i].Met ||
			!sameIndices(got[i].MatchData, want[i].MatchData) {
			failures++
			if failures <= 20 {
				t.Errorf("match(%q, %q) precision %d ignoreAccents %v:\n  got  %+v\n  want %+v",
					cases[i].Query, cases[i].Compare, cases[i].Precision, cases[i].IgnoreAccents, got[i], want[i])
			}
		}
	}
	if failures > 0 {
		t.Fatalf("%d of %d cases differ from the reference", failures, len(cases))
	}
	t.Logf("%d cases agree with the reference; %d made it throw where this port answers no match",
		len(cases)-crashes, crashes)
}

// at returns the case at an index, or nil when there is none.
func at(cases any, i int) any {
	switch list := cases.(type) {
	case []oracleCase:
		if i >= 0 && i < len(list) {
			return list[i]
		}
	case []hashCase:
		if i >= 0 && i < len(list) {
			return list[i]
		}
	}
	return nil
}

func sameIndices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// matcherCorpus is every case the differential test runs: the reference's
// own test cases, then generated ones that reach the corners of the
// algorithm.
func matcherCorpus() []oracleCase {
	var cases []oracleCase
	add := func(query, compare string) {
		cases = append(cases, oracleCase{Query: query, Compare: compare})
	}

	// The reference's test cases.
	stringsToSearch := searchStrings()
	for _, str := range stringsToSearch {
		for _, query := range []string{"chr", "chrom", "chrome", "cand", "cpywa", "ccs", "Chrome", "sql", "c", "1", "z", "cc"} {
			add(query, str)
		}
	}
	for _, c := range [][2]string{
		{chrome, chrome}, {chrome, lastIsChrome}, {"chro", helpCureHopeRaiseOnMindChrome},
		{"chr", helpCureHopeRaiseOnMindChrome}, {chrome, uninstallOrChangePrograms},
		{chrome, candyCrushSagaFromKing}, {"sql", microsoftSqlServerMgmtStudio},
		{"sql  manag", microsoftSqlServerMgmtStudio},
		{"man", taskManager}, {"man", "eManual"}, {"term", "Windows Terminal"},
		{"term", cygwinTerminal}, {"winterm", "Windows Terminal"}, {"winterm", cygwinTerminal},
		{"red", "red colour"}, {"red", "metro red"}, {"red", "this red colour"},
		{"red", "this colour red"}, {"red", "this colour is very red"},
		{"red", "this colour is surprisingly super awesome red and cool"},
		{"vim", "Vim"}, {"vim", "Vim Diff"}, {"vim", "ignoreDescription"}, {"vim", "ignore.exe"},
		{"goo", "Google Chrome"}, {"chr", "Google Chrome"}, {"wt", windowsTerminalFromMicrosoft},
		{"exce", overLeafLatex}, {"term", windowsTerminalPreview},
		{"sql s managa", microsoftSqlServerMgmtStudio}, {"sql' s manag", microsoftSqlServerMgmtStudio},
		{"servez", microsoftSqlServerMgmtStudio}, {"mic", microsoftSqlServerMgmtStudio},
		{"mssms", microsoftSqlServerMgmtStudio}, {"msms", microsoftSqlServerMgmtStudio},
		{"chr", shutdown}, {"chr", changeSettingsForTextToSpeech},
		{"ch r", changeSettingsForTextToSpeech}, {"a test", thisIsATest},
		{"test", thisIsATest}, {"cod", visualStudioCode}, {"codes", visualStudioCodes},
		{"vsp", visualStudio2019Preview}, {"2019p", visualStudio2019Preview},
		{"camera", "câmera"}, {"câmera", "camera"}, {"cafe", "café"}, {"café", "cafe"},
		{"uber", "über"}, {"uber", "Über"}, {"nino", "niño"},
		{"", ""}, {"", "Chrome"}, {"Chrome", ""}, {"   ", "Chrome"}, {"chrome", "   "},
		{"  chrome  ", "chrome"}, {"chrome", "  chrome  "},
		{"1", oneOneOneOne}, {"11", oneOneOneOne}, {"1111", oneOneOneOne},
		{"vs2019", "VisualStudio2019"}, {"vs2019", "visualStudio2019"},
		{"上海", "上海交通"}, {"shanghai", "上海交通"}, {"jts", "上海交通"},
		{"文件", "我的文件.txt"}, {"file", "我的文件.txt"},
	} {
		add(c[0], c[1])
	}

	// Generated cases: a vocabulary of names that look like what a launcher
	// searches, and queries cut out of them.
	names := []string{
		"Google Chrome", "Visual Studio Code", "Microsoft SQL Server Management Studio",
		"Windows Terminal", "Windows Terminal (Preview)", "Candy Crush Saga from King",
		"JetBrain Rider", "Postman canary", "Postman super Canary", "7-Zip File Manager",
		"VLC media player", "Notepad++", "Microsoft Office Word 2019",
		"Task Manager", "Control Panel", "Add or remove programs",
		"Program Files\\Google\\Chrome\\Application\\chrome.exe",
		"C:\\Users\\crazy\\Documents\\report-2026-10.pdf",
		"Save all open documents and close the editor",
		"user@example.com", "192.168.1.1", "https://github.com/Flow-Launcher/Flow.Launcher",
		"Settings for text-to-speech and speech recognition",
		"a", "A", "aa", "AAA", "aA", "a a", "  spaced  out  words  ",
		"Ünïcödé Strïng", "naïve café résumé", "naive cafe resume",
		"日本語のテキスト", "中文字符串测试", "한국어 텍스트",
		"camelCaseName", "PascalCaseName", "snake_case_name", "kebab-case-name",
		"file.name.with.dots.txt", "2026-10-11T07:31:11Z", "v1.2.3-beta+build.4",
	}

	rng := rand.New(rand.NewSource(20261011))
	for i := 0; i < 900; i++ {
		compare := names[rng.Intn(len(names))]
		runes := []rune(compare)

		var query string
		switch rng.Intn(8) {
		case 0: // a slice of the name
			if len(runes) > 0 {
				start := rng.Intn(len(runes))
				end := start + 1 + rng.Intn(len(runes)-start)
				query = string(runes[start:end])
			}
		case 1: // the initials of its words
			var b strings.Builder
			for _, word := range strings.Fields(compare) {
				if w := []rune(word); len(w) > 0 {
					b.WriteRune(w[0])
				}
			}
			query = b.String()
		case 2: // random letters
			n := 1 + rng.Intn(4)
			var b strings.Builder
			for j := 0; j < n; j++ {
				b.WriteByte(byte('a' + rng.Intn(26)))
			}
			query = b.String()
		case 3: // a slice with a character changed
			if len(runes) > 1 {
				start := rng.Intn(len(runes) - 1)
				end := start + 1 + rng.Intn(len(runes)-start)
				query = string(runes[start:end]) + string(rune('a'+rng.Intn(26)))
			}
		case 4: // a slice with a character removed
			if len(runes) > 2 {
				start := rng.Intn(len(runes) - 1)
				end := start + 1 + rng.Intn(len(runes)-start)
				part := []rune(string(runes[start:end]))
				if len(part) > 1 {
					part = append(part[:0], part[1:]...)
				}
				query = string(part)
			}
		case 5: // the whole name, in another case
			query = strings.ToUpper(compare)
		case 6: // words of the name
			fields := strings.Fields(compare)
			if len(fields) > 0 {
				query = strings.Join(fields[:1+rng.Intn(len(fields))], " ")
			}
		case 7: // digits
			query = fmt.Sprint(rng.Intn(10000))
		}

		precision := []int{0, 20, 50}[rng.Intn(3)]
		ignoreAccents := rng.Intn(4) == 0
		cases = append(cases, oracleCase{
			Query: query, Compare: compare,
			Precision: precision, IgnoreAccents: ignoreAccents,
		})
	}

	return cases
}

// TestWhitespaceOnlyQueryDoesNotCrash records the one place this port
// deliberately differs from the reference: the reference trims the query
// and then indexes the empty list of substrings it leaves, throwing
// IndexOutOfRangeException, where this port reports no match.
func TestWhitespaceOnlyQueryDoesNotCrash(t *testing.T) {
	for _, query := range []string{" ", "   ", "\t", "\n"} {
		if result := matcher().FuzzyMatch(query, "Chrome"); result.Success {
			t.Errorf("match(%q, Chrome) succeeded, want no match", query)
		}
	}
}

// TestRuneIndicesForNonBMPText records a deliberate difference: the
// reference counts its highlight indexes in UTF-16 code units, so an emoji
// before a match shifts them by one; this port counts runes, which is what
// the view highlights.
func TestRuneIndicesForNonBMPText(t *testing.T) {
	m := matcher()
	result := m.FuzzyMatch("chrome", "🙂 Chrome")
	if !result.Success {
		t.Fatal("the query did not match")
	}
	// "🙂 Chrome": the space is rune 1, "Chrome" starts at rune 2.
	if got := result.MatchData; !sameIndices(got, []int{2, 3, 4, 5, 6, 7}) {
		t.Errorf("highlights are %v, want [2 3 4 5 6 7] (the reference would answer [3 4 5 6 7 8])", got)
	}
}

// hashCase is one case of the hash comparison.
type hashCase struct {
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	SubTitle      string `json:"subTitle"`
	RecordKey     string `json:"recordKey"`
	ActionKeyword string `json:"actionKeyword,omitempty"`
	Search        string `json:"search,omitempty"`
}

// TestHashesMatchTheReference checks the hashes that identify a result for
// the counts of what the user picks and for what they pin: a result the
// reference counts and this port does not would stop rising.
func TestHashesMatchTheReference(t *testing.T) {
	exe := oracleExe(t)

	type hashCase struct {
		Title         string `json:"title"`
		SubTitle      string `json:"subTitle"`
		RecordKey     string `json:"recordKey"`
		ActionKeyword string `json:"actionKeyword,omitempty"`
		Search        string `json:"search,omitempty"`
		Kind          string `json:"kind"`
	}

	texts := []string{
		"", "a", "Alpha", "Chrome", "Visual Studio Code", "subtitle of Alpha",
		`C:\Users\crazy\Documents`, "🙂 emoji", "中文标题", "ß", "ßeta",
		"Visual Studio 2019", "  spaces  ", "Mixed CASE", "ümlaut",
	}
	cases := []hashCase{}
	for _, title := range texts {
		for _, subtitle := range texts {
			cases = append(cases, hashCase{Kind: "resultHash", Title: title, SubTitle: subtitle})
			cases = append(cases, hashCase{
				Kind: "resultHash", Title: title, SubTitle: subtitle, RecordKey: subtitle,
			})
			cases = append(cases, hashCase{
				Kind: "queryResultHash", Title: title, SubTitle: subtitle,
				ActionKeyword: "f", Search: "search",
			})
			cases = append(cases, hashCase{
				Kind: "queryResultHash", Title: title, SubTitle: subtitle, RecordKey: "key",
			})
		}
	}

	var in strings.Builder
	for _, c := range cases {
		line, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		in.Write(line)
		in.WriteByte('\n')
	}

	cmd := exec.Command(exe)
	cmd.Stdin = strings.NewReader(in.String())
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

	var answers []struct {
		Hash  int    `json:"hash"`
		Error string `json:"error"`
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var answer struct {
			Hash  int    `json:"hash"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &answer); err != nil {
			t.Fatalf("the oracle answered %q: %v", line, err)
		}
		answers = append(answers, answer)
	}
	if len(answers) != len(cases) {
		t.Fatalf("the oracle answered %d of %d cases", len(answers), len(cases))
	}

	for i, c := range cases {
		if answers[i].Error != "" {
			t.Fatalf("the oracle failed on %+v: %s", c, answers[i].Error)
		}

		result := Result{Title: c.Title, SubTitle: c.SubTitle, RecordKey: c.RecordKey}
		var got int
		if c.Kind == "resultHash" {
			got = ResultHashCode(result)
		} else {
			got = QueryAndResultHashCode(&Query{ActionKeyword: c.ActionKeyword, Search: c.Search}, result)
		}

		if got != answers[i].Hash {
			t.Errorf("%s(%q, %q, key %q) = %d, the reference says %d",
				c.Kind, c.Title, c.SubTitle, c.RecordKey, got, answers[i].Hash)
		}
	}
	t.Logf("%d hashes agree with the reference", len(cases))
}
