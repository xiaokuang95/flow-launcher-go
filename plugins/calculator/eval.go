package calculator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// The expression evaluator. It answers the arithmetic people type into a
// launcher, with the meanings the reference's engine gives them: ^ raises to
// a power, ! is a factorial, 0x starts a hexadecimal number, a value beside
// another multiplies, and the functions are the ones the reference
// documents.

// evalError is an expression that cannot be worked out, which the plugin
// answers with no result at all.
type evalError struct{ message string }

func (e evalError) Error() string { return e.message }

func errf(format string, args ...any) error { return evalError{fmt.Sprintf(format, args...)} }

// Evaluate works out an arithmetic expression, and answers a number. An
// expression that works out to something other than a number, a comparison
// for instance, has no answer to show.
func Evaluate(expression string) (float64, error) {
	parser := &parser{tokens: tokenize(expression)}
	result, err := parser.parseExpression()
	if err != nil {
		return 0, err
	}
	if parser.peek().kind != tokenEnd {
		return 0, errf("unexpected %q", parser.peek().text)
	}
	if result.isBool {
		return 0, errf("the answer is not a number")
	}
	return result.number, nil
}

// value is what an expression works out to: a number, or the answer to a
// comparison, which is not shown.
type value struct {
	number float64
	// isBool marks the value as true or false rather than a number. The
	// arithmetic treats one as the other, but a comparison's own answer is
	// not something to show as a sum.
	isBool bool
	truth  bool
}

func numberValue(number float64) value { return value{number: number} }

func boolValue(holds bool) value {
	number := 0.0
	if holds {
		number = 1
	}
	return value{number: number, isBool: true, truth: holds}
}

// asNumber reads a value as a number, which a comparison is too.
func (v value) asNumber() float64 { return v.number }

// --- Tokens ---

type tokenKind int

const (
	tokenEnd tokenKind = iota
	tokenNumber
	tokenName
	tokenOperator
	tokenLeftParen
	tokenRightParen
	tokenComma
)

type token struct {
	kind tokenKind
	text string
}

