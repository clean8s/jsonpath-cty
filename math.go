package jsonpathcty

import (
	"math"
	"strings"
	"github.com/zclconf/go-cty/cty"
	"regexp"
	"fmt"
)

// Function - internal left function of JSONPath
type Function func(node cty.Value) (result cty.Value, err error)

// Operation - internal script operation of JSONPath
type Operation func(left cty.Value, right cty.Value) (result cty.Value, err error)

var (
	// Operator precedence
	// From https://golang.org/ref/spec#Operator_precedence
	//
	//	Precedence    Operator
	//	    5             *  /  %  <<  >>  &  &^
	//	    4             +  -  |  ^
	//	    3             ==  !=  <  <=  >  >= =~
	//	    2             &&
	//	    1             ||
	//
	// Arithmetic operators
	// From https://golang.org/ref/spec#Arithmetic_operators
	//
	//	+    sum                    integers, floats, complex values, strings
	//	-    difference             integers, floats, complex values
	//	*    product                integers, floats, complex values
	//	/    quotient               integers, floats, complex values
	//	%    remainder              integers
	//
	//	&    bitwise AND            integers
	//	|    bitwise OR             integers
	//	^    bitwise XOR            integers
	//	&^   bit clear (AND NOT)    integers
	//
	//	<<   left shift             integer << unsigned integer
	//	>>   right shift            integer >> unsigned integer
	//
	//	==  equals                  any
	//	!=  not equals              any
	//	<   less                    any
	//	<=  less or equals          any
	//	>   larger                  any
	//	>=  larger or equals        any
	//	=~  equals regex string     strings
	//
	priority = map[string]uint8{
		"**": 6, // additional: power
		"*":  5,
		"/":  5,
		"%":  5,
		"<<": 5,
		">>": 5,
		"&":  5,
		"&^": 5,
		"+":  4,
		"-":  4,
		"|":  4,
		"^":  4,
		"==": 3,
		"!=": 3,
		"<":  3,
		"<=": 3,
		">":  3,
		">=": 3,
		"=~": 3,
		"&&": 2,
		"||": 1,
	}
	priorityChar = map[byte]bool{
		'*': true,
		'/': true,
		'%': true,
		'<': true,
		'>': true,
		'&': true,
		'|': true,
		'^': true,
		'+': true,
		'-': true,
		'=': true,
		'!': true,
	}

	rightOp = map[string]bool{
		"**": true,
	}

	validPrimitives = func (left, right cty.Value) bool {
		if left.IsNull() || right.IsNull() { return false }
		if !left.Type().IsPrimitiveType() || !right.Type().IsPrimitiveType() {
			return false
		}
		return true
	}

	operations = map[string]Operation{
		"*": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.Multiply(right), nil
		},
		"/": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.Divide(right), nil
		},
		"%": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.Modulo(right), nil
		},
		"+": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.Add(right), nil
		},
		"-": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.Subtract(right), nil
		},
		"==": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			return left.Equals(right), nil
		},
		"!=": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			return left.NotEqual(right), nil
		},
		"=~": func(left cty.Value, right cty.Value) (node cty.Value, err error) {
			pattern := right.AsString()
			val := left.AsString()
			res, err := regexp.MatchString(pattern, val)
			if err != nil {
				return cty.NilVal, err
			}
			return cty.BoolVal(res), nil
		},
		"<": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.LessThan(right), nil
		},
		"<=": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.LessThanOrEqualTo(right), nil
		},
		">": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.GreaterThan(right), nil
		},
		">=": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			if !validPrimitives(left, right) {
				return cty.NilVal, errorRequest("Operation on invalid values %v, %v", left, right)
			}
			return left.GreaterThanOrEqualTo(right), nil
		},
		"&&": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			return left.And(right), nil
		},
		"||": func(left cty.Value, right cty.Value) (result cty.Value, err error) {
			return left.Or(right), nil
		},
	}

	functions = map[string]Function{
		"abs":         numericFunction("Abs", math.Abs),
		"acos":        numericFunction("Acos", math.Acos),
		"acosh":       numericFunction("Acosh", math.Acosh),
		"asin":        numericFunction("Asin", math.Asin),
		"asinh":       numericFunction("Asinh", math.Asinh),
		"atan":        numericFunction("Atan", math.Atan),
		"atanh":       numericFunction("Atanh", math.Atanh),
		"cbrt":        numericFunction("Cbrt", math.Cbrt),
		"ceil":        numericFunction("Ceil", math.Ceil),
		"cos":         numericFunction("Cos", math.Cos),
		"cosh":        numericFunction("Cosh", math.Cosh),
		"erf":         numericFunction("Erf", math.Erf),
		"erfc":        numericFunction("Erfc", math.Erfc),
		"erfcinv":     numericFunction("Erfcinv", math.Erfcinv),
		"erfinv":      numericFunction("Erfinv", math.Erfinv),
		"exp":         numericFunction("Exp", math.Exp),
		"exp2":        numericFunction("Exp2", math.Exp2),
		"expm1":       numericFunction("Expm1", math.Expm1),
		"floor":       numericFunction("Floor", math.Floor),
		"gamma":       numericFunction("Gamma", math.Gamma),
		"j0":          numericFunction("J0", math.J0),
		"j1":          numericFunction("J1", math.J1),
		"log":         numericFunction("Log", math.Log),
		"log10":       numericFunction("Log10", math.Log10),
		"log1p":       numericFunction("Log1p", math.Log1p),
		"log2":        numericFunction("Log2", math.Log2),
		"logb":        numericFunction("Logb", math.Logb),
		"round":       numericFunction("Round", math.Round),
		"roundtoeven": numericFunction("RoundToEven", math.RoundToEven),
		"sin":         numericFunction("Sin", math.Sin),
		"sinh":        numericFunction("Sinh", math.Sinh),
		"sqrt":        numericFunction("Sqrt", math.Sqrt),
		"tan":         numericFunction("Tan", math.Tan),
		"tanh":        numericFunction("Tanh", math.Tanh),
		"trunc":       numericFunction("Trunc", math.Trunc),
		"y0":          numericFunction("Y0", math.Y0),
		"y1":          numericFunction("Y1", math.Y1),

		"pow10": func(node cty.Value) (result cty.Value, err error) {
			return cty.EmptyObjectVal, nil
		},
		"length": func(node cty.Value) (result cty.Value, err error) {
			if node.IsNull() {
				return cty.NilVal, fmt.Errorf("Can't find length on nil")
			}
			if len(node.Type().TestConformance(cty.String)) == 0 {
				strlen := len(node.AsString())
				return cty.NumberIntVal(int64(strlen)), nil
			}
			return node.Length(), nil
		},
		"not": func(node cty.Value) (result cty.Value, err error) {
			return result.Not(), nil
		},
	}

	constants = map[string]cty.Value{
		"true": cty.True,
		"false": cty.False,
	}
)

// AddFunction add a function for internal JSONPath script
func AddFunction(alias string, function Function) {
	functions[strings.ToLower(alias)] = function
}

// AddOperation add an operation for internal JSONPath script
func AddOperation(alias string, prior uint8, right bool, operation Operation) {
	alias = strings.ToLower(alias)
	operations[alias] = operation
	priority[alias] = prior
	priorityChar[alias[0]] = true
	if right {
		rightOp[alias] = true
	}
}

// AddConstant add a constant for internal JSONPath script
func AddConstant(alias string, value cty.Value) {
	constants[strings.ToLower(alias)] = value
}

func numericFunction(name string, fn func(float float64) float64) Function {
	return func(node cty.Value) (result cty.Value, err error) {
		x, _ := node.AsBigFloat().Float64()
		return cty.NumberFloatVal(fn(x)), nil
	}
}

func mathFactorial(x uint) uint {
	if x == 0 {
		return 1
	}
	return x * mathFactorial(x-1)
}
