/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jsonpath

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/zclconf/go-cty/cty"
)



type JSONPath struct {
	name       string
	parser     *Parser
	beginRange int
	inRange    int
	endRange   int

	lastEndNode *Node

	allowMissingKeys bool
	outputJSON       bool
}

// NewPath creates a new JSONPath with the given name.
func NewPath(jsonPath string) (*JSONPath, error) {
	j := &JSONPath{
		name:       "",
		beginRange: 0,
		inRange:    0,
		endRange:   0,
	}
	var err error
	j.parser, err = Parse(jsonPath)
	return j, err
}

type markPathRef struct { path *cty.Path }

func newPathRef(path cty.Path) markPathRef {
	p := path.Copy()
	return markPathRef{&p}
}

type SearchResult struct {
	original cty.Value
	Values []cty.Value
	Paths []cty.Path
}

// Given a JSON Path, this lets you search a cty.Value and return
// a result struct containg Value/Path pairs. There may not be 1:1
// mapping between both (due to recursive calls).
//
// Instead, it's better to iterate the paths and call .Apply(value)
// on them.
func (j *JSONPath) Search(data cty.Value) SearchResult {
	var res SearchResult
	vals, paths, err := j.Eval(data)
	if err != nil {
		return res
	}
	return SearchResult{data, vals, paths}
}

func (s SearchResult) String() (out string) {
	for _, item := range s.Paths {
		applied, _ := item.Apply(s.original)
		out += fmt.Sprintf("%#v => %s\n", PrettyCtyPath(item), applied.GoString())
	}
	return
}

// EvalRaw is like Eval() without extra processing (cty.Path and unmarking)
func (j *JSONPath) EvalRaw(data cty.Value) ([][]cty.Value, error) {
	data, _ = cty.Transform(data, func(path cty.Path, value cty.Value) (cty.Value, error) {
		return value.Mark(newPathRef(path)), nil
	})
	res, err := j.fullEvaluate(data)
	return res, err
}

// Returns a list of matched lists and paths based on a JSON path.
func (j *JSONPath) Eval(data cty.Value) ([]cty.Value, []cty.Path, error) {
	data, _ = cty.Transform(data, func(path cty.Path, value cty.Value) (cty.Value, error) {
		return value.Mark(newPathRef(path)), nil
	})
	res, err := j.fullEvaluate(data)
	if err != nil {
		return nil, nil, err
	}
	unmarkedData, _ := data.UnmarkDeep()
	if len(res) == 1 {
		result := res[0]
		paths := []cty.Path{}
		for _, item := range result {
			for mark, _ := range item.Marks() {
				if pr, ok := mark.(markPathRef); ok {
					P := *(pr.path)
					paths = append(paths, P)
				}
			}
		}

		for i, _ := range result {
			result[i], _ = result[i].UnmarkDeep()
		}

		filteredPaths := []cty.Path{}
		for _, path := range paths {
			outcome, _ := path.Apply(unmarkedData)
			put := false
			for _, item := range result {
				if item.Equals(outcome).True() {
					put = true
					break
				}
			}
			if put {
				filteredPaths = append(filteredPaths, path)
			}
		}

		return result, filteredPaths, err
	}
	return nil, nil, fmt.Errorf("expected len(nodes) = 1, shouldn't happen unless internal error.")
}

func (j *JSONPath) fullEvaluate(data cty.Value) ([][]cty.Value, error) {
	if j.parser == nil {
		return nil, fmt.Errorf("%s is an incomplete jsonpath template", j.name)
	}

	//cur := []cty.Value{reflect.ValueOf(data)}
	cur := []cty.Value{data}
	nodes := j.parser.Root.Nodes
	fullResult := [][]cty.Value{}
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		results, err := j.walk(cur, node)
		if err != nil {
			return nil, err
		}

		// encounter an end node, break the current block
		if j.endRange > 0 && j.endRange <= j.inRange {
			j.endRange--
			j.lastEndNode = &nodes[i]
			break
		}
		// encounter a range node, start a range loop
		if j.beginRange > 0 {
			j.beginRange--
			j.inRange++
			if len(results) > 0 {
				for _, value := range results {
					j.parser.Root.Nodes = nodes[i+1:]
					nextResults, err := j.fullEvaluate(value)
					if err != nil {
						return nil, err
					}
					fullResult = append(fullResult, nextResults...)
				}
			} else {
				// If the range has no results, we still need to process the nodes within the range
				// so the position will advance to the end node
				j.parser.Root.Nodes = nodes[i+1:]
				_, err := j.fullEvaluate(cty.NilVal)
				if err != nil {
					return nil, err
				}
			}
			j.inRange--

			// Fast forward to resume processing after the most recent end node that was encountered
			for k := i + 1; k < len(nodes); k++ {
				if &nodes[k] == j.lastEndNode {
					i = k
					break
				}
			}
			continue
		}
		fullResult = append(fullResult, results)
	}
	return fullResult, nil
}

