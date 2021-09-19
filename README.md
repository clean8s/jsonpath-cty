# JSONPath for go-cty: peekty

[![Go Test](https://github.com/clean8s/jsonpathcty/actions/workflows/go.yml/badge.svg)](https://github.com/clean8s/jsonpathcty/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/clean8s/jsonpathcty.svg)](https://pkg.go.dev/github.com/clean8s/jsonpathcty)

**peekcty** lets you iterate over `cty` datastructures using JSONPath syntax.

Note: [go-cty](https://github.com/zclconf/go-cty/) is the serialization / typesystem library
for Go powering HCL, Terraform, zclconf.

## Example

Given `text_fixture_cars.json`:
```go
import "github.com/clean8s/peekcty"

func demo() {
  p, err := peekcty.NewPath("$..has")
  fmt.Println(p.Search(carExample))
}
```

Prints:
```
".carOwners.A.has" => []{StringVal("Honda Accord"), StringVal("VW Up"), StringVal("Porsche 911")}
".carOwners.B.has" => []{StringVal("Renault Clio"), StringVal("Jaguar F-Type"), StringVal("Dodge Viper")}
".cars[0].has" => []{StringVal("4 doors")}
```

## Implementation

It's based on Kubernetes/`kubectl`'s implementation
[here](https://github.com/kubernetes/client-go/blob/cc7616029c18572e01973d10efe5391e3140c050/util/jsonpath/jsonpath.go#L44).
With two differences:
* it doesn't require templates or `range` blocks
* it operates on `cty.Value` instead of `reflect.Value`


You can use all features except for filters:

* `$[0, 1]`
* `$.field`
* `$.wildcard[*]`
* `$.x.y..recursive`
* `m[1:]`, `slice2[:2]`, `slice3[1:5]`

## LICENSE

Licensed under MIT.

This is an extension not officially affiliated with the `cty` library by Martin Atkins.