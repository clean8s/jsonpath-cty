// Package jsonpath implements Stefan Goener's JSONPath http://goessner.net/articles/JsonPath/
//
// A jsonpath applies to any JSON decoded data using cty.Value when
// decoded with encoding/json (http://golang.org/pkg/encoding/json/) :
//
//    var bookstore cty.Value
//    err := json.Unmarshal(data, &bookstore)
//    authors, err := jsonpath.Read(bookstore, "$..authors")
//
// A jsonpath expression can be prepared to be reused multiple times :
//
//    allAuthors, err := jsonpath.ParsePath("$..authors")
//    ...
//    var bookstore cty.Value
//    err = json.Unmarshal(data, &bookstore)
//    authors, err := allAuthors(bookstore)
//
// The type of the values returned by the `Read` method or `ParsePath`
// functions depends on the jsonpath expression.
//
// Limitations
//
// No support for subexpressions and filters.
// Strings in brackets must use double quotes.
// It cannot operate on JSON decoded struct fields.
//
package jsonpathcty

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/zclconf/go-cty/cty"
	"reflect"
)

// A cty.Path cannot become a Mark because it is not comparable (and cannot be a key).
// ctyPathRef is a plain pointer which can therefore fit inside a cty Mark.
//
// Each cty.Value processed as an immediate or nested child inside a root cty value gets assigned its mark that holds the
// cty.Path of that value in respect to the root.
type ctyPathRef struct { *cty.Path }

func newCtyPathRef(path cty.Path) ctyPathRef {
	return ctyPathRef{&path}
}

type Wrapper struct {
	baseVal cty.Value
	path cty.Path
}

var WrapperType cty.Type
var WrapperVal = func(value cty.Value, path cty.Path) cty.Value {
	return cty.CapsuleVal(WrapperType, &Wrapper{value, path})
}

func init() {
	WrapperType = cty.CapsuleWithOps("Wrapper", reflect.TypeOf(Wrapper{}), &cty.CapsuleOps{
		GoString: func(val interface{}) string {
			return val.(*Wrapper).baseVal.GoString()
		},
		TypeGoString: func(goTy reflect.Type) string {
			return "Wrapper"
		},
		Equals: func(a, b interface{}) cty.Value {
			at, ok := a.(*Wrapper)
			bt, ok2 :=  b.(*Wrapper)
			if ok && ok2 {
				return at.baseVal.Equals(bt.baseVal)
			}
			return cty.False
		},
		RawEquals: func(a, b interface{}) bool {
			at, ok := a.(*Wrapper)
			bt, ok2 :=  b.(*Wrapper)
			if ok && ok2 {
				return at.baseVal.Equals(bt.baseVal).True()
			}
			return false
		},
		ConversionFrom: nil,
		ConversionTo:   nil,
		ExtensionData:  nil,
	})
}

func GetWrapper(value cty.Value) *Wrapper {
	if value.Type().Equals(WrapperType) {
		w, ok := value.EncapsulatedValue().(*Wrapper)
		if ok {
			return w
		}
	}
	return nil
}

func MaybeGet(value cty.Value) cty.Value {
	w := GetWrapper(value)
	if w != nil {
		return w.baseVal
	}
	return value
}

// Creates a JSONPath from a source string
// which can be used to manipulate with cty data structures.
//
// Example:
//   NewPath("$.servers..racks[0]")
func NewPath(path string) JSONPath {
	return JSONPath{path}
}

// Replaces nested values inside a cty.Value targeted by a JSON path.
//
// Example:
//   newLargeDoc := ReplaceByPath(largeDoc, "$.x.y", newY)
//
// Returns a new (immutable) version of the first argument that has the changes applied.
func ReplaceByPath(wholeDocument cty.Value, targetPath string, newValue cty.Value) (cty.Value, error){
	_, target, err := NewPath(targetPath).Evaluate(wholeDocument)
	if err != nil {
		return cty.NilVal, err
	}
	return cty.Transform(wholeDocument, func(path cty.Path, value cty.Value) (cty.Value, error) {
		if cty.NewPathSet(target...).Has(path) {
			return newValue, nil
		}
		return value, nil
	})
}