func tokenize(input string) []token {
	var tokens []token
	runes := []rune(input)

	for i := 0; i < len(runes); {
		r := runes[i]

		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			i++
		case isDigit(r) || r == '.' && i+1 < len(runes) && isDigit(runes[i+1]):
			start := i
			// A hexadecimal number starts with 0x.
			if r == '0' && i+1 < len(runes) && (runes[i+1] == 'x' || runes[i+1] == 'X') {
				i += 2
				for i < len(runes) && isHexDigit(runes[i]) {
					i++
				}
				tokens = append(tokens, token{kind: tokenNumber, text: string(runes[start:i])})
				continue
			}
			for i < len(runes) && (isDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			// An exponent belongs to the number: 1e3, 2.5E-4.
			if i < len(runes) && (runes[i] == 'e' || runes[i] == 'E') {
				lookahead := i + 1
				if lookahead < len(runes) && (runes[lookahead] == '+' || runes[lookahead] == '-') {
					lookahead++
				}
				if lookahead < len(runes) && isDigit(runes[lookahead]) {
					i = lookahead
					for i < len(runes) && isDigit(runes[i]) {
						i++
					}
				}
			}
			tokens = append(tokens, token{kind: tokenNumber, text: string(runes[start:i])})
		case isNameStart(r):
			start := i
			for i < len(runes) && isNameRune(runes[i]) {
				i++
			}
			tokens = append(tokens, token{kind: tokenName, text: string(runes[start:i])})
		case r == '(':
			tokens = append(tokens, token{kind: tokenLeftParen, text: "("})
			i++
		case r == ')':
			tokens = append(tokens, token{kind: tokenRightParen, text: ")"})
			i++
		case r == ',':
			tokens = append(tokens, token{kind: tokenComma, text: ","})
			i++
		default:
			// The two-character operators first, so that <= is not read as
			// < and =.
			if i+1 < len(runes) && isTwoCharOperator(string(runes[i:i+2])) {
				tokens = append(tokens, token{kind: tokenOperator, text: string(runes[i : i+2])})
				i += 2
				continue
			}
			tokens = append(tokens, token{kind: tokenOperator, text: string(r)})
			i++
		}
	}

	tokens = append(tokens, token{kind: tokenEnd})
	return tokens
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isHexDigit(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isNameStart(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isNameRune(r rune) bool { return isNameStart(r) || isDigit(r) }

func isTwoCharOperator(text string) bool {
	switch text {
	case "<=", ">=", "==", "!=", "&&", "||":
		return true
	default:
		return false
	}
}

// --- Parsing ---

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token { return p.tokens[p.pos] }

func (p *parser) next() token {
	token := p.tokens[p.pos]
	if token.kind != tokenEnd {
		p.pos++
	}
	return token
}

func (p *parser) acceptOperator(text string) bool {
	if token := p.peek(); token.kind == tokenOperator && token.text == text {
		p.pos++
		return true
	}
	return false
}

// parseExpression is a conditional, the loosest of the operators.
func (p *parser) parseExpression() (value, error) {
	condition, err := p.parseOr()
	if err != nil {
		return value{}, err
	}
	if !p.acceptOperator("?") {
		return condition, nil
	}

	yes, err := p.parseExpression()
	if err != nil {
		return value{}, err
	}
	if !p.acceptOperator(":") {
		return value{}, errf("the conditional has no else")
	}
	no, err := p.parseExpression()
	if err != nil {
		return value{}, err
	}
	if condition.asNumber() != 0 {
		return yes, nil
	}
	return no, nil
}

func (p *parser) parseOr() (value, error) {
	left, err := p.parseAnd()
	if err != nil {
		return value{}, err
	}
	for p.acceptOperator("||") {
		right, err := p.parseAnd()
		if err != nil {
			return value{}, err
		}
		left = boolValue(left.asNumber() != 0 || right.asNumber() != 0)
	}
	return left, nil
}

func (p *parser) parseAnd() (value, error) {
	left, err := p.parseComparison()
	if err != nil {
		return value{}, err
	}
	for p.acceptOperator("&&") {
		right, err := p.parseComparison()
		if err != nil {
			return value{}, err
		}
		left = boolValue(left.asNumber() != 0 && right.asNumber() != 0)
	}
	return left, nil
}

func (p *parser) parseComparison() (value, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return value{}, err
	}

	for {
		token := p.peek()
		if token.kind != tokenOperator {
			return left, nil
		}

		var compare func(a, b float64) bool
		switch token.text {
		case "<":
			compare = func(a, b float64) bool { return a < b }
		case ">":
			compare = func(a, b float64) bool { return a > b }
		case "<=":
			compare = func(a, b float64) bool { return a <= b }
		case ">=":
			compare = func(a, b float64) bool { return a >= b }
		case "==":
			compare = func(a, b float64) bool { return a == b }
		case "!=":
			compare = func(a, b float64) bool { return a != b }
		default:
			return left, nil
		}
		p.pos++

		right, err := p.parseAdditive()
		if err != nil {
			return value{}, err
		}
		left = boolValue(compare(left.asNumber(), right.asNumber()))
	}
}

func (p *parser) parseAdditive() (value, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return value{}, err
	}
	for {
		switch {
		case p.acceptOperator("+"):
			right, err := p.parseMultiplicative()
			if err != nil {
				return value{}, err
			}
			left = numberValue(left.asNumber() + right.asNumber())
		case p.acceptOperator("-"):
			right, err := p.parseMultiplicative()
			if err != nil {
				return value{}, err
			}
			left = numberValue(left.asNumber() - right.asNumber())
		default:
			return left, nil
		}
	}
}

func (p *parser) parseMultiplicative() (value, error) {
	left, err := p.parseUnary()
	if err != nil {
		return value{}, err
	}
	for {
		switch {
		case p.acceptOperator("*"):
			right, err := p.parseUnary()
			if err != nil {
				return value{}, err
			}
			left = numberValue(left.asNumber() * right.asNumber())
		case p.acceptOperator("/"):
			right, err := p.parseUnary()
			if err != nil {
				return value{}, err
			}
			left = numberValue(left.asNumber() / right.asNumber())
		case p.acceptOperator("%"):
			right, err := p.parseUnary()
			if err != nil {
				return value{}, err
			}
			left = numberValue(math.Mod(left.asNumber(), right.asNumber()))
		case startsPrimary(p.peek()):
			// A value written beside another multiplies by it, so that
			// 2(3+4) is 14.
			right, err := p.parseUnary()
			if err != nil {
				return value{}, err
			}
			left = numberValue(left.asNumber() * right.asNumber())
		default:
			return left, nil
		}
	}
}

// startsPrimary reports whether a token can begin a value, which is what
// makes a value written beside another a multiplication.
func startsPrimary(token token) bool {
	switch token.kind {
	case tokenNumber, tokenLeftParen, tokenName:
		return true
	default:
		return false
	}
}

// parseUnary reads a value with a power over it. A sign belongs to the
// value it is written against, before the power is worked out, so that
// -2^2 is (-2)^2, which is how the reference's engine reads it.
func (p *parser) parseUnary() (value, error) {
	return p.parsePower()
}

// parsePower reads a power, which groups to the right, so that 2^3^2 is
// 2^(3^2).
func (p *parser) parsePower() (value, error) {
	base, err := p.parseSignedPostfix()
	if err != nil {
		return value{}, err
	}
	if p.acceptOperator("^") {
		exponent, err := p.parseUnary()
		if err != nil {
			return value{}, err
		}
		return numberValue(math.Pow(base.asNumber(), exponent.asNumber())), nil
	}
	return base, nil
}

// parseSignedPostfix reads a sign written against a value.
func (p *parser) parseSignedPostfix() (value, error) {
	switch {
	case p.acceptOperator("-"):
		operand, err := p.parseSignedPostfix()
		if err != nil {
			return value{}, err
		}
		return numberValue(-operand.asNumber()), nil
	case p.acceptOperator("+"):
		return p.parseSignedPostfix()
	}
	return p.parsePostfix()
}

// parsePostfix reads a factorial after a value.
func (p *parser) parsePostfix() (value, error) {
	result, err := p.parsePrimary()
	if err != nil {
		return value{}, err
	}
	for p.acceptOperator("!") {
		result, err = factorial(result.asNumber())
		if err != nil {
			return value{}, err
		}
	}
	return result, nil
}

func (p *parser) parsePrimary() (value, error) {
	token := p.next()

	switch token.kind {
	case tokenNumber:
		return parseNumber(token.text)

	case tokenLeftParen:
		inner, err := p.parseExpression()
		if err != nil {
			return value{}, err
		}
		if next := p.next(); next.kind != tokenRightParen {
			return value{}, errf("a parenthesis was not closed")
		}
		return inner, nil

	case tokenName:
		// A name is a constant, or a function when a parenthesis follows.
		if p.peek().kind == tokenLeftParen {
			return p.parseCall(token.text)
		}
		return constant(token.text)
	}

	if token.kind == tokenEnd {
		return value{}, errf("the expression is not complete")
	}
	return value{}, errf("unexpected %q", token.text)
}

// parseCall reads the arguments of a function and works it out.
func (p *parser) parseCall(name string) (value, error) {
	if next := p.next(); next.kind != tokenLeftParen {
		return value{}, errf("expected a parenthesis after %s", name)
	}

	var args []float64
	if p.peek().kind != tokenRightParen {
		for {
			argument, err := p.parseExpression()
			if err != nil {
				return value{}, err
			}
			args = append(args, argument.asNumber())

			next := p.next()
			if next.kind == tokenRightParen {
				break
			}
			if next.kind != tokenComma {
				return value{}, errf("expected a comma in %s(...)", name)
			}
		}
	} else {
		p.next()
	}

	return call(name, args)
}

// parseNumber reads a number, in decimal or hexadecimal.
func parseNumber(text string) (value, error) {
	if len(text) > 2 && text[0] == '0' && (text[1] == 'x' || text[1] == 'X') {
		parsed, err := strconv.ParseUint(text[2:], 16, 64)
		if err != nil {
			return value{}, errf("%q is not a number", text)
		}
		return numberValue(float64(parsed)), nil
	}

	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return value{}, errf("%q is not a number", text)
	}
	return numberValue(parsed), nil
}

// factorial works out n!, and the gamma function for a value that is not a
// whole number, as the engine does.
func factorial(n float64) (value, error) {
	if n < 0 {
		return value{}, errf("a factorial needs a value that is not negative")
	}
	if n == math.Trunc(n) {
		result := 1.0
		for i := 2.0; i <= n; i++ {
			result *= i
		}
		return numberValue(result), nil
	}
	return numberValue(math.Gamma(n + 1)), nil
}

// constant is the value of a name used on its own.
func constant(name string) (value, error) {
	switch strings.ToLower(name) {
	case "pi", "π":
		return numberValue(math.Pi), nil
	case "e":
		return numberValue(math.E), nil
	case "true":
		return boolValue(true), nil
	case "false":
		return boolValue(false), nil
	case "nan":
		return numberValue(math.NaN()), nil
	case "infinity", "inf":
		return numberValue(math.Inf(1)), nil
	default:
		return value{}, errf("%q is not a value", name)
	}
}
