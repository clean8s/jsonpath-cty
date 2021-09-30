package kv

import (
	peek "github.com/clean8s/peekcty"
)

type prebuilt map[string]peek.Val

func (p prebuilt) Build() map[string]peek.Val {
	return p
}

func New(key string, value peek.Val) peek.KVBuilder {
	M := map[string]peek.Val{}
	M[key] = value
	return prebuilt(M)
}

func NewMany(keyValueMany ...interface{}) peek.KVBuilder {
	if len(keyValueMany) % 2 != 0 {
		return prebuilt{}
	}
	M := map[string]peek.Val{}
	for i := 0; i < len(keyValueMany); i+= 2 {
		var key peek.Val
		k, v := keyValueMany[i], keyValueMany[i + 1]
		switch realK := k.(type) {
		case string:
			key = peek.Str(realK)
		case peek.Val:
			if !realK.Is(peek.StrType) {
				continue
			}
			key = realK
		}
		if pv, ok := v.(peek.Val); ok {
			M[key.AsString()] = pv
		}
	}
	return prebuilt(M)
}

func NewFromMap(val map[string]peek.Val) peek.KVBuilder {
	return prebuilt(val)
}

