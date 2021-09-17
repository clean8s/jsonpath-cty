package jsonpathcty

import (
	"testing"
	"github.com/zclconf/go-cty/cty/gocty"
	"github.com/zclconf/go-cty/cty"
)

type Car struct {
	Brand string `cty:"Brand"`
	Model string `cty:"Model"`
	Color string `cty:"Color"`
}
type Person struct {
	Name         string `cty:"Name"`
	Surname      string `cty:"Surname"`
	QuoteInField string `cty:"'"`
	Cars         []Car  `cty:"Cars"`
}

var Don = Person{"Don", "Knuth", "Something", []Car{
	{"Honda", "Civic", "red"},
	{"Ford", "Mustang", "green"},
	{"Honda", "Accord", "black"},
}}

var Andrew = Person{"Andrew", "Woo", "orange", []Car{}}

func TestNewPath(t *testing.T) {
	validPaths := map[string]string{
		"root": "$",
		"dot":  "$.x",
		"accessorSimple": "$['x']",
		"accessorTwoValues": "$['x', 'y']",
	}
	for name, val := range validPaths {
		t.Run(name, func(t *testing.T) {
			_, err := NewPath(val)
			if err != nil {
				t.Fatal("Got an error on a valid path", err)
			}
		})
	}
}

func TestMustNewPath(t *testing.T) {
	p := MustNewPath("invalidPath'")
	if len(p.parts) != 0 {
		t.Fatal("Path should be empty")
	}
}

func TestApply(t *testing.T) {
	var cases = []struct {
		instance      Person
		path          string
		expectedValue Val
		testName      string
	}{
		{Don, "$.Name", List(Str("Don")), `single_field`},
		{Don, "$.Surname", List(Str("Knuth")), `single_field2`},
		{Don, "$['Name']", List(Str("Don")), `name_quoted_bracket`},
		{Don, "$['Name','Surname']", List(Str("Don"), Str("Knuth")), `name_union`},
		{Don, `$["'"]`, List(Str("Something")), `quotedfield`},
		{Don, `$['\'']`, List(Str("Something")), `quotedfield2`},
		{Don, `$.Cars.length`, List(cty.NumberIntVal(3)), `arr_length`},
		{Andrew, `$.Cars.length`, List(cty.NumberIntVal(0)), `arr_length_zero`},
		{Andrew, `$.UnknownField`, cty.NilVal, `field_not_there`},
		{Don, `$.Cars..Brand`, List(Str("Honda"), Str("Ford"), Str("Honda")), `recursive`},
		{Don, `$.Cars[0].Color`, List(Str("red")), `arr_index`},
		{Don, `$.Cars[1:].Color`, List(Str("green"), Str("black")), `arr_slice3`},
		{Don, `$.Cars[:1].Color`, List(Str("red")), `arr_slice`},
		{Don, `$.Cars[0:1].Color`, List(Str("red")), `arr_slice2`},
		{Don, `$.Cars[*].Color`, List(Str("red"), Str("green"), Str("black")), `wildcard`},
		{Don, `$.Cars[?(@.Brand == 'Honda')].length`, List(cty.NumberIntVal(2)), `filter`},
	}

	var PersonType, _ = gocty.ImpliedType(Person{})
	for _, curCase := range cases {
		t.Run(curCase.testName, func(t *testing.T) {
			itemCty, _ := gocty.ToCtyValue(curCase.instance, PersonType)
			p, pathErr := NewPath(curCase.path)
			if pathErr != nil {
				t.Fatal("path != nil", pathErr)
			}
			values, err := p.Apply(itemCty)
			if err != nil {
				t.Fatal("err != nil", err)
			}

			if values == nil || len(values) == 0 {
				if !curCase.expectedValue.IsNull() {
					t.Fatal("expected non-empty result")
				}
				return
			}
			if !cty.ListVal(values).Equals(curCase.expectedValue).True() {
				t.Fatal("result != expectedValue", cty.ListVal(values).GoString())
			}
		})
	}

}

var Str = cty.StringVal
var List = func(v ...Val) Val {
	return cty.ListVal(v)
}

type Val = cty.Value
