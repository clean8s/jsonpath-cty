package jsonpathcty

import (
	"strconv"
	"strings"
	"github.com/zclconf/go-cty/cty"
	"io"
)

type JSONPath struct {
	parts []string
}

// Creates a JSONPath from a string named "path".
// Returns an error if the path itself contains syntax errors.
func NewPath(path string) (JSONPath, error) {
	parts, err := parseJsonPath(path)
	if err != nil {
		return JSONPath{}, err
	}
	return JSONPath{parts}, nil
}

// Creates a path like NewPath, but doesn't return an error.
// When applied to a cty.Value it may silently fail or panic.
func MustNewPath(path string) JSONPath {
	pathVal, _ := NewPath(path)
	return pathVal
}

// Applies the JSONPath to a cty.Value, returning the result or an error.
func (p JSONPath) Apply(value cty.Value) ([]cty.Value, error) {
	return evaluateCommands(value, p.parts)
}

// Just like JSONPath.Apply() except it doesn't return an error.
func (p JSONPath) MustApply(value cty.Value) []cty.Value {
	result, _ := p.Apply(value)
	return result
}

// Checks whether the cty.Value contains something JSON would consider
// an array. Currently: cty.List and cty.Tuple
func isArray(val cty.Value) bool {
	if val.Type().IsListType() || val.Type().IsTupleType() {
		return true
	}
	return false
}

// Checks if cty holds something JSON would consider an Object.
// Currently: cty.Map and cty.Object
func isObject(val cty.Value) bool {
	if val.Type().IsMapType() || val.Type().IsObjectType() {
		return true
	}
	return false
}


// Creates a list of nodes containing the immediate children of 'node'
// as well as their children and all nested descendents.
func recursiveChildren(node cty.Value) (result []cty.Value) {
	// result = list of node children
	if node.Type().IsListType() {
		result = append(result, node.AsValueSlice()...)
	}

	// temp allocates 'result', and then calls the same function on result itself.
	temp := make([]cty.Value, 0, len(result))
	temp = append(temp, result...)
	for _, el := range result {
		temp = append(temp, recursiveChildren(el)...)
	}
	return temp
}

// parseJsonPath will parse a JSONPath and split it into subpaths called 'commands'.
// Example:
//
// 	result, _ := parseJsonPath("$.store.book[?(@.price < 10)].title")
// 	result == []string{"$", "store", "book", "?(@.price < 10)", "title"}
//
func parseJsonPath(path string) (result []string, err error) {
	buf := newTokenizer([]byte(path))
	result = make([]string, 0)
	const (
		fQuote  = 1 << 0
		fQuotes = 1 << 1
	)
	var (
		c           byte
		start, stop int
		childEnd    = map[byte]bool{dot: true, bracketL: true}
		flag        int
		brackets    int
	)
	for {
		c, err = buf.current()
		if err != nil {
			break
		}
	parseSwitch:
		switch true {
		case c == dollar || c == at:
			result = append(result, string(c))
		case c == dot:
			start = buf.index
			c, err = buf.next()
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil {
				break
			}
			if c == dot {
				result = append(result, "..")
				buf.index--
				break
			}
			err = buf.skipAny(childEnd)
			stop = buf.index
			if err == io.EOF {
				err = nil
				stop = buf.length
			} else {
				buf.index--
			}
			if err != nil {
				break
			}
			if start+1 < stop {
				result = append(result, string(buf.data[start+1:stop]))
			}
		case c == bracketL:
			_, err = buf.next()
			if err != nil {
				return nil, buf.errorEOF()
			}
			brackets = 1
			start = buf.index
			for ; buf.index < buf.length; buf.index++ {
				c = buf.data[buf.index]
				switch c {
				case quote:
					if flag&fQuotes == 0 {
						if flag&fQuote == 0 {
							flag |= fQuote
						} else if !buf.backslash() {
							flag ^= fQuote
						}
					}
				case quotes:
					if flag&fQuote == 0 {
						if flag&fQuotes == 0 {
							flag |= fQuotes
						} else if !buf.backslash() {
							flag ^= fQuotes
						}
					}
				case bracketL:
					if flag == 0 && !buf.backslash() {
						brackets++
					}
				case bracketR:
					if flag == 0 && !buf.backslash() {
						brackets--
					}
					if brackets == 0 {
						result = append(result, string(buf.data[start:buf.index]))
						break parseSwitch
					}
				}
			}
			return nil, buf.errorEOF()
		default:
			return nil, buf.errorSymbol()
		}
		err = buf.step()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
	}
	return
}

