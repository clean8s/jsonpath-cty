package peek

import (
	"strings"
	"fmt"
	"github.com/zclconf/go-cty/cty"
	"bytes"
	"strconv"
)

func (c Child) String() string {
	return fmt.Sprintf("Child(key=%s, val=%s)", c.Key, c.Value)
}

func (v Val) CtyValue() cty.Value {
	return cty.Value(v)
}

func (v Val) CtyType() cty.Type {
	return cty.Type(v.Type())
}

func (v Val) String() string {
	return v.stringer("")
}

func FormatCtyPath(path cty.Path) string {
	var buf bytes.Buffer
	for _, step := range path {
		switch ts := step.(type) {
		case cty.GetAttrStep:
			fmt.Fprintf(&buf, ".%s", ts.Name)
		case cty.IndexStep:
			buf.WriteByte('[')
			key := ts.Key
			keyTy := key.Type()
			switch {
			case key.IsNull():
				buf.WriteString("null")
			case !key.IsKnown():
				buf.WriteString("(not yet known)")
			case keyTy == cty.Number:
				bf := key.AsBigFloat()
				buf.WriteString(bf.Text('g', -1))
			case keyTy == cty.String:
				buf.WriteString(strconv.Quote(key.AsString()))
			default:
				buf.WriteString("...")
			}
			buf.WriteByte(']')
		}
	}
	return buf.String()
}

func (v Val) stringer(prefix string) string {
	val, _ := v.CtyValue().UnmarkDeep()
	if val == cty.NilVal || val.IsNull(){
		return "nil"
	}
	if !val.IsKnown() {
		return "unknown"
	}
	if !val.Type().IsObjectType() {
		if val.Type().IsPrimitiveType() {
			if val.Type().Equals(cty.String) {
				return fmt.Sprintf("%#v", val.AsString())
			}
			if val.Type().Equals(cty.Number) {
				return fmt.Sprintf("%s", val.AsBigFloat().String())
			}
			if val.Type().Equals(cty.Bool) {
				if val.True() {
					return "true"
				}
				return "false"
			}
			return v.CtyValue().GoString()
		}
		if val.CanIterateElements() {
			it := val.ElementIterator()
			ret := []string{}
			for it.Next() {
				k, v := it.Element()
				key := ""
				if val.Type().IsMapType() {
					key = fmt.Sprintf("%v: ", Val(k).stringer(prefix))
				}
				ret = append(ret, fmt.Sprintf("%s%v", key, Val(v).stringer(prefix)))
			}
			sepL, sepR := "[", "]"
			return fmt.Sprintf("%s%s%s", sepL, strings.Join(ret, ", "), sepR)
		}
	} else {
		it := val.ElementIterator()
		ret := []string{}
		for it.Next() {
			k, v := it.Element()
			ret = append(ret, fmt.Sprintf("%s%v = %v", prefix + "  ", k.AsString(), Val(v).stringer(prefix + "  ")))
		}
		return "object(\n" + strings.Join(ret, "\n") + "\n)"
	}

	return v.CtyValue().GoString()
}

func (v Type) String() string {
	t := cty.Type(v)
	if t.IsPrimitiveType() {
		return strings.ReplaceAll(t.FriendlyName(), "cty.", "")
	}
	if t.IsObjectType() {
		out := "object(\n"
		for k, v := range t.AttributeTypes() {
			out += fmt.Sprintf("  %v: %s\n", k, Type(v).String())
		}
		return out + ")\n"
	}
	if t.IsListType() {
		return "list(" + Type(t.ElementType()).String() + ")"
	}
	if t.IsMapType() {
		return "map(" + Type(t.ElementType()).String() + ")"
	}
	if t.IsTupleType() {
		tupTypes := []string{}
		for _, item := range t.TupleElementTypes() {
			tupTypes = append(tupTypes, Type(item).String())
		}
		return "tuple(" + strings.Join(tupTypes, ", ") + ")"
	}
	if t.IsSetType() {
		return "set(" + Type(t.ElementType()).String() + ")"
	}
	if t == cty.DynamicPseudoType {
		return "dynamic"
	}
	if t == cty.NilType {
		return "nil"
	}
	return t.GoString()
}
