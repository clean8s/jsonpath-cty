# JSONPath for the cty Go library

[![Go Test](https://github.com/clean8s/jsonpathcty/actions/workflows/go.yml/badge.svg)](https://github.com/clean8s/jsonpathcty/actions/workflows/go.yml)

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

## LICENSE

cty and ajson are licensed under MIT, and are created by:

    ajson: Copyright (c) 2019 Pyzhov Stepan
    cty:   Copyright (c) 2017-2018 Martin Atkins

This library, jsonpath-cty, is licensed under the MIT license.
