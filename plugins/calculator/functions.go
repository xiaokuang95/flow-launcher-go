package calculator

import (
	"math"
	"strings"
)

// call works out a function of the engine's, with the meanings it gives
// them.
func call(name string, args []float64) (value, error) {
	switch strings.ToLower(name) {
	case "abs":
		return unary(name, args, math.Abs)
	case "sign":
		return unary(name, args, func(v float64) float64 {
			switch {
			case v > 0:
				return 1
			case v < 0:
				return -1
			default:
				return 0
			}
		})
	case "ceil":
		return unary(name, args, math.Ceil)
	case "floor":
		return unary(name, args, math.Floor)
	case "round":
		return unary(name, args, func(v float64) float64 { return math.RoundToEven(v) })
	case "exp":
		return unary(name, args, math.Exp)
	case "sqrt":
		return unary(name, args, math.Sqrt)
	case "sin":
		return unary(name, args, math.Sin)
	case "cos":
		return unary(name, args, math.Cos)
	case "tan":
		return unary(name, args, math.Tan)
	case "arcsin", "asin":
		return unary(name, args, math.Asin)
	case "arccos", "acos":
		return unary(name, args, math.Acos)
	case "arctan", "atan":
		return unary(name, args, math.Atan)
	case "isnan":
		return predicate(name, args, math.IsNaN)
	case "isint":
		return predicate(name, args, func(v float64) bool {
			return !math.IsNaN(v) && !math.IsInf(v, 0) && v == math.Trunc(v)
		})
	case "isinfty":
		return predicate(name, args, func(v float64) bool { return math.IsInf(v, 0) })
	case "isprime":
		return predicate(name, args, isPrime)

	case "log":
		// The reference's engine has log and ln the other way round and the
		// plugin swaps them, so that what people type means what they
		// expect: log is the ten-based one and ln the natural one.
		return unary(name, args, math.Log10)
	case "ln":
		return unary(name, args, math.Log)
	case "log2":
		return unary(name, args, math.Log2)
	case "log10":
		return unary(name, args, math.Log10)

	case "min":
		return fold(name, args, math.Min)
	case "max":
		return fold(name, args, math.Max)
	case "clamp":
		// The engine clamps only against the lower bound, which leaves a
		// value above the upper one alone. This clamps against both, which
		// is what the name says.
		if len(args) != 3 {
			return value{}, errf("clamp takes three arguments")
		}
		return numberValue(math.Min(math.Max(args[0], args[1]), args[2])), nil
	case "lerp":
		if len(args) != 3 {
			return value{}, errf("lerp takes three arguments")
		}
		return numberValue(args[0] + (args[1]-args[0])*args[2]), nil

	case "pow":
		// The engine's pow is broken and the reference works around it by
		// writing the power out, which is what this does.
		if len(args) != 2 {
			return value{}, errf("pow takes two arguments")
		}
		return numberValue(math.Pow(args[0], args[1])), nil
	case "factorial":
		if len(args) != 1 {
			return value{}, errf("factorial takes one argument")
		}
		return factorial(args[0])

	case "lt":
		return compare(name, args, func(a, b float64) bool { return a < b })
	case "gt":
		return compare(name, args, func(a, b float64) bool { return a > b })
	case "eq":
		return compare(name, args, func(a, b float64) bool { return a == b })

	default:
		return value{}, errf("%q is not a function", name)
	}
}

func unary(name string, args []float64, fn func(float64) float64) (value, error) {
	if len(args) != 1 {
		return value{}, errf("%s takes one argument", name)
	}
	return numberValue(fn(args[0])), nil
}

// predicate works out a function whose answer is yes or no, which is not
// something the calculator shows.
func predicate(name string, args []float64, holds func(float64) bool) (value, error) {
	if len(args) != 1 {
		return value{}, errf("%s takes one argument", name)
	}
	return boolValue(holds(args[0])), nil
}

func fold(name string, args []float64, fn func(a, b float64) float64) (value, error) {
	if len(args) == 0 {
		return value{}, errf("%s takes at least one argument", name)
	}
	result := args[0]
	for _, argument := range args[1:] {
		result = fn(result, argument)
	}
	return numberValue(result), nil
}

func compare(name string, args []float64, holds func(a, b float64) bool) (value, error) {
	if len(args) != 2 {
		return value{}, errf("%s takes two arguments", name)
	}
	return boolValue(holds(args[0], args[1])), nil
}

// isPrime reports whether a whole number is prime.
func isPrime(value float64) bool {
	if value < 2 || value != math.Trunc(value) {
		return false
	}
	for divisor := 2.0; divisor*divisor <= value; divisor++ {
		if math.Mod(value, divisor) == 0 {
			return false
		}
	}
	return true
}