// Evaluates a Reverse Polish expression on a cty.Value
func eval(node cty.Value, expression rpn, cmd string) (result cty.Value, err error) {
	var (
		stack    = make([]cty.Value, 0)
		slice    []cty.Value
		temp     cty.Value
		fn       Function
		op       Operation
		ok       bool
		size     int
		commands []string
		bstr     []byte
	)
	for _, exp := range expression {
		size = len(stack)
		if fn, ok = functions[exp]; ok {
			if size < 1 {
				return cty.NilVal, errorRequest("wrong request: %s", cmd)
			}
			stack[size-1], err = fn(stack[size-1])
			if err != nil {
				return
			}
		} else if op, ok = operations[exp]; ok {
			if size < 2 {
				return cty.NilVal, errorRequest("wrong request: %s", cmd)
			}
			stack[size-2], err = op(stack[size-2], stack[size-1])
			if err != nil {
				return
			}
			stack = stack[:size-1]
		} else if len(exp) > 0 {
			if exp[0] == dollar || exp[0] == at {
				commands, err = parseJsonPath(exp)
				if err != nil {
					return
				}
				slice, err = evaluateCommands(node, commands)
				if err != nil {
					return
				}
				if len(slice) > 1 { // array given
					stack = append(stack, cty.ListVal(slice))
				} else if len(slice) == 1 {
					stack = append(stack, slice[0])
				} else { // no data found
					return cty.NilVal, nil
				}
			} else if constant, ok := constants[strings.ToLower(exp)]; ok {
				stack = append(stack, constant)
			} else {
				bstr = []byte(exp)

				size = len(bstr)
				if size >= 2 && bstr[0] == quote && bstr[size-1] == quote {
					if sstr, ok := unquote(bstr, quote); ok {
						temp = cty.StringVal(sstr)
					} else {
						err = errorRequest("wrong request: %s", cmd)
					}
				} else {
					err = errorRequest("Unreachable condition")
					//temp, err = Unmarshal(bstr)
				}
				if err != nil {
					return
				}
				stack = append(stack, temp)
			}
		} else {
			stack = append(stack, cty.StringVal(""))
			//stack = append(stack, valueNode(nil, "", String, ""))
		}
	}
	if len(stack) == 1 {
		return stack[0], nil
	}
	if len(stack) == 0 {
		return cty.NilVal, nil
	}
	return cty.NilVal, errorRequest("wrong request: %s", cmd)
}

func getPositiveIndex(index int, count int) int {
	if index < 0 {
		index += count
	}
	return index
}

