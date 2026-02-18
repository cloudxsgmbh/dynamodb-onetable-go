/*
Package onetable – expression builder.

Mirrors JS: Expression.js – converts model operations into DynamoDB command parameters.
*/
package onetable

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// KeyOperators are valid sort-key comparison operators for find operations.
var KeyOperators = map[string]bool{
	"<": true, "<=": true, "=": true, ">=": true, ">": true,
	"begins": true, "begins_with": true, "between": true,
}

type updates struct {
	add    []string
	del    []string
	remove []string
	set    []string
}

// expression builds the DynamoDB command parameters for a model operation.
type expression struct {
	model      *Model
	op         string
	properties Item
	params     *Params

	// index selection
	index *IndexDef
	hash  string
	sort  string

	// command building state
	already   map[string]bool // attributes already processed
	key       Item            // primary key values (get/delete/update)
	keys      []string        // key condition expressions (find)
	conditions []string
	filters    []string
	project    []string
	puts       Item            // for put operations
	mapped     map[string]Item // packed attribute staging

	names     map[string]string // ExpressionAttributeNames index → name
	namesMap  map[string]int    // name → index (dedup)
	values    map[string]any    // ExpressionAttributeValues index → value
	valuesMap map[string]int    // value → index (dedup, non-object/non-number)
	nindex    int
	vindex    int

	updates updates
	execute bool
	canPut  bool

	tableName string
}

