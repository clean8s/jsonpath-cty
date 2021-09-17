// jsonpathcty lets you iterate cty (the library powering Terraform/HCL/zclconf) datastructures using JSONPath syntax.
//
// The implementation is based on github.com/spyzhov/ajson - by replacing ajson.Node with the corresponding cty.Value
package jsonpathcty