func evaluateCommands(val cty.Value, commands []string) (result []cty.Value, err error) {
	result = make([]cty.Value, 0)
	var (
		temporary []cty.Value
		keys      []string
		num       int
		key       string
		ok        bool
		value cty.Value
		tokens tokens
		expr   rpn
	)
	for i, cmd := range commands {
		tokens, err = newTokenizer([]byte(cmd)).tokenize()
		if err != nil {
			return
		}
		switch {
		case cmd == "$": // root element
			if i == 0 {
				result = append(result, val)
			}
		case cmd == "@": // current element
			if i == 0 {
				result = append(result, val)
			}
		case cmd == "..": // recursive descent
			temporary = make([]cty.Value, 0)
			for _, element := range result {
				temporary = append(temporary, recursiveChildren(element)...)
			}
			result = append(result, temporary...)
		case cmd == "*": // wildcard
			temporary = make([]cty.Value, 0)
			for _, element := range result {
				if isArray(element) {
					temporary = append(temporary, element.AsValueSlice()...)
				}
			}
			result = temporary
		case tokens.exists(":"): // array slice operator
			if tokens.count(":") > 3 {
				return nil, errorRequest("slice must contains no more than 2 colons, got '%s'", cmd)
			}
			keys = tokens.slice(":")

			temporary = make([]cty.Value, 0)
			for _, element := range result {
				if element.Type().IsListType() && element.LengthInt() > 0 {
					indices := []int{0, element.LengthInt(), 1}
					if indices[1] == -1 { indices[1] = 0 }
					for i, kStr := range keys {
						if kStr != "" && i < 3{
							ki, err := strconv.Atoi(kStr)
							if err != nil {
								return nil, err
							}
							indices[i] = ki
						}
					}
					if indices[0] < 0 || indices[0] >= element.LengthInt() {
						return nil, errorRequest("bad slice %v", keys)
					}
					if indices[1] < indices[0] || indices[1] > element.LengthInt() {
						return nil, errorRequest("bad slice %v", keys)
					}
					if indices[2] != 1 {
						return nil, errorRequest("only [a:b] slice operator supported, not [a:b:c]: '%v'", keys)
					}
					temporary = append(temporary, element.AsValueSlice()[indices[0] : indices[1]]...)
				}
			}
			result = temporary
		case strings.HasPrefix(cmd, "?(") && strings.HasSuffix(cmd, ")"): // applies a filter (script) expression
			expr, err = newTokenizer([]byte(cmd[2 : len(cmd)-1])).rpn()
			if err != nil {
				return nil, errorRequest("wrong request: %s", cmd)
			}
			//temporary = make([]cty.Value, 0)
			L := []cty.Value{}
			for _, element := range result {
				if isArray(element) {
					for _, temp := range element.AsValueSlice() {
						value, err = eval(temp, expr, cmd)
						if err != nil {
							return nil, errorRequest("wrong request: %s", cmd)
						}
						if value.IsNull() || len(value.Type().TestConformance(cty.Bool)) != 0 {
							continue
						}
						if !value.True() {
							continue
						}
						L = append(L, temp)
					}
				}
			}
			ok = true
			if len(L) == 0 {
				result = []cty.Value {}
			} else {
				result = []cty.Value{cty.ListVal(L)}
			}
		default: // try to get by key & Union
			if tokens.exists(",") {
				keys = tokens.slice(",")
				if len(keys) == 0 {
					return nil, errorRequest("wrong request: %s", cmd)
				}
			} else {
				keys = []string{cmd}
			}

			temporary = make([]cty.Value, 0)
			for _, key = range keys { // fixme
				for _, element := range result {
					if isArray(element) {
						sl := element.AsValueSlice()
						if key == "length" || key == "'length'" || key == "\"length\"" {
							value, err = functions["length"](element)
							if err != nil {
								return
							}
							ok = true
						} else {
							key, _ = plainString(key)
							num, err = strconv.Atoi(key)
							if err != nil || len(sl) == 0 {
								ok = false
								err = nil
							} else {
								num = getPositiveIndex(num, len(sl))
								if !element.HasIndex(cty.NumberIntVal(int64(num))).True() {
									ok = false
								} else {
									value = sl[num]
									ok = true
								}
							}
						}
					} else if isObject(element) {
						key, ok = cleanKey(key)
						ctyKey := cty.StringVal(key)
						if element.Type().IsMapType() && !element.HasIndex(ctyKey).True() {
							ok = false
							continue
						} else if !element.Type().HasAttribute(key) {
							ok = false
							continue
						}

						value = element.AsValueMap()[key]
					}
					if ok {
						temporary = append(temporary, value)
						ok = false
					}
				}
			}
			result = temporary
		}
	}
	return
}

func cleanKey(key string) (string, bool) {
	bString := []byte(key)
	from := len(bString)
	if from > 1 && (bString[0] == quotes && bString[from-1] == quotes) {
		return unquote(bString, quotes)
	}
	if from > 1 && (bString[0] == quote && bString[from-1] == quote) {
		return unquote(bString, quote)
	}
	return key, true
	// todo quote string and unquote it:
	// {
	// 	bString = append([]byte{quotes}, bString...)
	// 	bString = append(bString, quotes)
	// }
	// return unquote(bString, quotes)
}