func T(v cty.Value, paths *[]cty.Path) (cty.Value) {
	v, _ = cty.Transform(v, func(path cty.Path, value cty.Value) (cty.Value, error) {
		g := GetWrapper(value)
		if g != nil {
			*paths = append(*paths, g.path)
			return T(g.baseVal, paths), nil
		}
		return value, nil
	})
	return v
}

// Evaluates a JSON Path on some cty.Value. The returned cty.Value may be a primitive or a tuple containing
// many different matches (depending on the operators used).
//
// The second return value is a slice of paths which should point to the matched values.
//
// If the result is a primitive you should expect:
//   len(paths) == 1
//   assuming "$.x", path[0] == Path{Index('x')}
// If the result is multiple-valued, it'll get stored as a cty.Tuple and you should expect:
//   resTuple.Length() == len(paths)
//   assuming $["x","y"], paths[0] == Path{Index('x')} && paths[1] == Path{Index('y')}
func (path JSONPath) Evaluate(value cty.Value) (cty.Value, []cty.Path, error) {
	value, _ = cty.Transform(value, func(path cty.Path, value cty.Value) (cty.Value, error) {
		return WrapperVal(value, path), nil
	})

	p := newScanner(path.source)
	if err := p.parse(); err != nil {
		return cty.NilVal, nil, err
	}

	actions := p.actions
	result, err := actions.next(value, value)
	if err != nil {
		return cty.NilVal, nil, err
	}
	v, pathMarks := result.UnmarkDeepWithPaths()
	var paths = []cty.Path{}
	v = T(v, &paths)
	return v, paths, err
	//paths := []cty.Path{}
	var globalP *cty.Path
	for _, item := range pathMarks {
		for m, _ := range item.Marks {
			if pv, ok := m.(ctyPathRef); ok {
				_, err := item.Path.Apply(v)
				if err != nil {
					continue
				}
				if len(item.Path) == 0 {
					globalP = pv.Path
				}
				if len(item.Path) == 1 {
					paths = append(paths, *pv.Path)
				}
			}
		}
	}
	if len(paths) > 0 {
		return v, paths, nil
	}
	if globalP != nil {
		return v, []cty.Path{*globalP}, nil
	}
	return v, nil, nil
}

// JSONPath holds the source of a JSON path and provides
// the methods for manipulating with cty.Value by JSON paths.
type JSONPath struct {
	source string
}

// an iteratorMark is a special type used to Mark() values
// that are produced by a JSONPath construct rather than actual user provided cty.Value
// for example, "$.x..recursive" emits a tuple of all values in '$.x', but such tuple is not 'naturally' present inside the JSON
type iteratorMarkType string
var iteratorMark iteratorMarkType = iteratorMarkType("iterable.container")

// actionFunc applies a transformation to current value (possibility using root)
// then applies the next action from actions (using next()) to the output of the transformation
type actionFunc func(root, current cty.Value, actions actions) (cty.Value, error)

// a list of action functions to apply one after the other
type actions []actionFunc

// next applies the next action function
func (a actions) next(r, c cty.Value) (cty.Value, error) {
	return a[0](r, MaybeGet(c), a[1:])
}

// call applies the next action function without taking it out
func (a actions) call(r, c cty.Value) (cty.Value, error) {
	return a[0](r, c, a)
}

type exprFunc func(r, c cty.Value) (cty.Value, error)

type searchResults []cty.Value

type parser struct {
	scanner scanner.Scanner
	path    string
	actions actions
}

func newScanner(path string) *parser {
	return &parser{path: path}
}

func (p *parser) scan() rune {
	p.scanner.Error = func(s *scanner.Scanner, msg string) {}
	return p.scanner.Scan()
}

func (p *parser) text() string {
	return p.scanner.TokenText()
}

func (p *parser) column() int {
	return p.scanner.Position.Column
}

func (p *parser) peek() rune {
	return p.scanner.Peek()
}

func (p *parser) add(action actionFunc) {
	p.actions = append(p.actions, action)
}

func (p *parser) parse() error {
	p.scanner.Init(strings.NewReader(p.path))
	if p.scan() != '$' {
		return errors.New("path must start with a '$'")
	}
	return p.parsePath()
}

