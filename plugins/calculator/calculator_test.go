package calculator

import (
	"math"
	"testing"

	"launch/tools/oracle"
)

// The cases in this file are the reference's own, from
// Flow.Launcher.Test/Plugins/CalculatorTest.cs, with the settings that test
// uses: ten decimal places, no grouping, a dot for the decimal mark.

func calculate(t *testing.T, expression string) string {
	t.Helper()
	value, err := Evaluate(expression)
	if err != nil {
		return ""
	}
	text, err := FormatNumber(value, 10, ".", ",", false)
	if err != nil {
		return ""
	}
	return text
}

func TestCalculator(t *testing.T) {
	cases := []struct {
		expression string
		want       string
	}{
		{"1+1", "2"},
		{"2-1", "1"},
		{"2*2", "4"},
		{"4/2", "2"},
		{"2^3", "8"},
		{"10/3", "3.3333333333"},
		{"(1+2)*3", "9"},
		{"2^(1+2)", "8"},
		{"pow(2,3)", "8"},
		{"min(1,-1,-2)", "-2"},
		{"max(1,-1,-2)", "1"},
		{"sqrt(16)", "4"},
		{"sin(pi)", "0.0000000000"},
		{"cos(0)", "1"},
		{"tan(0)", "0"},
		{"log10(100)", "2"},
		{"log(100)", "2"},
		{"log2(8)", "3"},
		{"ln(e)", "1"},
		{"abs(-5)", "5"},
		{"pi", "3.1415926536"},
		{"(2+3)*sqrt(16)-log(100)/ln(e)", "18"},
		{"sin(pi/2)+cos(0)+tan(0)", "2"},
		{"10/0", ""},
		{"sqrt(-1)", ""},
		{"log(0)", ""},
		{"invalid_expression", ""},
	}

	for _, c := range cases {
		if got := calculate(t, c.expression); got != c.want {
			t.Errorf("%s = %q, want %q", c.expression, got, c.want)
		}
	}
}

func TestThousandsSeparator(t *testing.T) {
	value, err := Evaluate("1000+234")
	if err != nil {
		t.Fatal(err)
	}

	// With a dot for the decimal mark, the group mark is a comma.
	if got, _ := FormatNumber(value, 10, ".", ",", true); got != "1,234" {
		t.Errorf("with a dot for decimals the answer is %q, want 1,234", got)
	}
	// With a comma, it is a dot.
	if got, _ := FormatNumber(value, 10, ",", ".", true); got != "1.234" {
		t.Errorf("with a comma for decimals the answer is %q, want 1.234", got)
	}
	// Turned off, the digits run together.
	if got, _ := FormatNumber(value, 10, ".", ",", false); got != "1234" {
		t.Errorf("with grouping off the answer is %q, want 1234", got)
	}
}

func TestLargeNumberWithoutGrouping(t *testing.T) {
	value, err := Evaluate("1000000+234567")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := FormatNumber(value, 10, ".", ",", false); got != "1234567" {
		t.Errorf("the answer is %q, want 1234567", got)
	}
}

func TestSeparatorsFollowTheLanguage(t *testing.T) {
	settings := DefaultSettings()
	value, err := Evaluate("1000+234")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		language string
		want     string
	}{
		{"en", "1,234"},
		{"de", "1.234"},
		{"fr", "1\u202f234"},
		{"ru", "1\u00a0234"},
		{"zh-cn", "1,234"},
	}
	for _, c := range cases {
		decimal, group := settings.separators(c.language)
		got, err := FormatNumber(value, 10, decimal, group, true)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("in %s the answer is %q, want %q", c.language, got, c.want)
		}
	}
}

func TestNumberShowsOnlyThePlacesItCarries(t *testing.T) {
	cases := []struct {
		expression string
		want       string
	}{
		{"2", "2"},
		{"0.5", "0.5"},
		{"1.005", "1.005"},
		{"0.1234567891", "0.1234567891"},
		{"1234567.891234567", "1234567.89123457"},
		{"0.00000000001", "0.0000000000"},
		{"5!", "120"},
		{"0xff", "255"},
	}
	for _, c := range cases {
		if got := calculate(t, c.expression); got != c.want {
			t.Errorf("%s = %q, want %q", c.expression, got, c.want)
		}
	}
}

