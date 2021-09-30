package peek

import (
	"github.com/zclconf/go-cty/cty"
)

// Implemented in kv.New and kv.Many in package
// kv
type KVBuilder interface {
	Build() map[string]Val
}

func Obj(b ...KVBuilder) Val {
	M := map[string]cty.Value{}
	for _, item := range b {
		for k, v := range item.Build() {
			M[k] = cty.Value(v)
		}
	}

	return Val(cty.ObjectVal(M))
}

func Map(b ...KVBuilder) Val {
	M := map[string]cty.Value{}
	for _, item := range b {
		for k, v := range item.Build() {
			M[k] = cty.Value(v)
		}
	}

	return Val(cty.MapVal(M))
}