func newExpression(model *Model, op string, properties Item, params *Params) (*expression, error) {
	e := &expression{}
	if err := e.init(model, op, properties, params); err != nil {
		return nil, err
	}
	if err := e.prepare(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *expression) init(model *Model, op string, properties Item, params *Params) error {
	e.model = model
	e.op = op
	e.properties = properties
	e.params = params
	e.already = map[string]bool{}
	e.key = Item{}
	e.mapped = map[string]Item{}
	e.names = map[string]string{}
	e.namesMap = map[string]int{}
	e.values = map[string]any{}
	e.valuesMap = map[string]int{}
	e.puts = Item{}
	e.execute = params.Execute == nil || *params.Execute
	e.canPut = op == "put" || (params.Batch != nil && op == "update")
	e.tableName = model.tableName

	e.index = model.selectIndex(params)
	e.hash = e.index.Hash
	e.sort = e.index.Sort

	if model.table.client == nil {
		return NewArgError("Table has not yet defined a client instance")
	}
	return nil
}

func (e *expression) prepare() error {
	op := e.op
	switch op {
	case "find":
		e.addWhereFilters()
	case "delete", "put", "update", "check":
		e.addConditions(op)
	case "scan":
		e.addWhereFilters()
		// generic scan filters for unknown fields
		for name, value := range e.properties {
			if _, ok := e.model.block.Fields[name]; !ok && value != nil {
				e.addGenericFilter(name, value)
			}
		}
	}

	e.puts = e.addProperties(op, &e.model.block, e.properties)

	// check mapped attributes are complete
	for att, props := range e.mapped {
		expected := len(e.model.mappings[att])
		if len(props) != expected {
			return NewArgError(fmt.Sprintf(`Missing properties for mapped field "%s" in model "%s"`, att, e.model.Name))
		}
	}
	// emit mapped attributes as top-level fields
	for k, v := range e.mapped {
		field := &preparedField{Attribute: []string{k}, Name: k}
		e.add(op, e.properties, field, k, v, true)
		e.puts[k] = v
	}

	// projection fields
	if e.params.Fields != nil {
		for _, name := range e.params.Fields {
			if e.params.Batch != nil || e.model.generic {
				e.project = append(e.project, fmt.Sprintf("#_%d", e.addName(name)))
			} else if f, ok := e.model.block.Fields[name]; ok {
				att := f.Attribute[0]
				e.project = append(e.project, fmt.Sprintf("#_%d", e.addName(att)))
			}
		}
	}
	return nil
}

// addProperties processes all properties for a given block level.
func (e *expression) addProperties(op string, block *fieldBlock, properties Item) Item {
	rec := Item{}
	fields := block.Fields

	if properties == nil {
		return rec
	}
	for name, value := range properties {
		field := fields[name]
		if field == nil {
			// unknown field
			synth := &preparedField{Attribute: []string{name}, Name: name}
			if e.model.generic {
				e.add(op, properties, synth, name, value, true)
			}
			rec[name] = value
			continue
		}
		att := field.Attribute[0]
		path := att
		if field.Block == nil {
			e.add(op, properties, field, path, value, true)
		} else {
			// nested schema
			partial := e.model.getPartial(field, e.params)
			if field.IsArray {
				if arr, ok := value.([]any); ok {
					cp := make([]any, len(arr))
					for i, v := range arr {
						ipath := fmt.Sprintf("%s[%d]", path, i)
						if sub, ok := v.(Item); ok {
							cp[i] = e.addProperties(op, field.Block, sub)
						} else {
							cp[i] = v
						}
						_ = ipath
					}
					if !partial {
						e.add(op, properties, field, path, cp, true)
					}
					value = cp
				}
			} else {
				if sub, ok := value.(Item); ok {
					value = e.addProperties(op, field.Block, sub)
				}
				if !partial {
					e.add(op, properties, field, path, value, true)
				}
			}
		}
		rec[field.Attribute[0]] = value
	}
	return rec
}

// add emits key / filter / update expressions for a single field value.
func (e *expression) add(op string, properties Item, field *preparedField, path string, value any, emit bool) {
	if e.already[path] {
		return
	}
	att := field.Attribute
	if len(att) > 1 {
		// packed / mapped attribute
		top, sub := att[0], att[1]
		if e.mapped[top] == nil {
			e.mapped[top] = Item{}
		}
		e.mapped[top][sub] = value
		if op == "put" {
			properties[top] = value
		}
		return
	}

	isHash := path == e.hash
	isSort := path == e.sort

	if isHash || isSort {
		switch op {
		case "find":
			e.addKey(op, field, value)
		case "scan":
			if properties[field.Name] != nil && !filterDisabled(field) {
				e.addFilter(field, path, value)
			}
		case "delete", "get", "update", "check":
			if field.IsIndexed {
				e.addKey(op, field, value)
			}
		}
	} else if emit {
		switch op {
		case "find", "scan":
			if properties[field.Name] != nil && !filterDisabled(field) && e.params.Batch == nil {
				e.addFilter(field, path, value)
			}
		case "update":
			e.addUpdate(field, path, value)
		}
	}
}

func filterDisabled(field *preparedField) bool {
	return field.Def.Filter != nil && !*field.Def.Filter
}

// addConditions adds exists/type/where condition expressions.
func (e *expression) addConditions(op string) {
	hash := e.index.Hash
	sort := e.index.Sort
	params := e.params

	if params.Exists != nil && *params.Exists {
		e.conditions = append(e.conditions, fmt.Sprintf("attribute_exists(#_%d)", e.addName(hash)))
		if sort != "" {
			e.conditions = append(e.conditions, fmt.Sprintf("attribute_exists(#_%d)", e.addName(sort)))
		}
	} else if params.Exists != nil && !*params.Exists {
		e.conditions = append(e.conditions, fmt.Sprintf("attribute_not_exists(#_%d)", e.addName(hash)))
		if sort != "" {
			e.conditions = append(e.conditions, fmt.Sprintf("attribute_not_exists(#_%d)", e.addName(sort)))
		}
	}

	if op == "update" {
		e.addUpdateConditions()
	}

	if params.Where != "" {
		e.conditions = append(e.conditions, e.expand(params.Where))
	}
}

func (e *expression) addWhereFilters() {
	if e.params.Where != "" {
		e.filters = append(e.filters, e.expand(e.params.Where))
	}
}

func (e *expression) addFilter(field *preparedField, path string, value any) {
	if path == e.hash || path == e.sort {
		return
	}
	target, variable := e.prepareKeyValue(path, value)
	e.filters = append(e.filters, fmt.Sprintf("%s = %s", target, variable))
}

func (e *expression) addGenericFilter(att string, value any) {
	e.filters = append(e.filters, fmt.Sprintf("#_%d = :_%d", e.addName(att), e.addValue(value)))
}

func (e *expression) addKey(op string, field *preparedField, value any) {
	att := field.Attribute[0]
	if op == "find" {
		if att == e.sort {
			if obj, ok := value.(map[string]any); ok && len(obj) > 0 {
				for action, vars := range obj {
					if !KeyOperators[action] {
						panic(NewArgError(`Invalid KeyCondition operator "` + action + `"`).Error())
					}
					switch action {
					case "begins_with", "begins":
						e.keys = append(e.keys, fmt.Sprintf("begins_with(#_%d, :_%d)", e.addName(att), e.addValue(vars)))
					case "between":
						arr, _ := vars.([]any)
						if len(arr) == 2 {
							e.keys = append(e.keys, fmt.Sprintf("#_%d BETWEEN :_%d AND :_%d",
								e.addName(att), e.addValue(arr[0]), e.addValue(arr[1])))
						}
					default:
						e.keys = append(e.keys, fmt.Sprintf("#_%d %s :_%d", e.addName(att), action, e.addValue(obj[action])))
					}
				}
				return
			}
		}
		e.keys = append(e.keys, fmt.Sprintf("#_%d = :_%d", e.addName(att), e.addValue(value)))
	} else {
		e.key[att] = value
		e.already[att] = true
	}
}

func (e *expression) addUpdate(field *preparedField, path string, value any) {
	if path == e.hash || path == e.sort {
		return
	}
	if field.Name == e.model.typeField {
		if !(e.params.Exists == nil || (e.params.Exists != nil && !*e.params.Exists)) {
			return
		}
	}
	if containsStr(e.params.Remove, field.Name) {
		return
	}
	target := e.prepareKey(path)
	variable := e.addValueExp(value)
	e.updates.set = append(e.updates.set, fmt.Sprintf("%s = %s", target, variable))
}

func (e *expression) addUpdateConditions() {
	params := e.params
	assertNotPartition := func(key, op string) {
		if key == e.hash || key == e.sort {
			panic(NewArgError(fmt.Sprintf("Cannot %s hash or sort", op)).Error())
		}
	}
	for key, value := range params.Add {
		assertNotPartition(key, "add")
		target, variable := e.prepareKeyValue(key, value)
		e.updates.add = append(e.updates.add, fmt.Sprintf("%s %s", target, variable))
	}
	for key, value := range params.Delete {
		assertNotPartition(key, "delete")
		target, variable := e.prepareKeyValue(key, value)
		e.updates.del = append(e.updates.del, fmt.Sprintf("%s %s", target, variable))
	}
	for _, key := range params.Remove {
		assertNotPartition(key, "remove")
		target := e.prepareKey(key)
		e.updates.remove = append(e.updates.remove, target)
	}
	for key, value := range params.Set {
		assertNotPartition(key, "set")
		target, variable := e.prepareKeyValue(key, value)
		e.updates.set = append(e.updates.set, fmt.Sprintf("%s = %s", target, variable))
	}
	for key, value := range params.Push {
		assertNotPartition(key, "push")
		emptyIdx := e.addValue([]any{})
		itemsIdx := e.addValue(asSlice(value))
		target := e.prepareKey(key)
		e.updates.set = append(e.updates.set,
			fmt.Sprintf("%s = list_append(if_not_exists(%s, :_%d), :_%d)", target, target, emptyIdx, itemsIdx))
	}
}

// expand replaces ${attr} and {value} tokens in a where/set expression string.
func (e *expression) expand(where string) string {
	fields := e.model.block.Fields

	// ${attr} → #_N expression name
	attrRe := regexp.MustCompile(`\$\{(.*?)\}`)
	where = attrRe.ReplaceAllStringFunc(where, func(m string) string {
		varName := m[2 : len(m)-1]
		return e.makeTarget(fields, varName)
	})

	// @{sub} → :_N substitution value
	subRe := regexp.MustCompile(`@\{(\.\.\.)?([^}]+)\}`)
	where = subRe.ReplaceAllStringFunc(where, func(m string) string {
		spread := strings.HasPrefix(m, "@{...")
		name := m[2 : len(m)-1]
		if spread {
			name = name[3:] // strip ...
		}
		if e.params.Substitutions == nil || e.params.Substitutions[name] == nil {
			panic(fmt.Sprintf("missing substitution for %q", name))
		}
		val := e.params.Substitutions[name]
		if spread {
			if arr, ok := val.([]any); ok {
				idxs := make([]string, len(arr))
				for i, v := range arr {
					idxs[i] = fmt.Sprintf(":_%d", e.addValue(v))
				}
				return strings.Join(idxs, ", ")
			}
		}
		return fmt.Sprintf(":_%d", e.addValue(val))
	})

	// {value} → :_N literal value
	valRe := regexp.MustCompile(`\{([^}]*)\}`)
	where = valRe.ReplaceAllStringFunc(where, func(m string) string {
		inner := m[1 : len(m)-1]
		var val any
		// numeric?
		if f, err := strconv.ParseFloat(inner, 64); err == nil {
			val = f
		} else if inner == "true" {
			val = true
		} else if inner == "false" {
			val = false
		} else {
			// strip surrounding quotes
			if len(inner) >= 2 && inner[0] == '"' && inner[len(inner)-1] == '"' {
				val = inner[1 : len(inner)-1]
			} else {
				val = inner
			}
		}
		return fmt.Sprintf(":_%d", e.addValue(val))
	})

	return where
}

// makeTarget translates a dotted field path into expression attribute name references.
func (e *expression) makeTarget(fields map[string]*preparedField, name string) string {
	parts := strings.Split(name, ".")
	targets := make([]string, 0, len(parts))
	for _, part := range parts {
		subscript := ""
		if idx := strings.Index(part, "["); idx >= 0 {
			subscript = part[idx:]
			part = part[:idx]
		}
		var att string
		if f, ok := fields[part]; ok {
			att = f.Attribute[0]
			if f.Block != nil {
				fields = f.Block.Fields
			} else {
				fields = nil
			}
		} else {
			att = part
			fields = nil
		}
		targets = append(targets, fmt.Sprintf("#_%d%s", e.addName(att), subscript))
	}
	return strings.Join(targets, ".")
}

func (e *expression) prepareKey(key string) string {
	e.already[key] = true
	return e.makeTarget(e.model.block.Fields, key)
}

func (e *expression) prepareKeyValue(key string, value any) (string, string) {
	target := e.prepareKey(key)
	if s, ok := value.(string); ok {
		if strings.ContainsAny(s, "${@{") {
			return target, e.expand(s)
		}
	}
	return target, e.addValueExp(value)
}

func (e *expression) addName(name string) int {
	if idx, ok := e.namesMap[name]; ok {
		return idx
	}
	idx := e.nindex
	e.nindex++
	key := fmt.Sprintf("#_%d", idx)
	e.names[key] = name
	e.namesMap[name] = idx
	return idx
}

func (e *expression) addValue(value any) int {
	// dedup non-object, non-number values
	if value != nil {
		switch value.(type) {
		case map[string]any, []any, float64, int, int64:
			// do not dedup
		default:
			k := fmt.Sprintf("%v", value)
			if idx, ok := e.valuesMap[k]; ok {
				return idx
			}
			idx := e.vindex
			e.vindex++
			key := fmt.Sprintf(":_%d", idx)
			e.values[key] = value
			e.valuesMap[k] = idx
			return idx
		}
	}
	idx := e.vindex
	e.vindex++
	key := fmt.Sprintf(":_%d", idx)
	e.values[key] = value
	return idx
}

func (e *expression) addValueExp(value any) string {
	return fmt.Sprintf(":_%d", e.addValue(value))
}

func (e *expression) and(terms []string) string {
	if len(terms) == 1 {
		return terms[0]
	}
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = "(" + t + ")"
	}
	return strings.Join(parts, " and ")
}

