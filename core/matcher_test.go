package core

import (
	"sort"
	"testing"
)

// The cases in this file are the reference's own, from
// Flow.Launcher.Test/FuzzyMatcherTest.cs, so that this matcher is checked
// against the scores the reference produces rather than against itself.

const (
	chrome                        = "Chrome"
	candyCrushSagaFromKing        = "Candy Crush Saga from King"
	helpCureHopeRaiseOnMindChrome = "Help cure hope raise on mind entity Chrome"
	uninstallOrChangePrograms     = "Uninstall or change programs on your computer"
	lastIsChrome                  = "Last is chrome"
	microsoftSqlServerMgmtStudio  = "Microsoft SQL Server Management Studio"
	visualStudioCode              = "Visual Studio Code"
	oneOneOneOne                  = "1111"
	changeSettingsForTextToSpeech = "Change settings for text-to-speech and for speech recognition (if installed)."
	programsForActivities         = "Choose which programs you want Windows to use for activities like web browsing, editing photos, sending e-mail, and playing music."
	addChangeAndManageFonts       = "Add, change, and manage fonts on your computer"
	windowsTerminalFromMicrosoft  = "Windows Terminal From Microsoft Store"
	overLeafLatex                 = "OverLeaf-Latex: An online LaTeX editor"
	windowsTerminalPreview        = "Windows Terminal (Preview)"
	visualStudio2019Preview       = "Visual Studio 2019 Preview"
	visualStudio2019              = "Visual Studio 2019"
	visualStudioPreview           = "Visual Studio Preview"
	visualStudio                  = "Visual Studio"
	visualStudioCodes             = "Visual Studio Codes"
	postmanCanary                 = "postman canary"
	postmanSuperCanary            = "Postman super canary"
	postmanSuperCanaryCapital     = "Postman super Canary"
	jetBrainRider                 = "JetBrain Rider"
	shutdown                      = "Shutdown"
	thisIsATest                   = "This is a test"
	taskManager                   = "Task Manager"
	cygwinTerminal                = "Cygwin64 Terminal"
)

// matcher is the reference's matcher with its default settings: the regular
// search precision and accents not ignored.
func matcher() StringMatcher {
	return StringMatcher{UserSettingSearchPrecision: SearchPrecisionRegular}
}

func searchStrings() []string {
	return []string{
		chrome,
		programsForActivities,
		helpCureHopeRaiseOnMindChrome,
		candyCrushSagaFromKing,
		uninstallOrChangePrograms,
		addChangeAndManageFonts,
		lastIsChrome,
		oneOneOneOne,
	}
}

