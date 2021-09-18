package jsonpathcty

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"fmt"
)

var sampleDoc cty.Value

func TestPaths(t *testing.T) {
	val, pp, _ := NewPath("$.A[0]").Evaluate(sampleDoc)
	fmt.Println(val, pp)
}

func TestParsing(t *testing.T) {
	t.Run("pick", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Val{
			"$":         sampleDoc,
			"$.A[0]":    Str("string"),
			`$["A"][0]`: Str("string"),
			"$.A":       Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
			"$.A[*]":    Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
			"$.A.*":     Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
			// "$.A.*.a":   Tuple(),
		})
	})

	t.Run("slice", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Val{
			"$.A[2]":          Num(3),
			`$["B","C"]`:      Tuple(Str("value"), Float(3.14)),
			`$["C","B"]`:      Tuple(Float(3.14), Str("value")),
			"$.A[1:4]":        Tuple(Float(23.3), Num(3), cty.True),
			"$.A[::2]":        Tuple(Str("string"), Num(3), cty.False),
			"$.A[-2:]":        Tuple(cty.False, cty.NilVal),
			"$.A[:-1]":        Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False),
			"$.A[::-1]":       Tuple(cty.NilVal, cty.False, cty.True, Num(3), Float(23.3), Str("string")),
			"$.F.V[4:5][0,1]": Tuple(Str("string5a"), Str("string5b")),
			// "$.F.V[4:6][1]":   Tuple(Str("string6a"), Str("string6b")),
			"$.F.V[4:6][0,1]": Tuple(Str("string5a"), Str("string5b"), Str("string6a"), Str("string6b")),
			"$.F.V[4,5][0:2]": Tuple(Str("string5a"), Str("string5b"), Str("string6a"), Str("string6b")),
			"$.F.V[4:6]": Tuple(
				Tuple(
					Str("string5a"),
					Str("string5b"),
				),
				Tuple(
					Str("string6a"),
					Str("string6b"),
				),
			),
		})
	})

	t.Run("quote", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Val{
			`$[A][0]`:    Str("string"),
			`$["A"][0]`:  Str("string"),
			`$[B,C]`:     Tuple(Str("value"), Float(3.14)),
			`$["B","C"]`: Tuple(Str("value"), Float(3.14)),
		})
	})

	t.Run("search", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Val{
			"$..C":       Tuple(Float(3.14), Float(3.1415), Float(3.141592), Float(3.14159265)),
			`$..["C"]`:   Tuple(Float(3.14), Float(3.1415), Float(3.141592), Float(3.14159265)),
			"$.D.V..C":   Tuple(Float(3.141592)),
			"$.D.V.*.C":  Tuple(Float(3.141592)),
			"$.D.V..*.C": Tuple(Float(3.141592)),
			"$.D.*..C":   Tuple(Float(3.141592)),
			"$.*.V..C":   Tuple(Float(3.141592)),
			"$.*.D.V.C":  Tuple(Float(3.14159265)),
			"$.*.D..C":   Tuple(Float(3.14159265)),
			"$.*.D.V..*": Tuple(Float(3.14159265)),
			"$..D..V..C": Tuple(Float(3.141592), Float(3.14159265)),
			"$.*.*.*.C":  Tuple(Float(3.141592), Float(3.14159265)),
			"$..V..C":    Tuple(Float(3.141592), Float(3.14159265)),
			"$.D.V..*": Tuple(
				Str("string2a"),
				Str("string2b"),
				cty.MapVal(map[string]cty.Value{"C": Float(3.141592)}),
				Float(3.141592),
			),
			"$..A": Tuple(
				Tuple(Str("string"), Float(23.3), Float(3), cty.True, cty.False, cty.NilVal),
				Tuple(Str("string3")),
			),
			"$..A..*": Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal, Str("string3")),
			"$.A..*":  Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
			"$.A.*":   Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
			// "$..A[0,1]":    Tuple(Str("string"), Float(23.3)),
			"$..A[0]":      Tuple(Str("string"), Str("string3")),
			"$.*.V[0]":     Tuple(Str("string2a"), Str("string4a")),
			"$.*.V[1]":     Tuple(Str("string2b"), Str("string4b")),
			"$.*.V[0,1]":   Tuple(Str("string2a"), Str("string2b"), Str("string4a"), Str("string4b")),
			"$.*.V[0:2]":   Tuple(Str("string2a"), Str("string2b"), Str("string4a"), Str("string4b")),
			"$.*.V[2].C":   Tuple(Float(3.141592)),
			"$..V[2].C":    Tuple(Float(3.141592)),
			"$..V[*].C":    Tuple(Float(3.141592)),
			"$.*.V[2].*":   Tuple(Float(3.141592), Float(3.1415926535)),
			"$.*.V[2:3].*": Tuple(Float(3.141592), Float(3.1415926535)),
			"$.*.V[2:4].*": Tuple(Float(3.141592), Float(3.1415926535), Str("hello")),
			"$..V[2,3].CC": Tuple(Float(3.1415926535), Str("hello")),
			"$..V[2:4].CC": Tuple(Float(3.1415926535), Str("hello")),
			"$..V[*].*": Tuple(
				Float(3.141592),
				Float(3.1415926535),
				Str("hello"),
				Str("string5a"),
				Str("string5b"),
				Str("string6a"),
				Str("string6b"),
			),
			"$..[0]": Tuple(
				Str("string"),
				Str("string2a"),
				Str("string3"),
				Str("string4a"),
				Str("string5a"),
				Str("string6a"),
			),
			"$..ZZ": Tuple(),
		})
	})
}