// command builds the final DynamoDB command map.
func (e *expression) command() (Item, error) {
	op := e.op
	params := e.params

	namesLen := len(e.names)
	valuesLen := len(e.values)

	// marshall key and values
	key, err := marshallForDynamo(e.key)
	if err != nil {
		return nil, err
	}
	puts, err := marshallForDynamo(e.puts)
	if err != nil {
		return nil, err
	}
	values, err := marshallForDynamo(e.values)
	if err != nil {
		return nil, err
	}

	// batch mode
	if params.Batch != nil {
		var args Item
		switch op {
		case "get":
			args = Item{"Key": key}
		case "delete":
			args = Item{"Key": key}
		case "put":
			args = Item{"Item": puts}
		default:
			return nil, NewArgError(`Unsupported batch operation "` + op + `"`)
		}
		if len(e.filters) > 0 {
			return nil, NewArgError("Invalid filters with batch operation")
		}
		return args, nil
	}

	// regular mode
	var condExpr, filterExpr, keyCondExpr, projExpr *string

	if len(e.conditions) > 0 {
		s := e.and(e.conditions)
		condExpr = &s
	}
	if len(e.filters) > 0 {
		s := e.and(e.filters)
		filterExpr = &s
	}
	if len(e.keys) > 0 {
		s := strings.Join(e.keys, " and ")
		keyCondExpr = &s
	}
	if len(e.project) > 0 {
		s := strings.Join(e.project, ", ")
		projExpr = &s
	}

	args := Item{
		"TableName": e.tableName,
	}
	if condExpr != nil {
		args["ConditionExpression"] = *condExpr
	}
	if namesLen > 0 {
		args["ExpressionAttributeNames"] = e.names
	}
	if namesLen > 0 && valuesLen > 0 {
		args["ExpressionAttributeValues"] = values
	}
	if filterExpr != nil {
		args["FilterExpression"] = *filterExpr
	}
	if keyCondExpr != nil {
		args["KeyConditionExpression"] = *keyCondExpr
	}
	if projExpr != nil {
		args["ProjectionExpression"] = *projExpr
	}

	if params.Select != "" {
		args["Select"] = params.Select
	} else if params.Count {
		args["Select"] = "COUNT"
	}

	if params.Stats != nil || e.model.table.metrics != nil {
		args["ReturnConsumedCapacity"] = coalesce(params.Capacity, "TOTAL")
		args["ReturnItemCollectionMetrics"] = "SIZE"
	}

	// return values
	var returnValues string
	if params.Return != nil {
		switch r := params.Return.(type) {
		case bool:
			if r {
				if op == "delete" {
					returnValues = "ALL_OLD"
				} else {
					returnValues = "ALL_NEW"
				}
			} else {
				returnValues = "NONE"
			}
		case string:
			if strings.ToLower(r) == "none" {
				returnValues = "NONE"
			} else if r != "get" {
				returnValues = r
			}
		}
	}

	switch op {
	case "put":
		args["Item"] = puts
		if returnValues == "" {
			returnValues = "NONE"
		}
		args["ReturnValues"] = returnValues
	case "update":
		if returnValues == "" {
			returnValues = "ALL_NEW"
		}
		args["ReturnValues"] = returnValues
		var updateParts []string
		if len(e.updates.add) > 0 {
			updateParts = append(updateParts, "add "+strings.Join(e.updates.add, ", "))
		}
		if len(e.updates.del) > 0 {
			updateParts = append(updateParts, "delete "+strings.Join(e.updates.del, ", "))
		}
		if len(e.updates.remove) > 0 {
			updateParts = append(updateParts, "remove "+strings.Join(e.updates.remove, ", "))
		}
		if len(e.updates.set) > 0 {
			updateParts = append(updateParts, "set "+strings.Join(e.updates.set, ", "))
		}
		args["UpdateExpression"] = strings.Join(updateParts, " ")
	case "delete":
		if returnValues == "" {
			returnValues = "ALL_OLD"
		}
		args["ReturnValues"] = returnValues
	}

	if op == "delete" || op == "get" || op == "update" || op == "check" {
		args["Key"] = key
	}
	if op == "find" || op == "get" || op == "scan" {
		args["ConsistentRead"] = params.Consistent
		if params.Index != "" && params.Index != "primary" {
			args["IndexName"] = params.Index
		}
	}
	if op == "find" || op == "scan" {
		if params.Limit > 0 {
			args["Limit"] = params.Limit
		}
		// ScanIndexForward: reverse XOR prev-without-next
		reverse := params.Reverse
		prevMode := params.Prev != nil && params.Next == nil
		args["ScanIndexForward"] = !(reverse != prevMode) // XOR

		cursor := params.Next
		if cursor == nil {
			cursor = params.Prev
		}
		if cursor != nil {
			start := Item{e.hash: cursor[e.hash]}
			if e.sort != "" {
				if sv := cursor[e.sort]; sv != nil {
					start[e.sort] = sv
				}
			}
			if params.Index != "" && params.Index != "primary" {
				pi := e.model.indexes["primary"]
				start[pi.Hash] = cursor[pi.Hash]
				if pi.Sort != "" {
					if sv := cursor[pi.Sort]; sv != nil {
						start[pi.Sort] = sv
					}
				}
			}
			if start[e.hash] != nil {
				mk, err := marshallForDynamo(start)
				if err == nil {
					args["ExclusiveStartKey"] = mk
				}
			}
		}
	}
	if op == "scan" {
		if params.Segments > 0 {
			args["TotalSegments"] = params.Segments
		}
		if params.Segment > 0 {
			args["Segment"] = params.Segment
		}
	}

	// strip nil/zero values
	cleaned := Item{}
	for k, v := range args {
		if v != nil {
			cleaned[k] = v
		}
	}

	if params.PostFormat != nil {
		cleaned = params.PostFormat(e.model, cleaned)
	}
	return cleaned, nil
}

// asSlice wraps a value in a []any if it isn't already.
func asSlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{v}
}