func (p *parser) parsePath() (err error) {
	for err == nil {
		switch p.scan() {
		case '.':
			p.scanner.Mode = scanner.ScanIdents
			switch p.scan() {
			case scanner.Ident:
				err = p.parseObjAccess()
			case '*':
				err = p.prepareWildcard()
			case '.':
				err = p.parseDeep()
			default:
				err = fmt.Errorf("expected JSON child identifier after '.' at %d", p.column())
			}
		case '[':
			err = p.parseBracket()
		case scanner.EOF:
			// the end, add a last func that just return current node
			p.add(func(r, c cty.Value, a actions) (cty.Value, error) { return c, nil })
			return nil
		default:
			err = fmt.Errorf("unexpected token %s at %d", p.text(), p.column())
		}
	}
	return
}

// handles "$.attr": a plain attribute access.
func (p *parser) parseObjAccess() error {
	ident := p.text()
	column := p.scanner.Position.Column
	p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
		c, _ = c.Unmark()
		if c.Type().IsObjectType() {
			if !c.Type().HasAttribute(ident) {
				return cty.NilVal, fmt.Errorf("attribute '%s' doesn't exist at %d", ident, column)
			}
			attr := c.GetAttr(ident)
			return a.next(r, MaybeGet(attr))
		}
		if c.CanIterateElements() {
			identCty := cty.StringVal(ident)
			if !c.HasIndex(identCty).True() {
				return cty.NilVal, fmt.Errorf("attribute '%s' doesn't exist at %d", ident, column)
			}
			attr := MaybeGet(c.Index(identCty))
			return a.next(r, attr)
		}
		return cty.NilVal, fmt.Errorf("not supporting attributes at %d", column)
	})
	return nil
}

// handles ".*": the wildcard operator. it matches all immediate children of an array/object.
func (p *parser) prepareWildcard() error {
	p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
		out := []cty.Value{}
		c, _ = c.Unmark()
		if c.CanIterateElements() {
			iter := c.ElementIterator()
			for iter.Next() {
				_, v := iter.Element()
				v, err := a.next(r, v)
				if err != nil {
					continue
				}
				if v.HasMark(iteratorMark) {
					v, _ := v.Unmark()
					out = append(out, v.AsValueSlice()...)
				} else {
					out = append(out, v)
				}
			}
		} else {
			return cty.NilVal, fmt.Errorf("cannot iterate %s", c.GoString())
		}
		return cty.TupleVal(out).Mark(iteratorMark), nil
	})
	return nil
}

// handles deep/recursive scans with the ".." syntax
func (p *parser) parseDeep() (err error) {
	p.scanner.Mode = scanner.ScanIdents
	switch p.scan() {
	case scanner.Ident:
		p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
			return recSearchParent(r, c, a, searchResults{}), nil
		})
		return p.parseObjAccess()
	case '[':
		p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
			return recSearchParent(r, c, a, searchResults{}), nil
		})
		return p.parseBracket()
	case '*':
		p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
			return recSearchChildren(r, c, a, searchResults{}), nil
		})
		p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
			return a.next(r, c)
		})
		return nil
	case scanner.EOF:
		return fmt.Errorf("cannot end with a scan '..' at %d", p.column())
	default:
		return fmt.Errorf("unexpected token '%s' after deep search '..' at %d",
			p.text(), p.column())
	}
}

// bracket contains filter, wildcard or array access
func (p *parser) parseBracket() error {
	if p.peek() == '?' {
		return p.parseFilter()
	} else if p.peek() == '*' {
		p.scan() // eat *
		if p.scan() != ']' {
			return fmt.Errorf("expected closing bracket after [* at %d", p.column())
		}
		return p.prepareWildcard()
	}
	return p.parseArray()
}