func TestMatchOrder(t *testing.T) {
	sources := []string{
		"file open in browser-test",
		"Install Package",
		"add new bsd",
		"Inste",
		"aac",
	}

	m := matcher()
	type scored struct {
		title string
		score int
	}
	var results []scored
	for _, s := range sources {
		if score := m.FuzzyMatch("inst", s).RawScore; score > 0 {
			results = append(results, scored{s, score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })

	want := []string{"Inste", "Install Package", "file open in browser-test"}
	if len(results) != len(want) {
		t.Fatalf("got %d results %v, want %d", len(results), results, len(want))
	}
	for i, title := range want {
		if results[i].title != title {
			t.Errorf("result %d is %q, want %q (all: %v)", i, results[i].title, title, results)
		}
	}
}

func TestNotAllCharactersFoundScoresZero(t *testing.T) {
	if score := matcher().FuzzyMatch("Chrome", "Can have rum only in my glass").RawScore; score != 0 {
		t.Errorf("score is %d, want 0", score)
	}
}

func TestScores(t *testing.T) {
	cases := []struct {
		query, compare string
		want           int
	}{
		{chrome, chrome, 157},
		{chrome, lastIsChrome, 145},
		{"chro", helpCureHopeRaiseOnMindChrome, 50},
		{"chr", helpCureHopeRaiseOnMindChrome, 30},
		{chrome, uninstallOrChangePrograms, 21},
		{chrome, candyCrushSagaFromKing, 0},
		{"sql", microsoftSqlServerMgmtStudio, 109},
		{"sql  manag", microsoftSqlServerMgmtStudio, 120},
	}
	m := matcher()
	for _, c := range cases {
		if got := m.FuzzyMatch(c.query, c.compare).RawScore; got != c.want {
			t.Errorf("match(%q, %q) = %d, want %d", c.query, c.compare, got, c.want)
		}
	}
}

func TestPrecision(t *testing.T) {
	cases := []struct {
		query, compare string
		precision      SearchPrecisionScore
		want           bool
	}{
		{"goo", "Google Chrome", SearchPrecisionRegular, true},
		{"chr", "Google Chrome", SearchPrecisionLow, true},
		{"chr", chrome, SearchPrecisionRegular, true},
		{"chr", helpCureHopeRaiseOnMindChrome, SearchPrecisionRegular, false},
		{"chr", helpCureHopeRaiseOnMindChrome, SearchPrecisionLow, true},
		{"chr", candyCrushSagaFromKing, SearchPrecisionRegular, false},
		{"chr", candyCrushSagaFromKing, SearchPrecisionNone, true},
		{"ccs", candyCrushSagaFromKing, SearchPrecisionLow, true},
		{"cand", candyCrushSagaFromKing, SearchPrecisionRegular, true},
		{"cand", helpCureHopeRaiseOnMindChrome, SearchPrecisionRegular, false},
		{"vsc", visualStudioCode, SearchPrecisionRegular, true},
		{"vs", visualStudioCode, SearchPrecisionRegular, true},
		{"vc", visualStudioCode, SearchPrecisionRegular, true},
		{"vts", visualStudioCode, SearchPrecisionRegular, false},
		{"vcs", visualStudioCode, SearchPrecisionRegular, false},
		{"wt", windowsTerminalFromMicrosoft, SearchPrecisionRegular, false},
		{"vsp", visualStudio2019Preview, SearchPrecisionRegular, true},
		{"vsp", visualStudio2019, SearchPrecisionRegular, false},
		{"vps", visualStudio, SearchPrecisionRegular, false},
		{"2019p", visualStudio2019Preview, SearchPrecisionRegular, true},
		{"exce", overLeafLatex, SearchPrecisionRegular, false},
		{"term", windowsTerminalPreview, SearchPrecisionRegular, true},
		{"sql s managa", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, false},
		{"sql' s manag", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, false},
		{"sql s manag", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"sql manag", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"sql", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"sql serv", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"servez", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, false},
		{"sql servz", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, false},
		{"sql serv man", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"sql studio", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"mic", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"mssms", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"msms", microsoftSqlServerMgmtStudio, SearchPrecisionRegular, true},
		{"chr", shutdown, SearchPrecisionRegular, false},
		{"chr", changeSettingsForTextToSpeech, SearchPrecisionRegular, false},
		{"ch r", changeSettingsForTextToSpeech, SearchPrecisionRegular, true},
		{"a test", thisIsATest, SearchPrecisionRegular, true},
		{"test", thisIsATest, SearchPrecisionRegular, true},
		{"cod", visualStudioCode, SearchPrecisionRegular, true},
		{"code", visualStudioCode, SearchPrecisionRegular, true},
		{"codes", visualStudioCodes, SearchPrecisionRegular, true},
	}
	for _, c := range cases {
		m := matcher()
		m.UserSettingSearchPrecision = c.precision
		got := m.FuzzyMatch(c.query, c.compare)
		if got.IsSearchPrecisionScoreMet() != c.want {
			t.Errorf("match(%q, %q) at precision %d: raw %d, met %v, want %v",
				c.query, c.compare, c.precision, got.RawScore, got.IsSearchPrecisionScoreMet(), c.want)
		}
	}
}

func TestAcronymScores(t *testing.T) {
	cases := []struct {
		query, compare string
		want           int
	}{
		{"vsc", visualStudioCode, 100},
		{"jbr", jetBrainRider, 100},
		{"jr", jetBrainRider, 66},
		{"vs", visualStudio, 100},
		{"vs", visualStudioPreview, 66},
		{"vsp", visualStudioPreview, 100},
		{"pc", postmanCanary, 100},
		{"psc", postmanSuperCanary, 100},
		{"psc", postmanSuperCanaryCapital, 100},
		{"vsp", visualStudio, 0},
		{"vps", visualStudio, 0},
		{chrome, helpCureHopeRaiseOnMindChrome, 75},
		// A run of digits is one acronym group, however many digits match.
		{"vs2", visualStudio2019, 100},
		{"vs19", visualStudio2019, 100},
		{"vs2019", visualStudio2019, 100},
		{"v29", visualStudio2019, 66},
		{"v2019", visualStudio2019, 66},
		{"v19", visualStudio2019, 66},
		{"vs19", "Visual Studio 2018", 0},
		{"vs19", "Visual Studio 19", 100},
		{"vs2019", "VisualStudio2019", 100},
		{"vs2019", "visualStudio2019", 100},
	}
	m := matcher()
	for _, c := range cases {
		if got := m.FuzzyMatch(c.query, c.compare).Score(); got != c.want {
			t.Errorf("match(%q, %q) = %d, want %d", c.query, c.compare, got, c.want)
		}
	}
}

func TestScoresWeighTheStartOfAWord(t *testing.T) {
	cases := [][3]string{
		{"man", taskManager, "eManual"},
		{"winterm", "Windows Terminal", cygwinTerminal},
	}
	m := matcher()
	for _, c := range cases {
		first := m.FuzzyMatch(c[0], c[1])
		second := m.FuzzyMatch(c[0], c[2])
		if first.Score() <= second.Score() {
			t.Errorf("match(%q, %q) = %d, should be greater than match(%q, %q) = %d",
				c[0], c[1], first.Score(), c[0], c[2], second.Score())
		}
	}
}

// TestTermScoresTheSameInBothStrings records what the reference does with
// the third case of its own test of that name: "term" scores 122 against
// both "Windows Terminal" and "Cygwin64 Terminal", because the digits of
// the latter shift its match by one character and the space by one too,
// leaving the distance the score is built from equal. The reference's test
// still asserts the first is greater, and so fails; the oracle confirms the
// two scores are equal.
func TestTermScoresTheSameInBothStrings(t *testing.T) {
	m := matcher()
	first := m.FuzzyMatch("term", "Windows Terminal")
	second := m.FuzzyMatch("term", cygwinTerminal)
	if first.RawScore != 122 || second.RawScore != 122 {
		t.Errorf("scores are %d and %d, want 122 and 122", first.RawScore, second.RawScore)
	}
}

func TestScoresWeighTheStartOfTheString(t *testing.T) {
	cases := [][3]string{
		{"red", "red colour", "metro red"},
		{"red", "this red colour", "this colour red"},
		{"red", "this red colour", "this colour is very red"},
		{"red", "this red colour", "this colour is surprisingly super awesome red and cool"},
		{"red", "this colour is surprisingly super red very and cool", "this colour is surprisingly super very red and cool"},
	}
	m := matcher()
	for _, c := range cases {
		first := m.FuzzyMatch(c[0], c[1])
		second := m.FuzzyMatch(c[0], c[2])
		if first.Score() <= second.Score() {
			t.Errorf("match(%q, %q) = %d, should be greater than match(%q, %q) = %d",
				c[0], c[1], first.Score(), c[0], c[2], second.Score())
		}
	}
}

func TestExactMatchScoresHighest(t *testing.T) {
	m := matcher()
	best := func(name, description, executable string) int {
		scores := []int{
			m.FuzzyMatch("vim", name).RawScore,
			m.FuzzyMatch("vim", description).RawScore,
			m.FuzzyMatch("vim", executable).RawScore,
		}
		sort.Ints(scores)
		return scores[len(scores)-1]
	}

	first := best("Vim", "ignoreDescription", "ignore.exe")
	second := best("Vim Diff", "ignoreDescription", "ignore.exe")
	if first <= second {
		t.Errorf("the exact match scores %d, should be greater than %d", first, second)
	}
}

func TestIgnoreAccents(t *testing.T) {
	cases := []struct {
		ignoreAccents  bool
		query, compare string
		want           bool
	}{
		{true, "camera", "câmera", true},
		{false, "camera", "câmera", false},
	}
	for _, c := range cases {
		m := matcher()
		m.IgnoreAccents = c.ignoreAccents
		if got := m.FuzzyMatch(c.query, c.compare).IsSearchPrecisionScoreMet(); got != c.want {
			t.Errorf("match(%q, %q) with ignoreAccents=%v is %v, want %v",
				c.query, c.compare, c.ignoreAccents, got, c.want)
		}
	}
}

// TestMatchDataHighlights checks the indexes the view highlights: the
// characters of the compared string the query matched.
func TestMatchDataHighlights(t *testing.T) {
	cases := []struct {
		query, compare string
		want           []int
	}{
		{"chr", chrome, []int{0, 1, 2}},
		{"vsc", visualStudioCode, []int{0, 7, 14}},
		{"sql", microsoftSqlServerMgmtStudio, []int{10, 11, 12}},
	}
	m := matcher()
	for _, c := range cases {
		got := m.FuzzyMatch(c.query, c.compare).MatchData
		if len(got) != len(c.want) {
			t.Errorf("match(%q, %q) highlights %v, want %v", c.query, c.compare, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("match(%q, %q) highlights %v, want %v", c.query, c.compare, got, c.want)
				break
			}
		}
	}
}

func TestEmptyInputsDoNotMatch(t *testing.T) {
	m := matcher()
	if m.FuzzyMatch("", chrome).Success {
		t.Error("an empty query matched")
	}
	if m.FuzzyMatch("chrome", "").Success {
		t.Error("an empty string matched")
	}
}

func TestPrecisionFilterZeroesTheScore(t *testing.T) {
	m := matcher()
	// "chr" against the long help string scores 30, below the regular 50.
	result := m.FuzzyMatch("chr", helpCureHopeRaiseOnMindChrome)
	if result.RawScore != 30 {
		t.Fatalf("raw score is %d, want 30", result.RawScore)
	}
	if result.Score() != 0 {
		t.Errorf("score is %d, want 0 below the precision", result.Score())
	}
}