func TestErrors(t *testing.T) {
	tests := map[string]string{
		".A":           "path must start with a '$'",
		"$.":           "expected JSON child identifier after '.'",
		"$.1":          "unexpected token .1",
		"$.A[]":        "expected at least one key, index or expression",
		`$["]`:         "bad string invalid syntax",
		`$[A][0`:       "unexpected end of path",
		"$.ZZZ":        "attribute 'ZZZ'",
		"$.A*]":        "unexpected token *",
		"$.*V":         "unexpected token V",
		"$[B,C":        "unexpected end of path",
		"$.A[1,4.2]":   "unexpected token '.'",
		"$[C:B]":       "not a number",
		"$.A[1:4:0:0]": "bad range syntax [start:end:step]",
		"$.A[:,]":      "unexpected token ','",
		"$..":          "cannot end with a scan '..'",
		"$..1":         "unexpected token '1' after deep search '..'",
	}
	assertError(t, sampleDoc, tests)
}

func assert(t *testing.T, doc Val, tests map[string]Val) {
	for path, expected := range tests {
		actual, _, err := NewPath(path).Evaluate(doc)
		exp, _ := ctyjson.Marshal(expected, expected.Type())
		act, _ := ctyjson.Marshal(actual, actual.Type())
		if err != nil {
			t.Error("failed:", path, err)
		} else if string(exp) != string(act) {
			t.Errorf("failed: mismatch for %s\nexpected: %+v\nactual: %+v", path, string(exp), string(act))
		}
	}
}

func assertError(t *testing.T, doc Val, tests map[string]string) {
	for path, expectedError := range tests {
		_, _, err := NewPath(path).Evaluate(doc)
		if err == nil {
			t.Error("path", path, "should fail with", expectedError)
		} else if !strings.Contains(err.Error(), expectedError) {
			t.Error("path", path, "should fail with ", expectedError, "but failed with:", err)
		}
	}
}

type Val = cty.Value

func Str(S string) Val {
	return cty.StringVal(S)
}

func List(v ...Val) Val {
	return cty.ListVal(v)
}

func Num(n int) Val {
	return cty.NumberIntVal(int64(n))
}

func Float(f float64) Val {
	return cty.NumberFloatVal(f)
}

func Tuple(v ...Val) Val {
	return cty.TupleVal(v)
}

func TestMain(m *testing.M) {
	doc2Json, _ := json.Marshal(DemoSample)
	jType2, _ := ctyjson.ImpliedType(doc2Json)
	sampleDoc, _ = ctyjson.Unmarshal(doc2Json, jType2)
	os.Exit(m.Run())
}

var DemoSample = map[string]interface{}{
	"A": []interface{}{
		"string",
		23.3,
		3,
		true,
		false,
		nil,
	},
	"B": "value",
	"C": 3.14,
	"D": map[string]interface{}{
		"C": 3.1415,
		"V": []interface{}{
			"string2a",
			"string2b",
			map[string]interface{}{
				"C": 3.141592,
			},
		},
	},
	"E": map[string]interface{}{
		"A": []interface{}{"string3"},
		"D": map[string]interface{}{
			"V": map[string]interface{}{
				"C": 3.14159265,
			},
		},
	},
	"F": map[string]interface{}{
		"V": []interface{}{
			"string4a",
			"string4b",
			map[string]interface{}{
				"CC": 3.1415926535,
			},
			map[string]interface{}{
				"CC": "hello",
			},
			[]interface{}{
				"string5a",
				"string5b",
			},
			[]interface{}{
				"string6a",
				"string6b",
			},
		},
	},
}
