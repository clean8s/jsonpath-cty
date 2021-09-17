# JSONPath for the cty Go library

[![Go Test](https://github.com/clean8s/jsonpathcty/actions/workflows/go.yml/badge.svg)](https://github.com/clean8s/jsonpathcty/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/clean8s/jsonpathcty.svg)](https://pkg.go.dev/github.com/clean8s/jsonpathcty)

`jsonpathcty` lets you iterate over `cty` datastructures using JSONPath syntax.

Note: [cty](https://github.com/zclconf/go-cty/) is the serialization / typesystem library
for Go powering HCL, Terraform, zclconf.

## Example

```go
import "github.com/clean8s/jsonpathcty"

func demo() {
  name := jsonpathcty.MustNewPath("$.Name").Apply(someObj)
}
```

## Implementation

It's based on [spyzhov/ajson](https://github.com/spyzhov/ajson), by replacing
`ajson.Node` operations with the corresponding `cty` operations.

You can use all features:

* `$`
* `$.field`
* `$.wildcard[*]`
* `$.x.y..recursive`
* `["field1", "field2", ...]`
* `m[1:]`, `slice2[:2]`, `slice3[1:5]`
* `[?(expr)]`
* `$.a.items.length`

Scripting outside of filters is not allowed.

## LICENSE

cty and ajson are licensed under MIT, and are created by:

    ajson: Copyright (c) 2019 Pyzhov Stepan
    cty:   Copyright (c) 2017-2018 Martin Atkins

This library, jsonpath-cty, is licensed under the MIT license.
