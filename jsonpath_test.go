package jsonpathcty

import (
	"testing"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"os"
	"strings"
	_ "embed"
)

//go:embed test_fixture_cars.json
var EXAMPLE []byte

func TestA(t *testing.T) {
	var cases = []struct {
		path string
		expected string
	}{
		{
			path: "$..has",
			expected: r(`
				[
				{".carOwners.A.has": ["Honda Accord","VW Up","Porsche 911"]},
				{".carOwners.B.has": ["Renault Clio","Jaguar F-Type","Dodge Viper"]},
				{".cars[0].has": ["4 doors"]}
				]
			`),
		},
		{
			path: "$.carOwners.*.name",
			expected: r(`
				[
				{".carOwners.A.name": "Don Knuth"},
				{".carOwners.B.name": "Francois Touis"}
				]
			`),
		},
		{
			path: "$.cars[0]",
			expected: r(`
				[
				{".cars[0]": {"has":["4 doors"],"model":"Clio","name":"Renault"}}
				]
			`),
		},
		{
			path: "$.*.*",
			expected: r(`
				[
				{".cars[0]": {"has":["4 doors"],"model":"Clio","name":"Renault"}}
				]
			`),
		},
	}
	for _, singleCase := range cases {
		t.Run(singleCase.path, func(t *testing.T) {
			p, err := NewPath(singleCase.path).Evaluate(ExampleCty.Value)
			if err != nil {
				t.Fatal("error != nil: ", err)
			}
			if p.String() != singleCase.expected {
				t.Fatal("not matching", p.String(), singleCase.expected)
			}
		})

	}
}

var ExampleCty = ctyjson.SimpleJSONValue{}
func TestMain(m *testing.M) {
	ExampleCty.UnmarshalJSON(EXAMPLE)
	os.Exit(m.Run())
}

type b = []byte
func r(s string) string {
	out := ""
	for _, line := range strings.Split(s, "\n") {
		line = strings.Trim(line, " \t") + "\n"
		if line != "\n" {
			out += line
		}
	}
	return out
}