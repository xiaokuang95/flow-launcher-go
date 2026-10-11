package calculator

import (
	"math"
	"strconv"
	"strings"
)

// significantDigits is how many digits a value keeps when it is written as
// a decimal number, which is what the reference does before rounding it to
// a fixed number of places.
const significantDigits = 15

// FormatNumber writes a value the way the reference writes one: rounded to
// at most the given number of decimal places, with the digits grouped when
// they are long, and with the marks the language uses.
//
// The places shown are the ones the value carries, up to the limit: 2 is
// written "2" rather than "2.0000000000", while an answer that is almost
// but not quite zero keeps the ten places, so that it shows as
// 0.0000000000 rather than as 0.
func FormatNumber(value float64, places int, decimal, group string, useGrouping bool) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", errf("the answer is not a number")
	}
	if places < 0 {
		places = 0
	}

	digits, exponent, negative := decimalDigits(value)
	if digits == "" {
		return "0", nil
	}

	// The value is digits × 10^(exponent-14), so the digits start at the
	// place exponent+1.
	pointPosition := exponent + 1

	scale := len(digits) - pointPosition
	if scale < 0 {
		scale = 0
	}
	shown := scale
	if shown > places {
		shown = places
	}

	whole, fraction := roundTo(digits, pointPosition, shown)

	if useGrouping && len(whole) > 3 {
		whole = groupDigits(whole, group)
	}

	result := whole
	if shown > 0 {
		result += decimal + fraction
	}
	if negative && strings.Trim(whole+fraction, "0") != "" {
		result = "-" + result
	}
	return result, nil
}

// decimalDigits writes a value as the digits of a decimal number and the
// place its first digit sits at: the value is digits × 10^(exponent-14),
// where the digits are the significant ones with their trailing zeros
// dropped. The reference gets the same digits by turning the value into a
// decimal number, which keeps fifteen significant digits.
func decimalDigits(value float64) (digits string, exponent int, negative bool) {
	if value == 0 {
		return "", 0, false
	}

	text := strconv.FormatFloat(value, 'e', significantDigits-1, 64)
	if strings.HasPrefix(text, "-") {
		negative = true
		text = text[1:]
	}

	mantissa, exponentText, _ := strings.Cut(text, "e")
	digits = strings.Replace(mantissa, ".", "", 1)
	digits = strings.TrimRight(digits, "0")

	parsed, err := strconv.Atoi(exponentText)
	if err != nil {
		parsed = 0
	}
	return digits, parsed, negative
}

// roundTo writes the digits as a whole part and a fraction of the length
// asked for, rounding the last place away from zero.
func roundTo(digits string, pointPosition, fractionLength int) (whole, fraction string) {
	// Lay the digits out around the point, padding with zeros on both
	// sides so that every place that is written has a digit.
	var before, after string
	switch {
	case pointPosition <= 0:
		before = ""
		after = strings.Repeat("0", -pointPosition) + digits
	case pointPosition >= len(digits):
		before = digits + strings.Repeat("0", pointPosition-len(digits))
		after = ""
	default:
		before = digits[:pointPosition]
		after = digits[pointPosition:]
	}

	if before == "" {
		before = "0"
	}

	// The digit after the fraction decides whether it rounds up.
	roundUp := false
	if len(after) > fractionLength {
		roundUp = after[fractionLength] >= '5'
	}
	if len(after) < fractionLength {
		after += strings.Repeat("0", fractionLength-len(after))
	}
	after = after[:fractionLength]

	if roundUp {
		before, after = carry(before, after)
	}
	if fractionLength == 0 {
		after = ""
	}
	return before, after
}

// carry adds one at the last written place.
func carry(whole, fraction string) (string, string) {
	digits := []byte(whole + fraction)
	for i := len(digits) - 1; i >= 0; i-- {
		if digits[i] < '9' {
			digits[i]++
			break
		}
		digits[i] = '0'
		if i == 0 {
			// The number grew by a digit, which only matters when the
			// whole part grew.
			digits = append([]byte{'1'}, digits...)
			return string(digits[:len(whole)+1]), string(digits[len(whole)+1:])
		}
	}
	return string(digits[:len(whole)]), string(digits[len(whole):])
}

// groupDigits puts the mark between each group of three digits.
func groupDigits(whole, group string) string {
	if group == "" {
		return whole
	}
	var groups []string
	for len(whole) > 3 {
		groups = append([]string{whole[len(whole)-3:]}, groups...)
		whole = whole[:len(whole)-3]
	}
	return strings.Join(append([]string{whole}, groups...), group)
}
