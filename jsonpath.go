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
//    allAuthors, err := jsonpath.Prepare("$..authors")
//    ...
//    var bookstore cty.Value
//    err = json.Unmarshal(data, &bookstore)
//    authors, err := allAuthors(bookstore)
//
// The type of the values returned by the `Read` method or `Prepare`
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
)

// Read a path from a decoded JSON array or object ([]cty.Value or map[string]cty.Value)
// and returns the corresponding value or an error.
//
// The returned value type depends on the requested path and the JSON value.
func Read(value cty.Value, path string) (cty.Value, error) {
	filter, err := Prepare(path)
	if err != nil {
		return cty.NilVal, err
	}
	return filter(value)
}

// Prepare will parse the path and return a filter function that can then be applied to decoded JSON values.
func Prepare(path string) (FilterFunc, error) {
	p := newScanner(path)
	if err := p.parse(); err != nil {
		return nil, err
	}
	return p.prepareFilterFunc(), nil
}

// FilterFunc applies a prepared json path to a JSON decoded value
type FilterFunc func(cty.Value) (cty.Value, error)

// short variables
// p: the parser context
// r: root node => @
// c: current node => $
// a: the list of actions to apply next
// v: value

// actionFunc applies a transformation to current value (possibility using root)
// then applies the next action from actions (using next()) to the output of the transformation
type actionFunc func(r, c cty.Value, a actions) (cty.Value, error)

// a list of action functions to apply one after the other
type actions []actionFunc

// next applies the next action function
func (a actions) next(r, c cty.Value) (cty.Value, error) {
	return a[0](r, c, a[1:])
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

func (p *parser) prepareFilterFunc() FilterFunc {
	actions := p.actions
	return func(value cty.Value) (cty.Value, error) {
		result, err := actions.next(value, value)
		result, _ = result.UnmarkDeep()
		return result, err
	}
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

func (p *parser) parseObjAccess() error {
	ident := p.text()
	column := p.scanner.Position.Column
	p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
		if c.Type().IsObjectType() {
			if !c.Type().HasAttribute(ident) {
				return cty.NilVal, fmt.Errorf("attribute '%s' doesn't exist at %d", ident, column)
			}
			attr := c.GetAttr(ident)
			return a.next(r, attr)
		}
		if c.CanIterateElements() {
			identCty := cty.StringVal(ident)
			if !c.HasIndex(identCty).True() {
				return cty.NilVal, fmt.Errorf("attribute '%s' doesn't exist at %d", ident, column)
			}
			attr := c.Index(identCty)
			return a.next(r, attr)
		}
		return cty.NilVal, fmt.Errorf("not supporting attributes at %d", column)
	})
	return nil
}

func (p *parser) prepareWildcard() error {
	p.add(func(r, c cty.Value, a actions) (cty.Value, error) {
		out := []cty.Value{}
		if c.CanIterateElements() {
			iter := c.ElementIterator()
			for iter.Next() {
				_, v := iter.Element()
				v, err := a.next(r, v)
				if err != nil {
					continue
				}
				if v.HasMark(result) {
					v, _ = v.Unmark()
					out = append(out, v.AsValueSlice()...)
				} else {
					out = append(out, v)
				}
			}
		} else {
			return cty.NilVal, fmt.Errorf("cannot iterate %s", c.GoString())
		}
		return cty.TupleVal(out).Mark(result), nil
	})
	return nil
}

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

type resultMark int

var result resultMark = resultMark(1)

func recSearchParent(r, c cty.Value, a actions, acc searchResults) cty.Value {
	if v, err := a.next(r, c); err == nil {
		if v.HasMark(result) {
			v, _ = v.Unmark()
			acc = append(acc, v.AsValueSlice()...)
		} else {
			acc = append(acc, v)
		}
	}
	return recSearchChildren(r, c, a, acc).Mark(result)
}

func recSearchChildren(r, c cty.Value, a actions, acc searchResults) cty.Value {
	if c.CanIterateElements() {
		it := c.ElementIterator()
		for it.Next() {
			_, v := it.Element()
			v, _ = recSearchParent(r, v, a, acc).Unmark()
			acc = v.AsValueSlice()
		}
	}
	return cty.TupleVal(acc).Mark(result)
}

func prepareIndex(index cty.Value, column int) actionFunc {
	return func(r, c cty.Value, a actions) (cty.Value, error) {
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

func getInt(v cty.Value) int {
	i6, _ := v.AsBigFloat().Int64()
	return int(i6)
}

func prepareSlice(indexes []cty.Value, column int) actionFunc {
	return func(r, c cty.Value, a actions) (cty.Value, error) {
		for _, v := range indexes {
			if len(v.Type().TestConformance(cty.Number)) != 0 {
				return cty.NilVal, fmt.Errorf("not a number: %s", v.GoString())
			}
		}
		idxL, idxR := getInt(indexes[0]), getInt(indexes[1])
		idxL = negmax(idxL, c.LengthInt())
		if idxR == 0 {
			idxR = c.LengthInt()
		} else {
			idxR = negmax(idxR, c.LengthInt())
		}

		var idxM = 1
		if len(indexes) == 3 {
			idxM = getInt(indexes[2])
		}
		if c.CanIterateElements() {
			slice := c.AsValueSlice()

			out := []cty.Value{}
			if idxM < 0 {
				for i := idxR - 1; i >= idxL; i += idxM {
					v, err := a.next(r, slice[i])
					if err != nil {
						continue
					}
					if v.HasMark(result) {
						v, _ = v.Unmark()
						out = append(out, v.AsValueSlice()...)
					} else {
						out = append(out, v)
					}
				}
			} else {
				for i := idxL; i < idxR; i += idxM {
					v, err := a.next(r, slice[i])
					if err != nil {
						continue
					}
					if v.HasMark(result) {
						v, _ = v.Unmark()
						out = append(out, v.AsValueSlice()...)
					} else {
						out = append(out, v)
					}
				}
			}
			return cty.TupleVal(out).Mark(result), nil
		}
		return cty.NilVal, fmt.Errorf("cannot iterate %s", c.GoString())
	}
}

func prepareUnion(indexes []cty.Value, column int) actionFunc {
	return func(r, c cty.Value, a actions) (cty.Value, error) {
		output := []cty.Value{}
		if c.CanIterateElements() {
			for _, key := range indexes {
				it := c.ElementIterator()
				for it.Next() {
					k, v := it.Element()
					if !k.Equals(key).True() {
						continue
					}
					v, err := a.next(r, v)
					if err != nil {
						return cty.NilVal, err
					}
					if v.HasMark(result) {
						v, _ = v.Unmark()
						output = append(output, v.AsValueSlice()...)
					} else {
						output = append(output, v)
					}
				}
			}
			return cty.TupleVal(output).Mark(result), nil
		}
		return cty.NilVal, fmt.Errorf("not iterable: %s", c.GoString())
		// if len(output) == 1 {
		// 	return output[0], nil
		// }
		// return cty.TupleVal(output), nil
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