// array contains either a union [,,,], a slice [::] or a single element.
// Each element can be an int, a string or an expression.
// TODO optimize map/array access (by detecting the type of indexes)
func (p *parser) parseArray() error {
	Num := func(num int) cty.Value {
		return cty.NumberIntVal(int64(num))
	}
	var indexes []cty.Value // string, int or exprFunc
	var mode string         // slice or union
	p.scanner.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts
parse:
	for {
		// parse value
		switch p.scan() {
		case scanner.Int:
			index, err := strconv.Atoi(p.text())
			if err != nil {
				return fmt.Errorf("%s at %d", err.Error(), p.column())
			}
			indexes = append(indexes, Num(index))
		case '-':
			if p.scan() != scanner.Int {
				return fmt.Errorf("expect an int after the minus '-' sign at %d", p.column())
			}
			index, err := strconv.Atoi(p.text())
			if err != nil {
				return fmt.Errorf("%s at %d", err.Error(), p.column())
			}
			indexes = append(indexes, Num(-index))
		case scanner.Ident:
			indexes = append(indexes, cty.StringVal(p.text()))
		case scanner.String:
			s, err := strconv.Unquote(p.text())
			if err != nil {
				return fmt.Errorf("bad string %s at %d", err, p.column())
			}
			indexes = append(indexes, cty.StringVal(s))
		case '(':
			return fmt.Errorf("cant handle (")
			// filter, err := p.parseExpression()
			// if err != nil {
			// 	return err
			// }
			// indexes = append(indexes, filter)
		case ':': // when slice value is omitted
			if mode == "" {
				mode = "slice"
				indexes = append(indexes, Num(0))
			} else if mode == "slice" {
				indexes = append(indexes, Num(0))
			} else {
				return fmt.Errorf("unexpected ':' after %s at %d", mode, p.column())
			}
			continue // skip separator parsing, it's done
		case ']': // when slice value is omitted
			if mode == "slice" {
				indexes = append(indexes, Num(0))
			} else if len(indexes) == 0 {
				return fmt.Errorf("expected at least one key, index or expression at %d", p.column())
			}
			break parse
		case scanner.EOF:
			return fmt.Errorf("unexpected end of path at %d", p.column())
		default:
			return fmt.Errorf("unexpected token '%s' at %d", p.text(), p.column())
		}
		// parse separator
		switch p.scan() {
		case ',':
			if mode == "" {
				mode = "union"
			} else if mode != "union" {
				return fmt.Errorf("unexpeted ',' in %s at %d", mode, p.column())
			}
		case ':':
			if mode == "" {
				mode = "slice"
			} else if mode != "slice" {
				return fmt.Errorf("unexpected ':' in %s at %d", mode, p.column())
			}
		case ']':
			break parse
		case scanner.EOF:
			return fmt.Errorf("unexpected end of path at %d", p.column())
		default:
			return fmt.Errorf("unexpected token '%s' at %d", p.text(), p.column())
		}
	}
	if mode == "slice" {
		if len(indexes) > 3 {
			return fmt.Errorf("bad range syntax [start:end:step] at %d", p.column())
		}
		p.add(prepareSlice(indexes, p.column()))
	} else if len(indexes) == 1 {
		p.add(prepareIndex(indexes[0], p.column()))
	} else {
		p.add(prepareUnion(indexes, p.column()))
	}
	return nil
}

func (p *parser) parseFilter() error {
	return errors.New("Filters are not (yet) implemented")
}

func (p *parser) parseExpression() (exprFunc, error) {
	return nil, errors.New("Expression are not (yet) implemented")
}

func recSearchParent(r, c cty.Value, a actions, acc searchResults) cty.Value {
	if v, err := a.next(r, c); err == nil {
		v = MaybeGet(v)
		if v.HasMark(iteratorMark) {
			v, _ = v.Unmark()
			acc = append(acc, v.AsValueSlice()...)
		} else {
			acc = append(acc, v)
		}
	}
	return recSearchChildren(r, c, a, acc).Mark(iteratorMark)
}

func recSearchChildren(r, c cty.Value, a actions, acc searchResults) cty.Value {
	if c.HasMark(iteratorMark) {
		c, _ = c.Unmark()
	}
	c = MaybeGet(c)
	if c.CanIterateElements() {
		it := c.ElementIterator()
		for it.Next() {
			_, v := it.Element()
			v, _ = recSearchParent(r, v, a, acc).Unmark()
			acc = v.AsValueSlice()
		}
	}
	return cty.TupleVal(acc).Mark(iteratorMark)
}

