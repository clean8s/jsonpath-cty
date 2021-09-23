package peek

import (
	"github.com/zclconf/go-cty/cty"
	"math/big"
	"github.com/zclconf/go-cty/cty/json"
	"github.com/clean8s/peekcty/jsonpath"
	"github.com/zclconf/go-cty/cty/convert"
)

type Value cty.Value
type Type cty.Type

var ( // Primitives
	NumType  = Type(cty.Number)
	StrType  = Type(cty.String)
	BoolType = Type(cty.Bool)
)

var ( // constants
	True  = Value(cty.True)
	False = Value(cty.False)
	Zero  = Value(cty.Zero)
)

func Num(val int) Value {
	return Value(cty.NumberIntVal(int64(val)))
}

func NumFloat(val float64) Value {
	return Value(cty.NumberFloatVal(val))
}

func Str(val string) Value {
	return Value(cty.StringVal(val))
}

func Bool(val bool) Value {
	return Value(cty.BoolVal(val))
}

func List(vals ...Value) Value {
	return Value(cty.ListVal(SliceConvert.ToCty(vals)))
}

func Tuple(vals ...Value) Value {
	return Value(cty.TupleVal(SliceConvert.ToCty(vals)))
}

func Set(vals ...Value) Value {
	return Value(cty.SetVal(SliceConvert.ToCty(vals)))
}

var Nil = Value(cty.NilVal)
var Unknown = Value(cty.DynamicVal)
var UnknownType = Type(cty.DynamicPseudoType)

func (v Value) Hash() int {
	return (v.CtyValue().Hash())
}

func (v Value) GoString() string {
	return (v.CtyValue().GoString())
}

func (v Value) Equals(other Value) Value {
	return Value(v.CtyValue().Equals(cty.Value(other)))
}

func (v Value) NotEqual(other Value) Value {
	return Value(v.CtyValue().NotEqual(cty.Value(other)))
}

func (v Value) Type() Type {
	return Type(v.CtyValue().Type())
}

func (v Value) Is(typ Type) bool {
	if v.Type().Equals(cty.Type(typ)) {
		return true
	}
	return false
}

func (v Type) CtyType() cty.Type {
	return cty.Type(v)
}

func (v Type) IsList() bool {
	return v.CtyType().IsListType()
}

func (v Type) IsMap() bool {
	return v.CtyType().IsMapType()
}

func (v Type) IsPrimitive() bool {
	return v.CtyType().IsPrimitiveType()
}

func (v Type) IsObject() bool {
	return v.CtyType().IsObjectType()
}

func (v Type) IsTuple() bool {
	return v.CtyType().IsTupleType()
}

func (v Type) IsSet() bool {
	return v.CtyType().IsSetType()
}

// --

func (v Value) IsList() bool {
	return v.Type().IsList()
}

func (v Value) IsMap() bool {
	return v.Type().IsMap()
}

func (v Value) IsSet() bool {
	return v.Type().IsSet()
}

func (v Value) IsTuple() bool {
	return v.Type().IsTuple()
}

func (v Value) IsPrimitive() bool {
	return v.Type().IsPrimitive()
}

func (v Value) IsObject() bool {
	return v.Type().IsObject()
}

func (v Value) Get(indexOrAttr Value) Value {
	c, _ := v.CtyValue().Unmark()
	idx, _ := indexOrAttr.CtyValue().Unmark()
	//if !c.CanIterateElements() {
	//	return Unknown
	//}
	if v.IsSet() {
		if v.CtyValue().HasElement(indexOrAttr.CtyValue()).True() {
			return indexOrAttr
		} else {
			return Unknown
		}
	}
	if v.IsObject() {
		if !idx.Type().Equals(cty.String) {
			return Unknown
		}
		if !c.Type().HasAttribute(idx.AsString()) {
			return Unknown
		}
		return Value(c.GetAttr(idx.AsString()))
	}
	if !idx.HasIndex(idx).True() {
		return Unknown
	}
	return Value(idx.Index(idx))
}

func (v Value) Children() Children {
	ct, _ := v.CtyValue().Unmark()
	out := make(Children, 0)
	if !ct.CanIterateElements() {
		return out
	}
	it := ct.ElementIterator()

	keyIsPosition := false
	if v.IsList() || v.IsTuple() {
		keyIsPosition = true
	}
	for it.Next() {
		k, v := it.Element()
		out = append(out, Child{
			Key:   Value(k),
			Value: Value(v),
			KeyRepresentsPosition: keyIsPosition,
		})
	}

	return out
}

func (v Value) Str() string {
	if !v.Is(StrType) {
		return ""
	}
	return v.CtyValue().AsString()
}

func (v Value) Int() int {
	return int(v.Int64())
}

func (v Value) Int64() int64 {
	ret, _ := v.CtyValue().AsBigFloat().Int64()
	return ret
}

func (v Value) Float() float64 {
	ret, _ := v.BigFloat().Float64()
	return ret
}

func (v Value) BigFloat() *big.Float {
	if !v.Is(NumType) {
		return big.NewFloat(0)
	}
	return v.CtyValue().AsBigFloat()
}

func (v Value) Bool() bool {
	if !v.Is(BoolType) {
		return false
	}
	return v.CtyValue().True()
}

type Child struct { Key Value; Value Value; KeyRepresentsPosition bool }
type Children []Child

func (c Children) UnifiedKeyType() Type {
	if len(c) == 0 {
		return UnknownType
	}
	types := []cty.Type{}
	for _, child := range c {
		types = append(types, cty.Type(child.Key.Type()))
	}
	unifiedT, _ := convert.Unify(types)
	return Type(unifiedT)
}

func (c Children) KeysRepresentPosition() bool {
	if len(c) == 0 || !c.UnifiedKeyType().Equals(cty.Number) {
		return false
	}
	return c[0].KeyRepresentsPosition
}

func (kv Children) Merge(c Children) Children {
	return append(kv, c...)
}

func (kv Children) TryCtyMap() map[string]cty.Value {
	return MapConvert.ToCty(kv.TryStringMap())
}

func (kv Children) TryStringMap() map[string]Value {
	ret := make(map[string]Value)
	for _, item := range kv {
		if !item.Key.Is(StrType) {
			continue
		}
		ret[item.Key.Str()] = item.Value
	}
	return ret
}

func (v Value) Search(jsonPath string) []Value {
	p, err := jsonpath.NewPath(jsonPath)
	if err != nil {
		return nil
	}
	return SliceConvert.FromCty(p.Search(cty.Value(v)).Values)
}

func (v Value) Len() int {
	return v.CtyValue().LengthInt()
}

func (v Value) MarshalJSON() ([]byte, error) {
	s := json.SimpleJSONValue{cty.Value(v)}
	return s.MarshalJSON()
}

var SliceConvert = struct {
	ToCty   func([]Value) []cty.Value
	FromCty func([]cty.Value) []Value
}{
	func(vals []Value) []cty.Value {
		var result = []cty.Value{}
		for _, val := range vals {
			result = append(result, cty.Value(val))
		}
		return result
	},
	func(vals []cty.Value) []Value {
		var result = []Value{}
		for _, val := range vals {
			result = append(result, Value(val))
		}
		return result
	},
}

var MapConvert = struct {
	ToCty   func(map[string]Value) map[string]cty.Value
}{
	func(vals map[string]Value) map[string]cty.Value {
		ctyMap := make(map[string]cty.Value)
		for k, v := range vals {
			ctyMap[k] = cty.Value(v)
		}
		return ctyMap
	},
}
