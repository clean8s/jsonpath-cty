package peek

import "github.com/zclconf/go-cty/cty"

type ObjectBuilder struct{
	value map[string]Value
}

func Obj() Value {
	return Value(cty.EmptyObjectVal)
}

func NewObjBuilder() ObjectBuilder {
	return ObjectBuilder{make(map[string]Value)}
}

func (o ObjectBuilder) FromStringMap(val map[string]Value) ObjectBuilder {
	o.value = val
	return o
}

func (o ObjectBuilder) Put(key string, value Value) ObjectBuilder {
	o.value[key] = value
	return o
}

func (o ObjectBuilder) Merge(v Value) ObjectBuilder {
	ch := v.Children()
	if ch.KeysRepresentPosition() {
		return o
	}
	for _, item := range ch {
		if item.Key.Is(StrType) {
			o.value[item.Key.Str()] = item.Value
		}
	}
	return o
}

func (o ObjectBuilder) Value() Value {
	return Value(cty.ObjectVal(MapConvert.ToCty(o.value)))
}

