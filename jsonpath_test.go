package peek

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	_ "embed"
	"strings"
	"github.com/clean8s/peekcty/jsonpath"
)

var sampleDoc Value

func TestParsing(t *testing.T) {
	t.Run("pick", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Value{
			"$":      Tuple(sampleDoc),
			"$.A[0]": Tuple(Str("string")),
			"$.A":    Tuple(Tuple(Str("string"), NumFloat(23.3), Num(3), True, False, Nil)),
			"$.A[*]": Tuple(Str("string"), NumFloat(23.3), Num(3), True, False, Nil),
			"$.A.*":  Tuple(Str("string"), NumFloat(23.3), Num(3), True, False, Nil),
		})
	})

	t.Run("slice", func(t *testing.T) {
		assert(t, sampleDoc, map[string]Value{
			"$.A[2]":          Tuple(Num(3)),
			"$.A[1:4]":        Tuple(NumFloat(23.3), Num(3), True),
			"$.A[::2]":        Tuple(Str("string"), Num(3), False),
			"$.A[-2:]":        Tuple(False, Nil),
			"$.A[:-1]":        Tuple(Str("string"), NumFloat(23.3), Num(3), True, False),
			"$.F.Type[4:5][0,1]": Tuple(Str("string5a"), Str("string5b")),
			"$.F.Type[4:6][0,1]": Tuple(Str("string5a"), Str("string6a"), Str("string5b"), Str("string6b")),
			"$.F.Type[4,5][0:2]": Tuple(Str("string5a"), Str("string5b"), Str("string6a"), Str("string6b")),
			"$.F.Type[4:6]": Tuple(
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
		assert(t, sampleDoc, map[string]Value{
			"$..C":        Tuple(NumFloat(3.14), NumFloat(3.1415), NumFloat(3.141592), NumFloat(3.14159265)),
			"$.D.Type..C":    Tuple(NumFloat(3.141592)),
			"$.D.Type.*.C":   Tuple(NumFloat(3.141592)),
			"$.D.*..C":    Tuple(NumFloat(3.141592)),
			"$.*.Type..C":    Tuple(NumFloat(3.141592)),
			"$.*.D.Type.C": Tuple(NumFloat(3.14159265)),
			"$.*.D..C":    Tuple(NumFloat(3.14159265)),
			"$.*.D.Type...C": Tuple(NumFloat(3.14159265)),
			"$..D..Type..C":  Tuple(NumFloat(3.141592), NumFloat(3.14159265)),
			"$.*.*.*.C":   Tuple(NumFloat(3.141592), NumFloat(3.14159265)),
			"$..Type..C":     Tuple(NumFloat(3.141592), NumFloat(3.14159265)),
			"$..A": Tuple(
				Tuple(Str("string"), NumFloat(23.3), NumFloat(3), True, False, Nil),
				Tuple(Str("string3")),
			),
			"$..A..":       Tuple(Tuple(Str("string"), NumFloat(23.3), Num(3), True, False, Nil), Tuple(Str("string3"))),
			"$.A..":        Tuple(Tuple(Str("string"), NumFloat(23.3), Num(3), True, False, Nil)),
			"$.A.*":        Tuple(Str("string"), NumFloat(23.3), Num(3), True, False, Nil),
			"$..A[0]":      Tuple(Str("string"), Str("string3")),
			"$.*.Type[0]":     Tuple(Str("string2a"), Str("string4a")),
			"$.*.Type[1]":     Tuple(Str("string2b"), Str("string4b")),
			"$.*.Type[0,1]":   Tuple(Str("string2a"), Str("string4a"), Str("string2b"), Str("string4b")),
			"$.*.Type[0:2]":   Tuple(Str("string2a"), Str("string2b"), Str("string4a"), Str("string4b")),
			"$.*.Type[2].C":   Tuple(NumFloat(3.141592)),
			"$..Type[*].C":    Tuple(NumFloat(3.141592)),
			"$..Type[*].*": Tuple(
				NumFloat(3.141592),
				NumFloat(3.1415926535),
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

func assert(t *testing.T, doc Value, tests map[string]Value) {
	for path, expected := range tests {
		actualT, err := jsonpath.NewPath(path)
		if err != nil {
			t.Fatal("failed parsing", err)
		}
		v, _, err := actualT.Eval(cty.Value(doc))
		actual := cty.TupleVal(v)
		exp, _ := expected.MarshalJSON()
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
		_, err := jsonpath.NewPath(path)
		if err == nil {
			t.Error("path", path, "should fail")
		}
	}
}

func TestSearch(t *testing.T) {
	p, _ := jsonpath.NewPath("$.*.*.has")
	vals, _, _ := p.Eval(carExample.Value)
	if len(vals) != 3 {
		t.Fatal("Wrong car example len()", len(vals))
	}
	searchStr := p.Search(carExample.Value).String()
	if !strings.Contains(searchStr, ".carOwners.B.has") {
		t.Fatal("Search() doesn't yield valid res", searchStr)
	}
}

func TestMain(m *testing.M) {
	carExample.UnmarshalJSON(carBytes)
	doc2Json, _ := json.Marshal(DemoSample)
	jType2, _ := ctyjson.ImpliedType(doc2Json)
	sampleDocJson, _ := ctyjson.Unmarshal(doc2Json, jType2)
	sampleDoc = Value(sampleDocJson)
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
		"Type": []interface{}{
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
			"Type": map[string]interface{}{
				"C": 3.14159265,
			},
		},
	},
	"F": map[string]interface{}{
		"Type": []interface{}{
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