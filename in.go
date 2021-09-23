package peek

import (
	"reflect"
	"math/big"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/set"
	"github.com/zclconf/go-cty/cty/gocty"
	"fmt"
)

type StructPath struct {
	Path cty.Path
	FieldNames []string
}

type TypeTransformer func(typ Type, path []Value) (newTyp Type, continueWalk bool)

func Transform(t Type, path []Value, transformer TypeTransformer) Type {
	newType, continueWalk := transformer(t, path)
	if !continueWalk {
		return newType
	} else {
		t = newType
	}

	if t.CtyType().IsPrimitiveType() {
		return t
	} else if t.IsObject() {
		M := t.CtyType().AttributeTypes()
		for k, v := range M {
			M[k] = Transform(Type(v), append(path, Str(k)), transformer).CtyType()
		}
		return Type(cty.Object(M))
	} else if t.IsTuple() {
		types := []cty.Type{}
		for i, item := range t.CtyType().TupleElementTypes() {
			types = append(types, cty.Type(Transform(Type(item), append(path, Num(i)), transformer)))
		}
		return Type(cty.Tuple(types))
	} else {
		fn := cty.Set
		if t.IsSet() {
			fn = cty.Set
		}
		if t.IsList() {
			fn = cty.List
		}
		if t.IsMap() {
			fn = cty.Map
		}

		el := t.ElementType()
		fnType := Transform(el, append(path, Unknown), transformer).CtyType()
		return Type(fn(fnType))
	}
	return UnknownType
}

func (t Type) IsCapsule() bool {
	return t.CtyType().IsCapsuleType()
}

func (t Type) ElementType() Type {
	return Type(t.CtyType().ElementType())
}

func New(gv interface{}) Value {
	rt := reflect.TypeOf(gv)
	var path cty.Path
	var conv []StructPath = make([]StructPath, 0)
	res, err := impliedType(rt, path, &conv)
	for _, item := range conv {
		fmt.Println(FormatCtyPath(item.Path), item.FieldNames)
	}
	if err != nil {
		panic(err)
	}
	ct, _ := gocty.ToCtyValue(gv, res)
	ct, err = cty.Transform(ct, func(path cty.Path, value cty.Value) (cty.Value, error) {
		for _, spath := range conv {
			if path.Equals(spath.Path) {
				it := value.ElementIterator()
				namedFields := make(map[string]cty.Value)
				i := 0
				for it.Next() {
					_, v := it.Element()
					namedFields[spath.FieldNames[i]] = v
					i++
				}
				return cty.ObjectVal(namedFields), nil
			}
		}
		return value, nil
	})
	return Value(ct)
}

func impliedType(rt reflect.Type, path cty.Path, conv *[]StructPath) (cty.Type, error) {
	switch rt.Kind() {

	case reflect.Ptr:
		return impliedType(rt.Elem(), path, conv)

	// Primitive types
	case reflect.Bool:
		return cty.Bool, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cty.Number, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return cty.Number, nil
	case reflect.Float32, reflect.Float64:
		return cty.Number, nil
	case reflect.String:
		return cty.String, nil

	// Collection types
	case reflect.Slice:
		path := append(path, cty.IndexStep{Key: cty.UnknownVal(cty.Number)})
		ety, err := impliedType(rt.Elem(), path, conv)
		if err != nil {
			return cty.NilType, err
		}
		return cty.List(ety), nil
	case reflect.Map:
		if !stringType.AssignableTo(rt.Key()) {
			return cty.NilType, path.NewErrorf("no cty.Type for %s (must have string keys)", rt)
		}
		path := append(path, cty.IndexStep{Key: cty.UnknownVal(cty.String)})
		ety, err := impliedType(rt.Elem(), path, conv)
		if err != nil {
			return cty.NilType, err
		}
		return cty.Map(ety), nil

	// Structural types
	case reflect.Struct:
		return impliedStructType(rt, path, conv)

	default:
		return cty.NilType, path.NewErrorf("no cty.Type for %s", rt)
	}
}

func impliedStructType(rt reflect.Type, path cty.Path, conv *[]StructPath) (cty.Type, error) {
	if valueType.AssignableTo(rt) {
		// Special case: cty.Value represents cty.DynamicPseudoType, for
		// type conformance checking.
		return cty.DynamicPseudoType, nil
	}

	numFields := rt.NumField()
	vals := make([]cty.Type, 0)

	fieldNames := []string{}
	{
		// Temporary extension of path for attributes
		path := append(path, nil)

		for i := 0; i < numFields; i++ {
			field := rt.Field(i)
			k := field.Name
			if field.Tag.Get("cty") != "" {
				k = field.Tag.Get("cty")
			}
			fieldNames = append(fieldNames, k)
			path[len(path)-1] = cty.GetAttrStep{Name: k}

			ft := field.Type
			aty, err := impliedType(ft, path, conv)
			if err != nil {
				return cty.NilType, err
			}

			vals = append(vals, aty)
		}
	}

	spath := StructPath{
		Path:       path.Copy(),
		FieldNames: fieldNames,
	}
	*conv = append(*conv, spath)
	return cty.Tuple(vals), nil
}

var valueType = reflect.TypeOf(cty.Value{})
var typeType = reflect.TypeOf(cty.Type{})

var setType = reflect.TypeOf(set.Set{})
var bigFloatType = reflect.TypeOf(big.Float{})
var bigIntType = reflect.TypeOf(big.Int{})
var emptyInterfaceType = reflect.TypeOf(interface{}(nil))
var stringType = reflect.TypeOf("")