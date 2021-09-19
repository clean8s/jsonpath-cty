package jsonpathcty

import (
	"github.com/zclconf/go-cty/cty"
	"bytes"
	"fmt"
	"strconv"
	"github.com/zclconf/go-cty/cty/json"
)

var globalCache []cty.Path

func DeepCopyPath(path cty.Path) *cty.Path {
	p := cty.Path{}
	for _, step := range path.Copy() {
		switch ts := step.(type) {
		case cty.GetAttrStep:
			p = p.GetAttr(ts.Name)
		case cty.IndexStep:
			J := json.SimpleJSONValue{ts.Key}
			Jstr, _ := J.MarshalJSON()
			J.UnmarshalJSON(Jstr)

			p = p.Index(J.Value)
		}
	}
	globalCache = append(globalCache, p)
	return &globalCache[len(globalCache)-1]
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