// EnableJSONOutput changes the PrintResults behavior to return a JSON array of results
func (j *JSONPath) EnableJSONOutput(v bool) {
	j.outputJSON = v
}

// walk visits tree rooted at the given node in DFS order
func (j *JSONPath) walk(value []cty.Value, node Node) ([]cty.Value, error) {
	switch node := node.(type) {
	case *ListNode:
		return j.evalList(value, node)
	case *TextNode:
		return []cty.Value{cty.StringVal(node.Text)}, nil
	case *FieldNode:
		return j.evalField(value, node)
	case *ArrayNode:
		return j.evalArray(value, node)
	case *FilterNode:
		return j.evalFilter(value, node)
	case *IntNode:
		return j.evalInt(value, node)
	case *BoolNode:
		return j.evalBool(value, node)
	case *FloatNode:
		return j.evalFloat(value, node)
	case *WildcardNode:
		return j.evalWildcard(value, node)
	case *RecursiveNode:
		return j.evalRecursive(value, node)
	case *UnionNode:
		return j.evalUnion(value, node)
	case *IdentifierNode:
		return j.evalIdentifier(value, node)
	default:
		return value, fmt.Errorf("unexpected Node %v", node)
	}
}

// evalInt evaluates IntNode
func (j *JSONPath) evalInt(input []cty.Value, node *IntNode) ([]cty.Value, error) {
	result := make([]cty.Value, len(input))
	for i := range input {
		result[i] = cty.NumberIntVal(int64(node.Value))
	}
	return result, nil
}

// evalFloat evaluates FloatNode
func (j *JSONPath) evalFloat(input []cty.Value, node *FloatNode) ([]cty.Value, error) {
	result := make([]cty.Value, len(input))
	for i := range input {
		result[i] = cty.NumberFloatVal(float64(node.Value))
	}
	return result, nil
}

// evalBool evaluates BoolNode
func (j *JSONPath) evalBool(input []cty.Value, node *BoolNode) ([]cty.Value, error) {
	result := make([]cty.Value, len(input))
	for i := range input {
		result[i] = cty.BoolVal(node.Value)
	}
	return result, nil
}

// evalList evaluates ListNode
func (j *JSONPath) evalList(value []cty.Value, node *ListNode) ([]cty.Value, error) {
	var err error
	curValue := value
	for _, node := range node.Nodes {
		curValue, err = j.walk(curValue, node)
		if err != nil {
			return curValue, err
		}
	}
	return curValue, nil
}

// evalIdentifier evaluates IdentifierNode
func (j *JSONPath) evalIdentifier(input []cty.Value, node *IdentifierNode) ([]cty.Value, error) {
	results := []cty.Value{}
	switch node.Name {
	case "range":
		j.beginRange++
		results = input
	case "end":
		if j.inRange > 0 {
			j.endRange++
		} else {
			return results, fmt.Errorf("not in range, nothing to end")
		}
	default:
		return input, fmt.Errorf("unrecognized identifier %v", node.Name)
	}
	return results, nil
}

// evalArray evaluates ArrayNode
func (j *JSONPath) evalArray(input []cty.Value, node *ArrayNode) ([]cty.Value, error) {
	result := []cty.Value{}
	for _, value := range input {
		//
		//value, isNil := template.Indirect(value)
		//if isNil {
		//	continue
		//}
		//if value.Kind() != reflect.Array && value.Kind() != reflect.Slice {
		//	return input, fmt.Errorf("%v is not array or slice", value.Type())
		//}
		unmarked, _ := value.Unmark()
		sliceLength := unmarked.LengthInt()

		params := node.Params
		if !params[0].Known {
			params[0].Value = 0
		}
		if params[0].Value < 0 {
			params[0].Value += sliceLength
		}
		if !params[1].Known {
			params[1].Value = sliceLength
		}

		if params[1].Value < 0 || (params[1].Value == 0 && params[1].Derived) {
			params[1].Value += sliceLength
		}

		if params[1].Value != params[0].Value { // if you're requesting zero elements, allow it through.
			if params[0].Value >= sliceLength || params[0].Value < 0 {
				return input, fmt.Errorf("array index out of bounds: index %d, length %d", params[0].Value, sliceLength)
			}
			if params[1].Value > sliceLength || params[1].Value < 0 {
				return input, fmt.Errorf("array index out of bounds: index %d, length %d", params[1].Value-1, sliceLength)
			}
			if params[0].Value > params[1].Value {
				return input, fmt.Errorf("starting index %d is greater than ending index %d", params[0].Value, params[1].Value)
			}
		} else {
			return result, nil
		}

		indices := make([]int, sliceLength)
		for i, _ := range indices {
			indices[i] = i
		}
		indices = indices[params[0].Value : params[1].Value]
		newVal := []cty.Value{}
		for _, item := range indices {
			child, _ := cty.Path{}.IndexInt(item).Apply(unmarked)
			newVal = append(newVal, child)
		}
		value = cty.TupleVal(newVal)

		//value = cty.TupleVal(unmarked.AsValueSlice()[params[0].Value : params[1].Value])
		//value = value.Slice(params[0].Value, params[1].Value)

		step := 1
		if params[2].Known {
			if params[2].Value <= 0 {
				return input, fmt.Errorf("step must be > 0")
			}
			step = params[2].Value
		}
		_ = step
		for i := 0; i < value.LengthInt(); i += step {
			result = append(result, value.Index(cty.NumberIntVal(int64(i))))
		}
	}

	return result, nil
}