// handles "[x]" operator for indexing where x is a Number.
func prepareIndex(index cty.Value, column int) actionFunc {
	return func(r, c cty.Value, a actions) (cty.Value, error) {
		c, _ = c.Unmark()
		c = MaybeGet(c)
		if c.CanIterateElements() {
			it := c.ElementIterator()
			for it.Next() {
				k, v := it.Element()
				if k.Equals(index).True() {
					return a.next(r, v)
				}
			}
			return cty.NilVal, fmt.Errorf("not found at %d", column)
		}
		return cty.NilVal, fmt.Errorf("not iterable at %d", column)
	}
}

var ctyOne = cty.NumberIntVal(1)

// converts a cty.Value to an untyped int.
func getInt(v cty.Value) int {
	ctyInt64, _ := v.AsBigFloat().Int64()
	return int(ctyInt64)
}

// handles slice syntax "[low : high : increment]" which is an extension of the index operator.
// supports negative indexing.
func prepareSlice(indexes []cty.Value, column int) actionFunc {
	return func(r, c cty.Value, a actions) (cty.Value, error) {
		for _, v := range indexes {
			// make sure indexes has Numbers only
			if len(v.Type().TestConformance(cty.Number)) != 0 {
				return cty.NilVal, fmt.Errorf("not a number: %s", v.GoString())
			}
		}
		// slices should look like [idxL : idxR : increment]
		idxL, idxR := getInt(indexes[0]), getInt(indexes[1])

		// all marks must be removed before iterating a slice.
		slice, _ := c.Unmark()
		slice = MaybeGet(slice)

		// support negative values
		idxL = negmax(idxL, slice.LengthInt())
		if idxR == 0 {
			idxR = slice.LengthInt()
		} else {
			idxR = negmax(idxR, slice.LengthInt())
		}

		// increment is "+1" by default, unless there's a third argument.
		var increment = 1
		if len(indexes) == 3 {
			increment = getInt(indexes[2])
		}

		if slice.CanIterateElements() {
			slice := slice.AsValueSlice()

			out := []cty.Value{}
			if increment < 0 {

				// negative increments need a reverse loop
				// instead of [low, high) you need to start at (high - 1), down to (low)

				for i := idxR - 1; i >= idxL; i += increment {
					v, err := a.next(r, slice[i])
					v = MaybeGet(v)
					if err != nil {
						continue
					}
					if v.HasMark(iteratorMark) {
						v, _ = v.Unmark()
						out = append(out, v.AsValueSlice()...)
					} else {
						out = append(out, v)
					}
				}
			} else {
				for i := idxL; i < idxR; i += increment {
					v, err := a.next(r, slice[i])
					v = MaybeGet(v)
					if err != nil {
						continue
					}
					if v.HasMark(iteratorMark) {
						v, _ = v.Unmark()
						out = append(out, v.AsValueSlice()...)
					} else {
						out = append(out, v)
					}
				}
			}
			return cty.TupleVal(out).Mark(iteratorMark), nil
		}
		return cty.NilVal, fmt.Errorf("cannot iterate %s", c.GoString())
	}
}

// a union merges the elements of two objects
// this handles the feature $["x", "y", "z", ...]
func prepareUnion(indexes []cty.Value, column int) actionFunc {
	return func(r, c cty.Value, a actions) (cty.Value, error) {
		output := []cty.Value{}
		c, _ = c.Unmark()
		c = MaybeGet(c)
		if c.CanIterateElements() {
			for _, key := range indexes {
				it := c.ElementIterator()
				for it.Next() {
					k, v := it.Element()
					v = MaybeGet(v)
					if !k.Equals(key).True() {
						continue
					}
					v, err := a.next(r, v)
					if err != nil {
						return cty.NilVal, err
					}
					if v.HasMark(iteratorMark) {
						v, _ = v.Unmark()
						output = append(output, v.AsValueSlice()...)
					} else {
						output = append(output, v)
					}
				}
			}
			return cty.TupleVal(output).Mark(iteratorMark), nil
		}
		return cty.NilVal, fmt.Errorf("not iterable: %s", c.GoString())
	}
}

func negmax(n, max int) int {
	if n < 0 {
		n = max + n
		if n < 0 {
			n = 0
		}
	} else if n > max {
		return max
	}
	return n
}