// TestFormattingMatchesTheReference checks the way an answer is written
// against the way .NET writes the same value, over values that reach the
// corners of the rounding.
func TestFormattingMatchesTheReference(t *testing.T) {
	values := []float64{
		0, 1, 2, 0.5, 1.005, 10.0 / 3, math.Pi, math.E,
		1.2246467991473532e-16, -1.2246467991473532e-16,
		1234567.891234567, 1e20, -1e20, 1e-20, 0.1234567891,
		9999999999.999999, 1.9999999999999998, -0.5, 123456.789,
		1e15, 1e16, 0.1 + 0.2, 2.5e-7, -3.14159265358979,
		9.99999999995, 0.00000000005, 1000000.0000001,
		1.0 / 3, 2.0 / 3, 1e-10, -1e-10, 123.456789012345,
	}

	var cases []any
	var kept []float64
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		cases = append(cases, map[string]any{"kind": "decimal", "value": value, "places": 10})
		kept = append(kept, value)
	}

	answers := oracle.Ask(t, cases...)
	for i, value := range kept {
		got, err := FormatNumber(value, 10, ".", ",", false)
		if err != nil {
			t.Errorf("%v: %v", value, err)
			continue
		}
		if want := oracle.String(answers[i], "text"); got != want {
			t.Errorf("FormatNumber(%v) = %q, the reference writes %q", value, got, want)
		}
	}
}

// TestExpressionsMatchTheReferenceEngine checks the arithmetic against the
// engine the reference works it out with, over the expressions a launcher
// is likely to be given.
//
// The engine's own log, ln and pow are the wrong way round or broken, and
// the reference rewrites those three before handing an expression over;
// this port answers them the way that rewriting makes the reference answer,
// which its own tests check, so they are left out here.
func TestExpressionsMatchTheReferenceEngine(t *testing.T) {
	expressions := []string{
		"1+1", "2-1", "2*2", "4/2", "2^3", "2^10", "10/3", "(1+2)*3", "2^(1+2)",
		"min(1,-1,-2)", "max(1,-1,-2)", "sqrt(16)",
		"sqrt(2)", "cos(0)", "tan(0)", "log10(100)", "log2(8)",
		"abs(-5)", "pi", "sin(pi/2)+cos(0)+tan(0)", "0xff", "0x10", "5!",
		"3!", "2+3*4", "(2+3)*4", "100-50-25", "2^3^2", "-2^2", "-(2^2)",
		"1.5+2.5", "0.1+0.2", "1/3", "2/3", "1e3", "1.5e2", "100%7",
		"min(1,2,3)", "max(1,2,3)", "sqrt(123)", "cos(123)", "exp(1)",
		"round(2.5)", "ceil(2.1)", "floor(2.9)", "sign(-5)",
		"lerp(0,10,0.5)", "abs(-1)+abs(1)", "e", "2(3+4)", "2.5!",
		"-2^-3", "0!", "3!!",
	}

	cases := make([]any, 0, len(expressions))
	for _, expression := range expressions {
		cases = append(cases, map[string]any{"kind": "mages", "expression": expression})
	}

	answers := oracle.Ask(t, cases...)
	for i, expression := range expressions {
		if oracle.Error(answers[i]) != "" {
			continue
		}
		// Only the answers that are a number can be shown; the engine
		// answers with a matrix or nothing at all for the rest.
		if oracle.String(answers[i], "type") != "Double" {
			continue
		}
		want := oracle.Number(answers[i], "text")

		got, err := Evaluate(expression)
		if err != nil {
			t.Errorf("%s: %v, the engine answers %s", expression, err, oracle.String(answers[i], "text"))
			continue
		}
		if math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
			t.Errorf("%s = %v, the engine answers %s", expression, got, oracle.String(answers[i], "text"))
		}
	}
}