// evalUnion evaluates UnionNode
func (j *JSONPath) evalUnion(input []cty.Value, node *UnionNode) ([]cty.Value, error) {
	result := []cty.Value{}
	for _, listNode := range node.Nodes {
		temp, err := j.evalList(input, listNode)
		if err != nil {
			return input, err
		}
		result = append(result, temp...)
	}
	return result, nil
}

func (j *JSONPath) findFieldInValue(value *reflect.Value, node *FieldNode) (reflect.Value, error) {
	t := value.Type()
	var inlineValue *reflect.Value
	for ix := 0; ix < t.NumField(); ix++ {
		f := t.Field(ix)
		jsonTag := f.Tag.Get("json")
		parts := strings.Split(jsonTag, ",")
		if len(parts) == 0 {
			continue
		}
		if parts[0] == node.Value {
			return value.Field(ix), nil
		}
		if len(parts[0]) == 0 {
			val := value.Field(ix)
			inlineValue = &val
		}
	}
	if inlineValue != nil {
		if inlineValue.Kind() == reflect.Struct {
			// handle 'inline'
			match, err := j.findFieldInValue(inlineValue, node)
			if err != nil {
				return reflect.Value{}, err
			}
			if match.IsValid() {
				return match, nil
			}
		}
	}
	return value.FieldByName(node.Value), nil
}

// evalField evaluates field of struct or key of map.
func (j *JSONPath) evalField(input []cty.Value, node *FieldNode) ([]cty.Value, error) {
	results := []cty.Value{}
	// If there's no input, there's no output
	if len(input) == 0 {
		return results, nil
	}
	for _, value := range input {
		unmarked, _ := value.Unmark()
		var result cty.Value = cty.DynamicVal

		if value.Type().IsObjectType() {
			if value.Type().HasAttribute(node.Value) {
				result = value.GetAttr(node.Value)
			}
		} else {
			ss := cty.StringVal(node.Value)
			if unmarked.CanIterateElements() && unmarked.HasIndex(ss).True() {
				result = value.Index(ss)
			}
		}

		if result.IsKnown() {
			results = append(results, result)
		}
	}
	if len(results) == 0 {
		if true {
			return results, nil
		}
		return results, fmt.Errorf("%s is not found", node.Value)
	}
	return results, nil
}

func getByIter(value cty.Value, iter cty.ElementIterator) (out cty.Value) {
	out = cty.DynamicVal
	index, _ := iter.Element()
	if value.Type().IsObjectType() {
		if index.Type().Equals(cty.String) && value.Type().HasAttribute(index.AsString()) {
			out = value.GetAttr(index.AsString())
		}
	} else {
		if value.CanIterateElements() && value.HasIndex(index).True() {
			out, _ = cty.Path{}.Index(index).Apply(value)
		}
	}
	return
}
// evalWildcard extracts all contents of the given value
func (j *JSONPath) evalWildcard(input []cty.Value, node *WildcardNode) ([]cty.Value, error) {
	results := []cty.Value{}
	for _, value := range input {
		unmarked, _ := value.Unmark()
		if !unmarked.CanIterateElements() {
			continue
		}
		it := unmarked.ElementIterator()
		for it.Next() {
			results = append(results, getByIter(unmarked, it))
		}
	}
	return results, nil
}

// evalRecursive visits the given value recursively and pushes all of them to result
func (j *JSONPath) evalRecursive(input []cty.Value, node *RecursiveNode) ([]cty.Value, error) {
	result := []cty.Value{}
	for _, value := range input {
		results := []cty.Value{}

		unmarked, _ := value.Unmark()
		if !unmarked.CanIterateElements() {
			continue
		}

		it := unmarked.ElementIterator()
		for it.Next() {
			res := getByIter(unmarked, it)
			if !res.IsKnown() {
				continue
			}
			results = append(results, res)
		}

		if len(results) != 0 {
			result = append(result, value)

			output, err := j.evalRecursive(results, node)
			if len(output) == 0 {
				continue
			}
			if err != nil {
				return result, err
			}
			result = append(result, output...)
		}
	}
	return result, nil
}

// evalFilter filters array according to FilterNode
func (j *JSONPath) evalFilter(input []cty.Value, node *FilterNode) ([]cty.Value, error) {
	return nil, fmt.Errorf("filters not implemented yet")
}
