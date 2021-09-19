package jsonpathcty

import (
	"testing"
	"fmt"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var testSampleDoc = b(`{
  "A": {
    "B": {
       "C": [1,2,3]
     },
    "C": [3,4,5]
  },
  "D": "str"
}`)

var testSampleDocVal = ctyjson.SimpleJSONValue{}

func TestFormatCtyPath(t *testing.T) {
	testSampleDocVal.UnmarshalJSON(testSampleDoc)
	res, _, err := NewPath(`$["A","D"].C`).Evaluate(testSampleDocVal.Value)
	fmt.Println(res.GoString(), err)
}
