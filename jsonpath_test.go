package peekcty

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	_ "embed"
	"strings"
)

var sampleDoc cty.Value

func TestParsing(t *testing.T) {
	t.Run("pick", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Val{
			"$":      Tuple(sampleDoc),
			"$.A[0]": Tuple(Str("string")),
			"$.A":    Tuple(Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal)),
			"$.A[*]": Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
			"$.A.*":  Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
		})
	})

	t.Run("slice", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Val{
			"$.A[2]":          Tuple(Num(3)),
			"$.A[1:4]":        Tuple(Float(23.3), Num(3), cty.True),
			"$.A[::2]":        Tuple(Str("string"), Num(3), cty.False),
			"$.A[-2:]":        Tuple(cty.False, cty.NilVal),
			"$.A[:-1]":        Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False),
			"$.F.V[4:5][0,1]": Tuple(Str("string5a"), Str("string5b")),
			"$.F.V[4:6][0,1]": Tuple(Str("string5a"), Str("string6a"), Str("string5b"), Str("string6b")),
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

	t.Run("search", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Val{
			"$..C":        Tuple(Float(3.14), Float(3.1415), Float(3.141592), Float(3.14159265)),
			"$.D.V..C":    Tuple(Float(3.141592)),
			"$.D.V.*.C":   Tuple(Float(3.141592)),
			"$.D.*..C":    Tuple(Float(3.141592)),
			"$.*.V..C":    Tuple(Float(3.141592)),
			"$.*.D.V.C": Tuple(Float(3.14159265)),
			"$.*.D..C":    Tuple(Float(3.14159265)),
			"$.*.D.V...C": Tuple(Float(3.14159265)),
			"$..D..V..C":  Tuple(Float(3.141592), Float(3.14159265)),
			"$.*.*.*.C":   Tuple(Float(3.141592), Float(3.14159265)),
			"$..V..C":     Tuple(Float(3.141592), Float(3.14159265)),
			"$..A": Tuple(
				Tuple(Str("string"), Float(23.3), Float(3), cty.True, cty.False, cty.NilVal),
				Tuple(Str("string3")),
			),
			"$..A..":       Tuple(Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal), Tuple(Str("string3"))),
			"$.A..":        Tuple(Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal)),
			"$.A.*":        Tuple(Str("string"), Float(23.3), Num(3), cty.True, cty.False, cty.NilVal),
			"$..A[0]":      Tuple(Str("string"), Str("string3")),
			"$.*.V[0]":     Tuple(Str("string2a"), Str("string4a")),
			"$.*.V[1]":     Tuple(Str("string2b"), Str("string4b")),
			"$.*.V[0,1]":   Tuple(Str("string2a"), Str("string4a"), Str("string2b"), Str("string4b")),
			"$.*.V[0:2]":   Tuple(Str("string2a"), Str("string2b"), Str("string4a"), Str("string4b")),
			"$.*.V[2].C":   Tuple(Float(3.141592)),
			"$..V[*].C":    Tuple(Float(3.141592)),
			"$..V[*].*": Tuple(
				Float(3.141592),
				Float(3.1415926535),
				Str("hello"),
				Str("string5a"),
				Str("string5b"),
				Str("string6a"),
				Str("string6b"),
			),
			"$..ZZ": Tuple(),
		})
	})
}

func TestErrors(t *testing.T) {
	tests := []string{
		`$["]`,
		`$[A][0`,
		"$.A*]",
		"$.A[1:4:0:0]",
	}
	assertError(t, tests)
}

func assert(t *testing.T, doc Val, tests map[string]Val) {
	for path, expected := range tests {
		actualT, err := NewPath(path)
		if err != nil {
			t.Fatal("failed parsing", err)
		}
		v, _, err := actualT.Eval(doc)
		actual := cty.TupleVal(v)
		exp, _ := ctyjson.Marshal(expected, expected.Type())
		act, _ := ctyjson.Marshal(actual, actual.Type())
		if err != nil {
			t.Error("failed:", path, err)
		} else if string(exp) != string(act) {
			t.Errorf("failed: mismatch for %s\nexpected: %+v\nactual: %+v", path, string(exp), string(act))
		}
	}
}

func assertError(t *testing.T, tests []string) {
	for _, path := range tests {
		_, err := NewPath(path)
		if err == nil {
			t.Error("path", path, "should fail")
		}
	}
}

func TestSearch(t *testing.T) {
	p, _ := NewPath("$.*.*.has")
	vals, _, _ := p.Eval(carExample.Value)
	if len(vals) != 3 {
		t.Fatal("Wrong car example len()", len(vals))
	}
	searchStr := p.Search(carExample.Value).String()
	if !strings.Contains(searchStr, ".carOwners.B.has") {
		t.Fatal("Search() doesn't yield valid res", searchStr)
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
	carExample.UnmarshalJSON(carBytes)
	doc2Json, _ := json.Marshal(DemoSample)
	jType2, _ := ctyjson.ImpliedType(doc2Json)
	sampleDoc, _ = ctyjson.Unmarshal(doc2Json, jType2)
	os.Exit(m.Run())
}

//go:embed test_fixture_cars.json
var carBytes []byte
var carExample ctyjson.SimpleJSONValue

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