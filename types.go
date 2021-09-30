package peek

import (
	"github.com/zclconf/go-cty/cty"
	"math/big"
	"github.com/zclconf/go-cty/cty/json"
	"github.com/clean8s/peekcty/jsonpath"
	"github.com/zclconf/go-cty/cty/convert"
)

type Val cty.Value
type Type cty.Type

var ( // Primitives
	NumType  = Type(cty.Number)
	StrType  = Type(cty.String)
	BoolType = Type(cty.Bool)
)

var ( // constants
	True  = Val(cty.True)
	False = Val(cty.False)
	Zero  = Val(cty.Zero)
)

func MergeCollections(val1, val2 Val) {

}

func (v Val) IsIterable() bool {
	v = v.Unmark()
	if v.IsUnknown() || v.IsNil() || v.IsCapsule() {
		return false
	}
	if v.IsPrimitive() {
		return false
	}
	return true
}

func (v Val) IsCapsule() bool {
	return v.Type().IsCapsule()
}

func (v Val) IsNil() bool {
	v = v.Unmark()
	return v.CtyValue().IsNull()
}

func (v Val) IsUnknown() bool {
	v = v.Unmark()
	return !v.CtyValue().IsKnown()
}

func (v Val) Unmark() Val {
	cv, _ := v.CtyValue().Unmark()
	return Val(cv)
}

func Num(val int) Val {
	return Val(cty.NumberIntVal(int64(val)))
}

func NumFloat(val float64) Val {
	return Val(cty.NumberFloatVal(val))
}

func Str(val string) Val {
	return Val(cty.StringVal(val))
}

func Bool(val bool) Val {
	return Val(cty.BoolVal(val))
}

func List(vals ...Val) Val {
	return Val(cty.ListVal(sliceConv.ToCty(vals)))
}

func Tuple(vals ...Val) Val {
	return Val(cty.TupleVal(sliceConv.ToCty(vals)))
}

func Set(vals ...Val) Val {
	return Val(cty.SetVal(sliceConv.ToCty(vals)))
}

var Nil = Val(cty.NilVal)
var Unknown = Val(cty.DynamicVal)
var UnknownType = Type(cty.DynamicPseudoType)

func (v Val) Hash() int {
	return (v.CtyValue().Hash())
}

func (v Val) GoString() string {
	return (v.CtyValue().GoString())
}

func (v Val) Equals(other Val) Val {
	return Val(v.CtyValue().Equals(cty.Value(other)))
}

func (v Val) NotEqual(other Val) Val {
	return Val(v.CtyValue().NotEqual(cty.Value(other)))
}

func (v Val) Type() Type {
	return Type(v.CtyValue().Type())
}

func (v Val) Is(typ Type) bool {
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

func (v Val) IsList() bool {
	return v.Type().IsList()
}

func (v Val) IsMap() bool {
	return v.Type().IsMap()
}

func (v Val) IsSet() bool {
	return v.Type().IsSet()
}

func (v Val) IsTuple() bool {
	return v.Type().IsTuple()
}

func (v Val) IsPrimitive() bool {
	return v.Type().IsPrimitive()
}

func (v Val) IsObject() bool {
	return v.Type().IsObject()
}

func (v Val) Get(indexOrAttr Val) Val {
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
		return Val(c.GetAttr(idx.AsString()))
	}
	if !idx.HasIndex(idx).True() {
		return Unknown
	}
	return Val(idx.Index(idx))
}

func (v Val) Children() Children {
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
			Key:                   Val(k),
			Value:                 Val(v),
			KeyRepresentsPosition: keyIsPosition,
		})
	}

	return out
}

func (v Val) AsString() string {
	if !v.Is(StrType) {
		return ""
	}
	return v.CtyValue().AsString()
}

func (v Val) AsInt() int {
	return int(v.AsInt64())
}

func (v Val) AsInt64() int64 {
	ret, _ := v.CtyValue().AsBigFloat().Int64()
	return ret
}

func (v Val) AsFloat() float64 {
	ret, _ := v.AsBigFloat().Float64()
	return ret
}

func (v Val) AsBigFloat() *big.Float {
	if !v.Is(NumType) {
		return big.NewFloat(0)
	}
	return v.CtyValue().AsBigFloat()
}

func (v Val) AsBool() bool {
	if !v.Is(BoolType) {
		return false
	}
	return v.CtyValue().True()
}

type Child struct { Key Val; Value Val; KeyRepresentsPosition bool }
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
	return mapConv.ToCty(kv.TryStringMap())
}

func (kv Children) TryStringMap() map[string]Val {
	ret := make(map[string]Val)
	for _, item := range kv {
		if !item.Key.Is(StrType) {
			continue
		}
		ret[item.Key.AsString()] = item.Value
	}
	return ret
}

func (v Val) Search(jsonPath string) []Val {
	p, err := jsonpath.NewPath(jsonPath)
	if err != nil {
		return nil
	}
	return sliceConv.FromCty(p.Search(cty.Value(v)).Values)
}

func (v Val) Len() int {
	return v.CtyValue().LengthInt()
}

func (v Val) MarshalJSON() ([]byte, error) {
	s := json.SimpleJSONValue{cty.Value(v)}
	return s.MarshalJSON()
}

var sliceConv = struct {
	ToCty   func([]Val) []cty.Value
	FromCty func([]cty.Value) []Val
}{
	func(vals []Val) []cty.Value {
		var result = []cty.Value{}
		for _, val := range vals {
			result = append(result, cty.Value(val))
		}
		return result
	},
	func(vals []cty.Value) []Val {
		var result = []Val{}
		for _, val := range vals {
			result = append(result, Val(val))
		}
		return result
	},
}

var mapConv = struct {
	ToCty   func(map[string]Val) map[string]cty.Value
}{
	func(vals map[string]Val) map[string]cty.Value {
		ctyMap := make(map[string]cty.Value)
		for k, v := range vals {
			ctyMap[k] = cty.Value(v)
		}
		return ctyMap
	},
}
