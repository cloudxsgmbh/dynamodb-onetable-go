/*
Package onetable – Model type and high-level CRUD operations.

Mirrors JS: Model.js  (create / get / find / update / remove / scan /
putItem / getItem / deleteItem / queryItems / scanItems / updateItem).
*/
package onetable

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	sanityPages   = 1000
	followThreads = 10
)

// Model represents a DynamoDB single-table entity.
type Model struct {
	table  *Table
	Name   string
	schema *schemaManager

	// primary key attribute names (resolved during prep)
	hash string
	sort string

	// cached table fields
	typeField    string
	createdField string
	updatedField string
	tableName    string
	generic      bool
	timestamps   any // bool | "create" | "update"
	nulls        bool
	nested       bool
	partial      bool

	// prepared field block (top level)
	block fieldBlock

	// attribute→index mapping (attribute name → index name)
	indexProperties map[string]string

	// packed attribute mappings: attrName → []sub-props
	mappings map[string][]string

	indexes map[string]*IndexDef

	hasUniqueFields bool
}

// newModel constructs and prepares a Model. fields may be nil for generic/internal models.
func newModel(table *Table, name string, opts modelOptions) *Model {
	if table == nil {
		panic("onetable: nil table")
	}
	m := &Model{
		table:        table,
		Name:         name,
		typeField:    coalesce(opts.TypeField, table.typeField),
		createdField: table.createdField,
		updatedField: table.updatedField,
		tableName:    table.Name,
		generic:      opts.Generic,
		timestamps:   opts.Timestamps,
		nulls:        table.nulls,
		partial:      table.partial,
		block:        fieldBlock{Fields: map[string]*preparedField{}, Deps: nil},
	}

	if m.timestamps == nil {
		m.timestamps = table.timestamps
	}

	// prefer explicitly-supplied indexes (needed during schema bootstrap)
	if opts.Indexes != nil {
		m.indexes = opts.Indexes
	} else if table.schemaMgr != nil {
		m.indexes = table.schemaMgr.indexes
	}

	if m.indexes == nil {
		panic("onetable: indexes must be defined before creating models")
	}

	// schema manager may not be set yet during bootstrap – resolved lazily via m.schema property
	m.schema = table.schemaMgr

	m.indexProperties = getIndexProperties(m.indexes)

	if opts.Fields != nil {
		m.prepModel(opts.Fields, &m.block, nil)
	}
	return m
}

type modelOptions struct {
	Fields     FieldMap
	TypeField  string
	Generic    bool
	Timestamps any              // override table timestamps
	Indexes    map[string]*IndexDef // if non-nil, overrides table.schemaMgr.indexes
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// getSchemaMgr returns the schema manager, resolving lazily from table if needed.
func (m *Model) getSchemaMgr() *schemaManager {
	if m.schema != nil {
		return m.schema
	}
	return m.table.schemaMgr
}

// ─── High-level CRUD ────────────────────────────────────────────────────────

// Params holds optional operation modifiers (mirrors JS params objects).
type Params struct {
	// Execution control
	Execute  *bool  // false → return command, don't execute
	Log      *bool  // true → log at info level
	Parse    bool   // unmarshal DynamoDB response into Item map
	High     bool   // high-level API mode (adds type filter, etc.)
	Hidden   *bool  // override hidden field visibility
	Partial  *bool  // override partial nested-update behaviour

	// Condition / exists
	Exists *bool // true=must exist, false=must not exist, nil=don't care

	// Pagination
	Limit    int
	Next     Item // exclusive start key for forward pagination
	Prev     Item // exclusive start key for backward pagination
	Reverse  bool
	MaxPages int

	// Index selection
	Index string // index name; "" = primary

	// Projection
	Fields []string // field names to project

	// Read consistency
	Consistent bool

	// Write return value
	Return any // true|false|"NONE"|"ALL_NEW"|"ALL_OLD"|"get"

	// Filter / where / set expressions
	Where         string
	Set           map[string]string
	Add           map[string]any
	Remove        []string
	Delete        map[string]any
	Push          map[string]any
	Substitutions map[string]any

	// Scan segments
	Segments int
	Segment  int

	// Count only
	Count  bool
	Select string // "COUNT"|"ALL_ATTRIBUTES" etc.

	// Stats
	Stats    *Stats
	Capacity string // "INDEXES"|"TOTAL"|"NONE"

	// Batch / transaction references (maps filled by caller)
	Batch       map[string]any
	Transaction map[string]any

	// Follow GSI to primary
	Follow *bool

	// Many items allowed on remove
	Many bool

	// Internal: mark already-cloned args
	checked      bool
	prepared     bool
	fallback     bool
	existsWasSet bool  // true when Exists was explicitly set by caller (even to nil)
	expression   *expression // stored during transact/batch for later parseResponse

	// Custom post-format hook
	PostFormat func(model *Model, cmd map[string]any) map[string]any

	// Low-level passthrough: custom DynamoDB client
	Client DynamoClient

	// Context for AWS SDK calls
	Context context.Context
}

// Item is a generic property map returned from / passed to model operations.
type Item = map[string]any

// Stats accumulates consumed-capacity metrics across paginated calls.
type Stats struct {
	Count    int
	Scanned  int
	Capacity float64
}

// Result is the return type for find/scan operations (items + pagination cursors).
type Result struct {
	Items []Item
	Next  Item // non-nil when more pages exist
	Prev  Item // non-nil when caller provided Next/Prev
	Count int  // only set when params.Count==true
}

// Create creates a new item. Fails if an item with the same key already exists
// (mirrors JS exists:false default for create).
func (m *Model) Create(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, &Params{Parse: true, High: true, Exists: boolPtr(false)})
	if m.hasUniqueFields {
		return m.createUnique(ctx, properties, params)
	}
	return m.putItem(ctx, properties, params)
}

// Get retrieves a single item by its key properties.
func (m *Model) Get(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, &Params{Parse: true, High: true})
	prepared, err := m.prepareProperties(ctx, "get", properties, params)
	if err != nil {
		return nil, err
	}
	if params.fallback {
		params.Limit = 2
		result, err := m.Find(ctx, properties, params)
		if err != nil {
			return nil, err
		}
		if len(result.Items) > 1 {
			return nil, NewError("Get without sort key returns more than one result",
				WithCode(ErrNonUnique), WithContext(map[string]any{"properties": properties}))
		}
		if len(result.Items) == 0 {
			return nil, nil
		}
		return result.Items[0], nil
	}
	expr, err := newExpression(m, "get", prepared, params)
	if err != nil {
		return nil, err
	}
	item, err := m.run(ctx, "get", expr)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// Find queries items matching the given properties.
func (m *Model) Find(ctx context.Context, properties Item, params *Params) (*Result, error) {
	properties, params = m.checkArgs(ctx, properties, params, &Params{Parse: true, High: true})
	return m.queryItems(ctx, properties, params)
}

// Scan scans all items matching the given properties (may span model types).
func (m *Model) Scan(ctx context.Context, properties Item, params *Params) (*Result, error) {
	properties, params = m.checkArgs(ctx, properties, params, &Params{Parse: true, High: true})
	return m.scanItems(ctx, properties, params)
}

// Update updates an existing item. Fails if the item does not exist (exists:true default).
func (m *Model) Update(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, &Params{Exists: boolPtr(true), Parse: true, High: true})
	if m.hasUniqueFields {
		// check if any unique property is being changed
		for k := range properties {
			if f, ok := m.block.Fields[k]; ok && f.Def.Unique {
				return m.updateUnique(ctx, properties, params)
			}
		}
	}
	return m.updateItem(ctx, properties, params)
}

// Upsert updates or creates (exists:nil). Unlike Update, no existence check is enforced.
func (m *Model) Upsert(ctx context.Context, properties Item, params *Params) (Item, error) {
	if params == nil {
		params = &Params{}
	}
	// Use checkArgs with nil Exists (upsert — no existence check).
	properties, params = m.checkArgs(ctx, properties, params, &Params{Exists: nil, Parse: true, High: true})
	// params.Exists is nil: upsert. If caller set Exists, respect that.
	if m.hasUniqueFields {
		for k := range properties {
			if f, ok := m.block.Fields[k]; ok && f.Def.Unique {
				return m.updateUnique(ctx, properties, params)
			}
		}
	}
	return m.updateItem(ctx, properties, params)
}

// Remove deletes an item by its key properties.
func (m *Model) Remove(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, &Params{Parse: true, High: true})
	prepared, err := m.prepareProperties(ctx, "delete", properties, params)
	if err != nil {
		return nil, err
	}
	if params.fallback || params.Many {
		return m.removeByFind(ctx, prepared, params)
	}
	if m.hasUniqueFields {
		return m.removeUnique(ctx, prepared, params)
	}
	expr, err := newExpression(m, "delete", prepared, params)
	if err != nil {
		return nil, err
	}
	item, err := m.run(ctx, "delete", expr)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// Init initialises a local item with defaults and value templates without writing to DynamoDB.
func (m *Model) Init(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, &Params{Parse: true, High: true})
	return m.initItem(ctx, properties, params)
}

// ─── Low-level item ops (mirrors JS private API) ────────────────────────────

func (m *Model) putItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, nil)
	if !params.prepared {
		if params.Transaction == nil || params.Transaction["timestamp"] == nil {
			now := time.Now()
			ts := m.table.timestamps
			if ts == true || ts == "create" {
				properties[m.createdField] = now
			}
			if ts == true || ts == "update" {
				properties[m.updatedField] = now
			}
		} else {
			ts := m.table.timestamps
			now := params.Transaction["timestamp"].(time.Time)
			if ts == true || ts == "create" {
				properties[m.createdField] = now
			}
			if ts == true || ts == "update" {
				properties[m.updatedField] = now
			}
		}
		var err error
		properties, err = m.prepareProperties(ctx, "put", properties, params)
		if err != nil {
			return nil, err
		}
	}
	expr, err := newExpression(m, "put", properties, params)
	if err != nil {
		return nil, err
	}
	return m.run(ctx, "put", expr)
}

func (m *Model) getItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, nil)
	prepared, err := m.prepareProperties(ctx, "get", properties, params)
	if err != nil {
		return nil, err
	}
	expr, err := newExpression(m, "get", prepared, params)
	if err != nil {
		return nil, err
	}
	return m.run(ctx, "get", expr)
}

func (m *Model) deleteItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, nil)
	if !params.prepared {
		var err error
		properties, err = m.prepareProperties(ctx, "delete", properties, params)
		if err != nil {
			return nil, err
		}
	}
	expr, err := newExpression(m, "delete", properties, params)
	if err != nil {
		return nil, err
	}
	return m.run(ctx, "delete", expr)
}

func (m *Model) queryItems(ctx context.Context, properties Item, params *Params) (*Result, error) {
	properties, params = m.checkArgs(ctx, properties, params, nil)
	prepared, err := m.prepareProperties(ctx, "find", properties, params)
	if err != nil {
		return nil, err
	}
	expr, err := newExpression(m, "find", prepared, params)
	if err != nil {
		return nil, err
	}
	return m.runMulti(ctx, "find", expr)
}

func (m *Model) scanItems(ctx context.Context, properties Item, params *Params) (*Result, error) {
	properties, params = m.checkArgs(ctx, properties, params, nil)
	prepared, err := m.prepareProperties(ctx, "scan", properties, params)
	if err != nil {
		return nil, err
	}
	expr, err := newExpression(m, "scan", prepared, params)
	if err != nil {
		return nil, err
	}
	return m.runMulti(ctx, "scan", expr)
}

func (m *Model) updateItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	properties, params = m.checkArgs(ctx, properties, params, nil)
	ts := m.table.timestamps
	if ts == true || ts == "update" {
		var now time.Time
		if params.Transaction != nil {
			if t, ok := params.Transaction["timestamp"]; ok {
				now = t.(time.Time)
			} else {
				now = time.Now()
				params.Transaction["timestamp"] = now
			}
		} else {
			now = time.Now()
		}
		properties[m.updatedField] = now
		// if_not_exists for createdField when upserting
		if params.Exists == nil && (ts == true) {
			isoDates := m.table.isoDates
			var when any
			if isoDates {
				when = now.UTC().Format(time.RFC3339Nano)
			} else {
				when = now.UnixMilli()
			}
			if params.Set == nil {
				params.Set = map[string]string{}
			}
			params.Set[m.createdField] = fmt.Sprintf("if_not_exists(${%s}, {%v})", m.createdField, when)
		}
	}
	prepared, err := m.prepareProperties(ctx, "update", properties, params)
	if err != nil {
		return nil, err
	}
	expr, err := newExpression(m, "update", prepared, params)
	if err != nil {
		return nil, err
	}
	return m.run(ctx, "update", expr)
}

func (m *Model) initItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	fields := m.block.Fields
	m.setDefaults("init", fields, properties, params)
	for k := range fields {
		if _, ok := properties[k]; !ok {
			properties[k] = nil
		}
	}
	err := m.runTemplates("put", "", m.indexes["primary"], m.block.Deps, properties, params)
	if err != nil {
		return nil, err
	}
	return properties, nil
}

// ─── run: execute a prepared expression ──────────────────────────────────────

// run executes an expression for single-item operations (get/put/update/delete).
func (m *Model) run(ctx context.Context, op string, expr *expression) (Item, error) {
	params := expr.params

	cmd, err := expr.command()
	if err != nil {
		return nil, err
	}

	// return command without executing
	if expr.execute == false {
		logInfo(m.table.log, fmt.Sprintf(`OneTable command for "%s" "%s" (not executed)`, op, m.Name),
			map[string]any{"cmd": cmd, "op": op})
		return cmd, nil
	}

	// batch accumulation
	if params.Batch != nil {
		return m.accumulateBatch(op, cmd, expr)
	}

	// transaction accumulation
	if params.Transaction != nil {
		return m.accumulateTransaction(op, cmd, expr)
	}

	result, err := m.table.execute(ctx, m.Name, op, cmd, expr.properties, params)
	if err != nil {
		return nil, err
	}

	if !params.Parse {
		return result, nil
	}

	// extract the actual item from the normalised result envelope
	var rawItems []Item
	switch op {
	case "put":
		// put returns properties (DynamoDB doesn't echo the item back by default)
		rawItems = nil // parseResponse handles put specially
	case "get":
		if item, ok := result["Item"].(Item); ok {
			rawItems = []Item{item}
		}
	case "delete", "update":
		if item, ok := result["Attributes"].(Item); ok {
			rawItems = []Item{item}
		}
	}

	items, err := m.parseResponse(ctx, op, expr, rawItems)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0], nil
}

// runMulti executes a find/scan expression and handles pagination.
func (m *Model) runMulti(ctx context.Context, op string, expr *expression) (*Result, error) {
	params := expr.params

	cmd, err := expr.command()
	if err != nil {
		return nil, err
	}

	if expr.execute == false {
		return &Result{Items: []Item{cmd}}, nil
	}

	maxPages := params.MaxPages
	if maxPages == 0 {
		maxPages = sanityPages
	}

	var rawItems []Item
	var lastKey Item
	var totalCount int
	pages := 0

	for {
		result, err := m.table.execute(ctx, m.Name, op, cmd, expr.properties, params)
		if err != nil {
			return nil, err
		}

		if items, ok := result["Items"].([]Item); ok {
			rawItems = append(rawItems, items...)
		}

		if result["Count"] != nil {
			totalCount += toInt(result["Count"])
		}

		if params.Stats != nil {
			if c := toInt(result["Count"]); c > 0 {
				params.Stats.Count += c
			}
			if s := toInt(result["ScannedCount"]); s > 0 {
				params.Stats.Scanned += s
			}
			if cap, ok := result["ConsumedCapacity"].(map[string]any); ok {
				if u, ok := cap["CapacityUnits"].(float64); ok {
					params.Stats.Capacity += u
				}
			}
		}

		lk, hasMore := result["LastEvaluatedKey"].(Item)
		if hasMore {
			cmd["ExclusiveStartKey"] = lk
			lastKey = lk
		}

		if params.Limit > 0 && len(rawItems) >= params.Limit {
			break
		}
		pages++
		if !hasMore || pages >= maxPages {
			break
		}
	}

	// compute prev cursor (first item keys)
	var prev Item
	if len(rawItems) > 0 && (params.Next != nil || params.Prev != nil) {
		idx := m.selectIndex(params)
		first := rawItems[0]
		prev = Item{idx.Hash: first[idx.Hash]}
		if idx.Sort != "" {
			prev[idx.Sort] = first[idx.Sort]
		}
		// also include primary keys if using non-primary index
		if params.Index != "" && params.Index != "primary" {
			pi := m.indexes["primary"]
			prev[pi.Hash] = first[pi.Hash]
			if pi.Sort != "" {
				prev[pi.Sort] = first[pi.Sort]
			}
		}
	}

	// parse response
	var items []Item
	if params.Parse {
		items, err = m.parseResponse(ctx, op, expr, rawItems)
		if err != nil {
			return nil, err
		}
	} else {
		items = rawItems
	}

	result := &Result{Items: items}

	if lastKey != nil {
		result.Next = m.table.unmarshallItem(lastKey)
	}
	if prev != nil {
		result.Prev = m.table.unmarshallItem(prev)
	}
	if params.Count || params.Select == "COUNT" {
		result.Count = totalCount
	}

	// reverse + swap next/prev when paginating backward
	if params.Prev != nil && params.Next == nil && op != "scan" {
		reverseItems(result.Items)
		result.Next, result.Prev = result.Prev, result.Next
	}

	// follow: resolve GSI items to primary via get
	if shouldFollow(params, m.selectIndex(params)) {
		result.Items, err = m.followItems(ctx, op, result.Items, params)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// ─── parseResponse ──────────────────────────────────────────────────────────

func (m *Model) parseResponse(ctx context.Context, op string, expr *expression, raw []Item) ([]Item, error) {
	var items []Item

	// put doesn't return the item from DynamoDB – use expression properties (already Go-typed)
	if op == "put" {
		raw = []Item{expr.properties}
		// raw is already plain Go values; skip unmarshalling below
	} else {
		// raw is already unmarshalled by execute() – no extra conversion needed
	}

	for _, item := range raw {
		typeName, _ := item[m.typeField].(string)
		if typeName == "" {
			typeName = m.Name
		}
		mod := m.getSchemaMgr().models[typeName]
		if mod == nil {
			mod = m
		}
		if mod == m.getSchemaMgr().uniqueModel {
			continue
		}
		transformed := mod.transformReadItem(op, item, expr.properties, expr.params, expr)
		if transformed != nil {
			items = append(items, transformed)
		}
	}
	return items, nil
}

// ─── transformReadItem ───────────────────────────────────────────────────────

func (m *Model) transformReadItem(op string, raw Item, properties Item, params *Params, expr *expression) Item {
	if raw == nil {
		return nil
	}
	return m.transformReadBlock(op, raw, properties, params, m.block.Fields, expr)
}

func (m *Model) transformReadBlock(op string, raw Item, properties Item, params *Params, fields map[string]*preparedField, expr *expression) Item {
	rec := Item{}
	showHidden := params != nil && params.Hidden != nil && *params.Hidden

	for name, field := range fields {
		// hidden visibility
		if field.Hidden && !showHidden {
			if params == nil || params.Follow == nil || !*params.Follow {
				if params == nil || params.Hidden == nil || !*params.Hidden {
					// skip hidden unless explicitly requested
					if !(params != nil && params.Hidden != nil && *params.Hidden) {
						continue
					}
				}
			}
		}

		var att, sub string
		if op == "put" {
			att = field.Name
		} else {
			att = field.Attribute[0]
			if len(field.Attribute) > 1 {
				sub = field.Attribute[1]
			}
		}

		value := raw[att]

		// decode encoded fields
		if value == nil && field.Def.Encode != nil {
			encSlice, ok := toSlice(field.Def.Encode)
			if ok && len(encSlice) >= 3 {
				encAtt, _ := encSlice[0].(string)
				sep, _ := encSlice[1].(string)
				idx, _ := toIntVal(encSlice[2])
				if src, ok := raw[encAtt].(string); ok {
					parts := strings.SplitN(src, sep, idx+2)
					if idx < len(parts) {
						value = parts[idx]
					}
				}
			}
		}

		// unpack sub-property
		if sub != "" && value != nil {
			if m, ok := value.(map[string]any); ok {
				value = m[sub]
			}
		}

		// decrypt
		if field.Def.Crypt && value != nil {
			if s, ok := value.(string); ok {
				dec, err := m.table.decrypt(s)
				if err == nil {
					value = dec
				}
			}
		}

		if value == nil {
			if field.Def.Default != nil {
				if params == nil || params.Fields == nil || containsStr(params.Fields, name) {
					rec[name] = field.Def.Default
				}
			} else if field.Required {
				// warn if required field missing (skip for transactions/batch/projections)
				if params != nil && !params.checked /* batch */ && params.Transaction == nil &&
					params.Batch == nil && params.Fields == nil {
					if m.table.warn {
						logError(m.table.log, fmt.Sprintf(`Required field "%s" in model "%s" not in item`, name, m.Name), nil)
					}
				}
			}
			continue
		}

		// nested block
		if field.Block != nil && value != nil {
			switch v := value.(type) {
			case []any:
				arr := make([]Item, 0, len(v))
				var propArr []any
				if properties != nil {
					propArr, _ = properties[name].([]any)
				}
				for i, elem := range v {
					var propElem Item
					if i < len(propArr) {
						propElem, _ = propArr[i].(Item)
					}
					if em, ok := elem.(map[string]any); ok {
						arr = append(arr, m.transformReadBlock(op, em, propElem, params, field.Block.Fields, expr))
					}
				}
				rec[name] = arr
			case map[string]any:
				var propNested Item
				if properties != nil {
					propNested, _ = properties[name].(Item)
				}
				rec[name] = m.transformReadBlock(op, v, propNested, params, field.Block.Fields, expr)
			}
			continue
		}

		rec[name] = m.transformReadAttribute(field, name, value, params, properties)
	}

	// generic models pass through all extra attributes
	if m.generic {
		for k, v := range raw {
			if _, exists := rec[k]; !exists {
				rec[k] = v
			}
		}
	}

	// inject typeField if hidden requested
	if params != nil && params.Hidden != nil && *params.Hidden {
		if _, ok := rec[m.typeField]; !ok && !m.generic {
			rec[m.typeField] = m.Name
		}
	}

	return rec
}

func (m *Model) transformReadAttribute(field *preparedField, name string, value any, params *Params, properties Item) any {
	switch field.Type {
	case FieldTypeDate:
		if value != nil {
			if field.Def.TTL {
				switch v := value.(type) {
				case float64:
					return time.Unix(int64(v), 0).UTC()
				case int64:
					return time.Unix(v, 0).UTC()
				}
			}
			switch v := value.(type) {
			case string:
				t, err := time.Parse(time.RFC3339Nano, v)
				if err == nil {
					return t
				}
				// try epoch millis as string
				if ms, err2 := strconv.ParseInt(v, 10, 64); err2 == nil {
					return time.UnixMilli(ms).UTC()
				}
				return v
			case float64:
				return time.UnixMilli(int64(v)).UTC()
			case int64:
				return time.UnixMilli(v).UTC()
			}
		}
	case FieldTypeBuffer, FieldTypeArrayBuffer, FieldTypeBinary:
		if s, ok := value.(string); ok {
			return []byte(s) // base64 decoded by attributevalue library
		}
	}
	return value
}

// ─── prepareProperties ───────────────────────────────────────────────────────

// prepareProperties validates and maps properties before building an expression.
func (m *Model) prepareProperties(ctx context.Context, op string, properties Item, params *Params) (Item, error) {
	delete(params.Batch, "fallback")
	params.fallback = false

	index := m.selectIndex(params)

	if m.needsFallback(op, index, params) {
		params.fallback = true
		return properties, nil
	}

	rec, err := m.collectProperties(ctx, op, "", &m.block, index, properties, params, nil)
	if err != nil {
		return nil, err
	}
	if params.fallback {
		return properties, nil
	}

	// ensure hash key is present for non-scan ops
	if op != "scan" && m.getHashValue(rec, m.block.Fields, index) == nil {
		return nil, NewError(fmt.Sprintf(`Cannot %s data for "%s". Missing data index key.`, op, m.Name),
			WithCode(ErrMissing), WithContext(map[string]any{"properties": properties}))
	}

	return rec, nil
}

// collectProperties processes one schema level recursively.
func (m *Model) collectProperties(ctx context.Context, op, pathname string, block *fieldBlock,
	index *IndexDef, properties Item, params *Params, context Item) (Item, error) {

	fields := block.Fields
	rec := Item{}

	if context == nil {
		if params.Context != nil {
			// params.Context is set via WithContext but not used here (it's the Go context)
		}
		context = m.table.context
	}

	// nested schemas first
	if m.nested && !keysOnlyOp(op) {
		if err := m.collectNested(ctx, op, pathname, fields, index, properties, params, context, rec); err != nil {
			return nil, err
		}
	}

	m.addContext(op, fields, index, properties, params, context)
	m.setDefaults(op, fields, properties, params)
	if err := m.runTemplates(op, pathname, index, block.Deps, properties, params); err != nil {
		return nil, err
	}
	m.convertNulls(op, pathname, fields, properties, params)
	if err := m.validateProperties(op, fields, properties, params); err != nil {
		return nil, err
	}
	m.selectProperties(op, block, index, properties, params, rec)
	m.transformProperties(op, fields, properties, params, rec)

	return rec, nil
}

func (m *Model) collectNested(ctx context.Context, op, pathname string, fields map[string]*preparedField,
	index *IndexDef, properties Item, params *Params, context Item, rec Item) error {

	for _, field := range fields {
		if field.Block == nil {
			continue
		}
		name := field.Name
		value, _ := properties[name]
		if op == "put" && value == nil {
			if field.Required {
				if field.Type == FieldTypeArray {
					value = []any{}
				} else {
					value = Item{}
				}
			} else if field.Def.Default != nil {
				value = field.Def.Default
			}
		}
		ctx2, _ := context[name].(Item)
		partial := m.getPartial(field, params)

		if value == nil {
			continue
		}

		if field.IsArray {
			if arr, ok := value.([]any); ok {
				result := make([]any, 0, len(arr))
				for i, elem := range arr {
					path := fmt.Sprintf("%s[%d]", name, i)
					if pathname != "" {
						path = pathname + "." + path
					}
					elemMap, _ := elem.(Item)
					obj, err := m.collectProperties(ctx, op, path, field.Block, index, elemMap, params, ctx2)
					if err != nil {
						return err
					}
					if !partial || len(obj) > 0 || field.Def.Default != nil {
						result = append(result, obj)
					}
				}
				rec[name] = result
			}
		} else {
			valMap, _ := value.(Item)
			path := name
			if pathname != "" {
				path = pathname + "." + name
			}
			obj, err := m.collectProperties(ctx, op, path, field.Block, index, valMap, params, ctx2)
			if err != nil {
				return err
			}
			if !partial || len(obj) > 0 || field.Def.Default != nil {
				rec[name] = obj
			}
		}
	}
	return nil
}

// addContext injects table/request context values into properties.
func (m *Model) addContext(op string, fields map[string]*preparedField, index *IndexDef, properties Item, params *Params, context Item) {
	for _, field := range fields {
		if field.Block != nil {
			continue
		}
		if op == "put" || (field.Attribute[0] != index.Hash && field.Attribute[0] != index.Sort) {
			if v, ok := context[field.Name]; ok {
				properties[field.Name] = v
			}
		}
	}
	if !m.generic && fields != nil {
		properties[m.typeField] = m.Name
	}
}

// setDefaults sets default values for put/init or upsert.
func (m *Model) setDefaults(op string, fields map[string]*preparedField, properties Item, params *Params) {
	if op != "put" && op != "init" && !(op == "update" && params != nil && params.Exists == nil) {
		return
	}
	for _, field := range fields {
		if field.Block != nil {
			continue
		}
		name := field.Name
		if _, ok := properties[name]; ok {
			continue
		}
		if field.ValueTemplate != "" {
			continue
		}
		if field.Def.Default != nil {
			properties[name] = field.Def.Default
		} else if op == "init" {
			if field.Def.Generate == "" {
				properties[name] = nil
			}
		} else if gen := field.Def.Generate; gen != "" {
			properties[name] = m.table.generate(gen)
		}
	}
}

// runTemplates expands value templates in dependency order.
func (m *Model) runTemplates(op, pathname string, index *IndexDef, deps []*preparedField, properties Item, params *Params) error {
	for _, field := range deps {
		if field.Block != nil {
			continue
		}
		name := field.Name
		if field.IsIndexed && op != "put" && op != "update" {
			if field.Attribute[0] != index.Hash && field.Attribute[0] != index.Sort {
				continue
			}
		}
		if field.ValueTemplate == "" {
			continue
		}
		if _, ok := properties[name]; ok {
			continue
		}
		val, err := m.runTemplate(op, index, field, properties, params, field.ValueTemplate)
		if err != nil {
			return err
		}
		if val != nil {
			properties[name] = val
		}
	}
	return nil
}

// runTemplate expands a single value template string.
func (m *Model) runTemplate(op string, index *IndexDef, field *preparedField, properties Item, params *Params, tmpl string) (any, error) {
	re := regexp.MustCompile(`\$\{(.*?)\}`)
	result := re.ReplaceAllStringFunc(tmpl, func(match string) string {
		inner := match[2 : len(match)-1] // strip ${ and }
		parts := strings.SplitN(inner, ":", 3)
		varName := parts[0]

		v := getPropValue(properties, varName)
		if v == nil {
			return match // unresolved – keep placeholder
		}

		var s string
		switch tv := v.(type) {
		case time.Time:
			if field.IsoDates || m.table.isoDates {
				s = tv.UTC().Format(time.RFC3339Nano)
			} else {
				s = strconv.FormatInt(tv.UnixMilli(), 10)
			}
		default:
			s = fmt.Sprintf("%v", tv)
		}

		// optional padding: ${var:len:pad}
		if len(parts) >= 2 {
			length, _ := strconv.Atoi(parts[1])
			pad := "0"
			if len(parts) >= 3 {
				pad = parts[2]
			}
			for len(s) < length {
				s = pad + s
			}
		}
		return s
	})

	// unresolved variables remain?
	if strings.Contains(result, "${") {
		if index != nil && field.Attribute[0] == index.Sort && op == "find" {
			// strip from first unresolved ${ onward, use prefix for begins_with
			idx := strings.Index(result, "${")
			prefix := result[:idx]
			if prefix != "" {
				return map[string]any{"begins": prefix}, nil
			}
		}
		return nil, nil // not yet resolvable
	}
	return result, nil
}

// convertNulls removes null properties unless nulls==true; adds to params.Remove.
func (m *Model) convertNulls(op, pathname string, fields map[string]*preparedField, properties Item, params *Params) {
	for name, value := range properties {
		field := fields[name]
		if field == nil || field.Block != nil {
			continue
		}
		if value == nil && !field.Nulls {
			if field.Required {
				continue // validation will catch
			}
			delete(properties, name)
			path := name
			if pathname != "" {
				path = pathname + "." + name
			}
			params.Remove = append(params.Remove, path)
		}
	}
}

// validateProperties checks required fields, regex, enum constraints.
func (m *Model) validateProperties(op string, fields map[string]*preparedField, properties Item, params *Params) error {
	if op != "put" && op != "update" {
		return nil
	}
	validation := map[string]string{}

	for name, value := range properties {
		field := fields[name]
		if field == nil || field.Block != nil {
			continue
		}
		if field.Def.Validate != "" || field.Def.Enum != nil {
			if err := m.validateProperty(field, value, validation, params); err != nil {
				return err
			}
			properties[name] = value
		}
	}
	// required check
	for _, field := range fields {
		if field.Required && field.Block == nil {
			v, exists := properties[field.Name]
			if op == "put" && (!exists || v == nil) {
				validation[field.Name] = fmt.Sprintf(`Value not defined for required field "%s"`, field.Name)
			} else if op == "update" && v == nil && exists {
				validation[field.Name] = fmt.Sprintf(`Value not defined for required field "%s"`, field.Name)
			}
		}
	}
	if len(validation) > 0 {
		keys := make([]string, 0, len(validation))
		for k := range validation {
			keys = append(keys, k)
		}
		return NewError(fmt.Sprintf(`Validation Error in "%s" for "%s"`, m.Name, strings.Join(keys, ", ")),
			WithCode(ErrValidation), WithContext(map[string]any{"validation": validation}))
	}
	return nil
}

func (m *Model) validateProperty(field *preparedField, value any, details map[string]string, params *Params) error {
	name := field.Name
	if field.Def.Validate != "" {
		pat := field.Def.Validate
		s, _ := value.(string)
		// parse /pattern/flags
		if strings.HasPrefix(pat, "/") {
			last := strings.LastIndex(pat, "/")
			if last > 0 {
				flags := pat[last+1:]
				inner := pat[1:last]
				if flags != "" {
					inner = "(?" + flags + ")" + inner
				}
				re, err := regexp.Compile(inner)
				if err == nil {
					if !re.MatchString(s) {
						details[name] = fmt.Sprintf(`Bad value "%v" for "%s"`, value, name)
					}
				}
			}
		} else {
			re, err := regexp.Compile(pat)
			if err == nil {
				if !re.MatchString(s) {
					details[name] = fmt.Sprintf(`Bad value "%v" for "%s"`, value, name)
				}
			}
		}
	}
	if field.Def.Enum != nil {
		s := fmt.Sprintf("%v", value)
		if !containsStr(field.Def.Enum, s) {
			details[name] = fmt.Sprintf(`Bad value "%v" for "%s"`, value, name)
		}
	}
	return nil
}

// selectProperties picks which properties go into the DynamoDB command.
func (m *Model) selectProperties(op string, block *fieldBlock, index *IndexDef, properties Item, params *Params, rec Item) {
	project := m.getProjection(index)

	for name, field := range block.Fields {
		if field.Block != nil {
			continue
		}
		omit := false

		if block == &m.block {
			att := field.Attribute[0]

			// missing sort key → fallback
			if properties[name] == nil && att == index.Sort && params.High && keysOnlyOp(op) {
				if op == "delete" && !params.Many {
					// hard error for delete without sort
					// handled by caller via fallback
				}
				params.fallback = true
				return
			}

			if keysOnlyOp(op) && att != index.Hash && att != index.Sort && !m.hasUniqueFields {
				omit = true
			} else if project != nil && !containsStr(project, att) {
				omit = true
			} else if name == m.typeField && name != index.Hash && name != index.Sort && op == "find" {
				omit = true
			} else if field.Def.Encode != nil {
				omit = true
			}
		}

		if !omit {
			if v, ok := properties[name]; ok {
				rec[name] = v
			}
		}
	}

	if block == &m.block {
		m.addProjectedProperties(op, properties, params, project, rec)
	}
}

func (m *Model) getProjection(index *IndexDef) []string {
	if index.Project == nil {
		return nil
	}
	switch p := index.Project.(type) {
	case string:
		if p == "all" {
			return nil
		}
		if p == "keys" {
			primary := m.indexes["primary"]
			keys := []string{primary.Hash, primary.Sort, index.Hash, index.Sort}
			return unique(keys)
		}
	case []string:
		primary := m.indexes["primary"]
		all := append(p, primary.Hash, primary.Sort, index.Hash, index.Sort)
		return unique(all)
	case []any:
		strs := make([]string, 0, len(p))
		for _, v := range p {
			if s, ok := v.(string); ok {
				strs = append(strs, s)
			}
		}
		primary := m.indexes["primary"]
		all := append(strs, primary.Hash, primary.Sort, index.Hash, index.Sort)
		return unique(all)
	}
	return nil
}

func (m *Model) addProjectedProperties(op string, properties Item, params *Params, project []string, rec Item) {
	generic := m.generic
	if !generic || keysOnlyOp(op) {
		return
	}
	for name, value := range properties {
		if project != nil && !containsStr(project, name) {
			continue
		}
		if _, exists := rec[name]; exists {
			continue
		}
		if t, ok := value.(time.Time); ok {
			if m.table.isoDates {
				rec[name] = t.UTC().Format(time.RFC3339Nano)
			} else {
				rec[name] = t.UnixMilli()
			}
		} else {
			rec[name] = value
		}
	}
}

// transformProperties converts Go values to DynamoDB-compatible types before writing.
func (m *Model) transformProperties(op string, fields map[string]*preparedField, properties Item, params *Params, rec Item) {
	for name, field := range fields {
		if field.Block != nil {
			continue
		}
		v, ok := rec[name]
		if !ok {
			continue
		}
		rec[name] = m.transformWriteAttribute(op, field, v, properties, params)
	}
}

func (m *Model) transformWriteAttribute(op string, field *preparedField, value any, properties Item, params *Params) any {
	if value == nil && field.Nulls {
		return nil
	}
	switch field.Type {
	case FieldTypeDate:
		if value != nil {
			return m.transformWriteDate(field, value)
		}
	case FieldTypeNumber:
		switch v := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return v
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				panic(fmt.Sprintf("invalid number value %q for field %s", v, field.Name))
			}
			return f
		}
	case FieldTypeBoolean:
		switch v := value.(type) {
		case bool:
			return v
		case string:
			return v != "false" && v != "null" && v != "undefined" && v != ""
		}
		return value != nil
	case FieldTypeString:
		if value != nil {
			// operator map (e.g. {begins: "prefix"}) — pass through for key conditions
			if _, ok := value.(map[string]any); ok {
				return value
			}
			return fmt.Sprintf("%v", value)
		}
	case FieldTypeBuffer, FieldTypeArrayBuffer, FieldTypeBinary:
		if b, ok := value.([]byte); ok {
			return b
		}
	case FieldTypeArray:
		if value != nil {
			if arr, ok := value.([]any); ok {
				return m.transformNestedWriteFields(field, arr)
			}
		}
	case FieldTypeObject:
		if value != nil {
			if obj, ok := value.(map[string]any); ok {
				return m.transformNestedWriteFieldsMap(field, obj)
			}
		}
	}

	if field.Def.Crypt && value != nil {
		if s, ok := value.(string); ok {
			enc, err := m.table.encrypt(s)
			if err == nil {
				return enc
			}
		}
	}
	return value
}

func (m *Model) transformNestedWriteFields(field *preparedField, arr []any) []any {
	for i, v := range arr {
		switch tv := v.(type) {
		case time.Time:
			arr[i] = m.transformWriteDate(field, tv)
		case map[string]any:
			arr[i] = m.transformNestedWriteFieldsMap(field, tv)
		}
	}
	return arr
}

func (m *Model) transformNestedWriteFieldsMap(field *preparedField, obj map[string]any) map[string]any {
	for k, v := range obj {
		switch tv := v.(type) {
		case time.Time:
			obj[k] = m.transformWriteDate(field, tv)
		case map[string]any:
			obj[k] = m.transformNestedWriteFieldsMap(field, tv)
		case []any:
			obj[k] = m.transformNestedWriteFields(field, tv)
		}
	}
	return obj
}

func (m *Model) transformWriteDate(field *preparedField, value any) any {
	isoDates := field.IsoDates
	if field.Def.TTL {
		switch v := value.(type) {
		case time.Time:
			return v.Unix()
		case string:
			t, _ := time.Parse(time.RFC3339, v)
			return t.Unix()
		case float64:
			return int64(math.Ceil(v / 1000))
		}
		return value
	}
	if isoDates {
		switch v := value.(type) {
		case time.Time:
			return v.UTC().Format(time.RFC3339Nano)
		case string:
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return v
			}
			return t.UTC().Format(time.RFC3339Nano)
		case float64:
			return time.UnixMilli(int64(v)).UTC().Format(time.RFC3339Nano)
		}
	} else {
		switch v := value.(type) {
		case time.Time:
			return v.UnixMilli()
		case string:
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				if ms, err2 := strconv.ParseInt(v, 10, 64); err2 == nil {
					return ms
				}
				return v
			}
			return t.UnixMilli()
		case float64:
			return int64(v)
		}
	}
	return value
}

// ─── unique / unique-update helpers ─────────────────────────────────────────

func (m *Model) createUnique(ctx context.Context, properties Item, params *Params) (Item, error) {
	transactHere := params.Transaction == nil
	if params.Transaction == nil {
		params.Transaction = map[string]any{}
	}
	now := time.Now()
	params.Transaction["timestamp"] = now

	ts := m.table.timestamps
	if ts == true || ts == "create" {
		properties[m.createdField] = now
	}
	if ts == true || ts == "update" {
		properties[m.updatedField] = now
	}

	var err error
	properties, err = m.prepareProperties(ctx, "put", properties, params)
	if err != nil {
		return nil, err
	}
	params.prepared = true

	primary := m.indexes["primary"]
	fields := m.block.Fields

	var uniqueFields []*preparedField
	for _, f := range fields {
		if f.Def.Unique && f.Attribute[0] != primary.Hash && f.Attribute[0] != primary.Sort {
			uniqueFields = append(uniqueFields, f)
		}
	}

	for _, field := range uniqueFields {
		if v, ok := properties[field.Name]; ok && v != nil {
			pk := fmt.Sprintf("_unique#%s#%s#%v", m.Name, field.Attribute[0], v)
			sk := "_unique#"
			up := Item{primary.Hash: pk, primary.Sort: sk}
			_, err := m.getSchemaMgr().uniqueModel.Create(ctx, up, &Params{Transaction: params.Transaction, Exists: boolPtr(false), Return: "NONE"})
			if err != nil {
				return nil, err
			}
		}
	}

	item, err := m.putItem(ctx, properties, params)
	if err != nil {
		return nil, err
	}
	if !transactHere {
		return item, nil
	}
	expr := params.expression
	_, err = m.table.Transact(ctx, "write", params.Transaction, params)
	if err != nil {
		if isConditionalFailed(err) {
			names := make([]string, 0, len(uniqueFields))
			for _, f := range uniqueFields {
				names = append(names, f.Name)
			}
			return nil, NewError(fmt.Sprintf(`Cannot create unique attributes "%s" for "%s". An item of the same name already exists.`,
				strings.Join(names, ", "), m.Name), WithCode(ErrUnique))
		}
		return nil, err
	}
	items, err := m.parseResponse(ctx, "put", expr, nil)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0], nil
}

func (m *Model) removeUnique(ctx context.Context, properties Item, params *Params) (Item, error) {
	transactHere := params.Transaction == nil
	if params.Transaction == nil {
		params.Transaction = map[string]any{}
	}

	primary := m.indexes["primary"]
	fields := m.block.Fields

	var uniqueFields []*preparedField
	for _, f := range fields {
		if f.Def.Unique && f.Attribute[0] != primary.Hash && f.Attribute[0] != primary.Sort {
			uniqueFields = append(uniqueFields, f)
		}
	}

	params.prepared = true
	var err error
	properties, err = m.prepareProperties(ctx, "delete", properties, params)
	if err != nil {
		return nil, err
	}

	keys := Item{primary.Hash: properties[primary.Hash]}
	if primary.Sort != "" {
		keys[primary.Sort] = properties[primary.Sort]
	}

	prior, err := m.Get(ctx, keys, &Params{Hidden: boolPtr(true)})
	if err != nil {
		return nil, err
	}
	if prior == nil {
		if params.Exists == nil || *params.Exists {
			return nil, NewError("Cannot find existing item to remove", WithCode(ErrNotFound))
		}
	}
	if prior != nil {
		var err2 error
		prior, err2 = m.prepareProperties(ctx, "update", prior, &Params{})
		if err2 != nil {
			return nil, err2
		}
	}

	for _, field := range uniqueFields {
		sk := "_unique#"
		if prior != nil {
			if v, ok := prior[field.Name]; ok && v != nil {
				pk := fmt.Sprintf("_unique#%s#%s#%v", m.Name, field.Attribute[0], v)
				_, err := m.getSchemaMgr().uniqueModel.Remove(ctx, Item{primary.Hash: pk, primary.Sort: sk},
					&Params{Transaction: params.Transaction})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	removed, err := m.deleteItem(ctx, properties, params)
	if err != nil {
		return nil, err
	}
	if transactHere {
		_, err = m.table.Transact(ctx, "write", params.Transaction, params)
		if err != nil {
			return nil, err
		}
	}
	return removed, nil
}

func (m *Model) updateUnique(ctx context.Context, properties Item, params *Params) (Item, error) {
	transactHere := params.Transaction == nil
	if params.Transaction == nil {
		params.Transaction = map[string]any{}
	}

	primary := m.indexes["primary"]
	fields := m.block.Fields

	var err error
	properties, err = m.prepareProperties(ctx, "update", properties, params)
	if err != nil {
		return nil, err
	}
	params.prepared = true

	keys := Item{primary.Hash: properties[primary.Hash]}
	if primary.Sort != "" {
		keys[primary.Sort] = properties[primary.Sort]
	}

	prior, err := m.Get(ctx, keys, &Params{Hidden: boolPtr(true)})
	if err != nil {
		return nil, err
	}
	if prior == nil {
		// Exists == nil means upsert (ok to create); Exists == true means must exist
		if params.Exists != nil && *params.Exists {
			return nil, NewError("Cannot find existing item to update", WithCode(ErrNotFound))
		}
	}
	if prior != nil {
		prior, err = m.prepareProperties(ctx, "update", prior, &Params{})
		if err != nil {
			return nil, err
		}
	}

	for _, field := range fields {
		if !field.Def.Unique || field.Attribute[0] == primary.Hash || field.Attribute[0] == primary.Sort {
			continue
		}
		toBeRemoved := containsStr(params.Remove, field.Name)
		var newVal, priorVal any
		newVal = properties[field.Name]
		if prior != nil {
			priorVal = prior[field.Name]
		}
		// If newVal is nil and the field is not explicitly being removed, treat as unchanged
		if newVal == nil && !toBeRemoved {
			continue
		}
		isUnchanged := fmt.Sprintf("%v", newVal) == fmt.Sprintf("%v", priorVal)
		if isUnchanged {
			continue
		}
		sk := "_unique#"
		if prior != nil && priorVal != nil {
			priorPk := fmt.Sprintf("_unique#%s#%s#%v", m.Name, field.Attribute[0], priorVal)
			if newVal != nil {
				newPk := fmt.Sprintf("_unique#%s#%s#%v", m.Name, field.Attribute[0], newVal)
				if priorPk == newPk {
					continue
				}
			}
			m.getSchemaMgr().uniqueModel.Remove(ctx, Item{primary.Hash: priorPk, primary.Sort: sk}, //nolint:errcheck
				&Params{Transaction: params.Transaction})
		}
		if newVal != nil && !toBeRemoved {
			pk := fmt.Sprintf("_unique#%s#%s#%v", m.Name, field.Attribute[0], newVal)
			up := Item{primary.Hash: pk, primary.Sort: sk}
			m.getSchemaMgr().uniqueModel.Create(ctx, up, &Params{Transaction: params.Transaction, Exists: boolPtr(false), Return: "NONE"}) //nolint:errcheck
		}
	}

	item, err := m.updateItem(ctx, properties, params)
	if err != nil {
		return nil, err
	}
	if !transactHere {
		return item, nil
	}
	_, err = m.table.Transact(ctx, "write", params.Transaction, params)
	if err != nil {
		if isConditionalFailed(err) {
			return nil, NewError(fmt.Sprintf(`Cannot update unique attributes for "%s"`, m.Name), WithCode(ErrUnique))
		}
		return nil, err
	}
	return item, nil
}

func (m *Model) removeByFind(ctx context.Context, properties Item, params *Params) (Item, error) {
	findParams := *params
	findParams.Parse = true
	delete(findParams.Transaction, "")
	items, err := m.Find(ctx, properties, &findParams)
	if err != nil {
		return nil, err
	}
	if len(items.Items) > 1 && !params.Many {
		return nil, NewError(fmt.Sprintf(`Removing multiple items from "%s". Use many:true to enable.`, m.Name),
			WithCode(ErrNonUnique))
	}
	var last Item
	for _, item := range items.Items {
		var removed Item
		if m.hasUniqueFields {
			removed, err = m.removeUnique(ctx, item, &Params{Transaction: params.Transaction})
		} else {
			removed, err = m.Remove(ctx, item, &Params{Transaction: params.Transaction, Return: params.Return})
		}
		if err != nil {
			return nil, err
		}
		last = removed
	}
	return last, nil
}

// ─── batch / transaction accumulation ───────────────────────────────────────

func (m *Model) accumulateBatch(op string, cmd Item, expr *expression) (Item, error) {
	b := expr.params.Batch
	ritems, _ := b["RequestItems"].(map[string]any)
	if ritems == nil {
		ritems = map[string]any{}
		b["RequestItems"] = ritems
	}
	switch op {
	case "get":
		tbl, _ := ritems[m.tableName].(map[string]any)
		if tbl == nil {
			tbl = map[string]any{"Keys": []any{}}
			ritems[m.tableName] = tbl
		}
		keys, _ := tbl["Keys"].([]any)
		tbl["Keys"] = append(keys, cmd["Key"])
	default:
		list, _ := ritems[m.tableName].([]any)
		bop := batchOpName(op)
		ritems[m.tableName] = append(list, map[string]any{bop: cmd})
	}
	return m.transformReadItem(op, expr.properties, expr.properties, expr.params, expr), nil
}

func batchOpName(op string) string {
	switch op {
	case "delete":
		return "DeleteRequest"
	default:
		return "PutRequest"
	}
}

func (m *Model) accumulateTransaction(op string, cmd Item, expr *expression) (Item, error) {
	topMap := map[string]string{
		"delete": "Delete", "get": "Get", "put": "Put", "update": "Update", "check": "ConditionCheck",
	}
	top, ok := topMap[op]
	if !ok {
		return nil, NewArgError("Unknown transaction operation: " + op)
	}
	t := expr.params.Transaction
	items, _ := t["TransactItems"].([]any)
	t["TransactItems"] = append(items, map[string]any{top: cmd})
	expr.params.expression = expr
	return m.transformReadItem(op, expr.properties, expr.properties, expr.params, expr), nil

}

// ─── follow ──────────────────────────────────────────────────────────────────

func shouldFollow(params *Params, index *IndexDef) bool {
	if params.Follow != nil {
		return *params.Follow
	}
	return index.Follow
}

func (m *Model) followItems(ctx context.Context, op string, items []Item, params *Params) ([]Item, error) {
	if op != "find" {
		return items, nil
	}
	p2 := *params
	p2.Follow = nil
	p2.Index = ""
	results := make([]Item, 0, len(items))
	sem := make(chan struct{}, followThreads)
	errs := make(chan error, len(items))
	out := make([]Item, len(items))
	for i, item := range items {
		i, item := i, item
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			got, err := m.Get(ctx, item, &p2)
			if err != nil {
				errs <- err
				return
			}
			out[i] = got
		}()
	}
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(errs)
	for e := range errs {
		if e != nil {
			return nil, e
		}
	}
	for _, item := range out {
		if item != nil {
			results = append(results, item)
		}
	}
	return results, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func (m *Model) checkArgs(ctx context.Context, properties Item, params *Params, overrides *Params) (Item, *Params) {
	if params != nil && params.checked {
		return properties, params
	}
	merged := &Params{}
	if overrides != nil {
		*merged = *overrides
	}
	if params != nil {
		// params fields override overrides (caller wins for most)
		if params.Execute != nil {
			merged.Execute = params.Execute
		}
		if params.Log != nil {
			merged.Log = params.Log
		}
		if params.Parse {
			merged.Parse = params.Parse
		}
		if params.High {
			merged.High = params.High
		}
		if params.Exists != nil {
			merged.Exists = params.Exists
		}
		if params.Hidden != nil {
			merged.Hidden = params.Hidden
		}
		if params.Partial != nil {
			merged.Partial = params.Partial
		}
		if params.Limit > 0 {
			merged.Limit = params.Limit
		}
		if params.Next != nil {
			merged.Next = params.Next
		}
		if params.Prev != nil {
			merged.Prev = params.Prev
		}
		if params.Reverse {
			merged.Reverse = params.Reverse
		}
		if params.MaxPages > 0 {
			merged.MaxPages = params.MaxPages
		}
		if params.Index != "" {
			merged.Index = params.Index
		}
		if params.Fields != nil {
			merged.Fields = params.Fields
		}
		if params.Consistent {
			merged.Consistent = params.Consistent
		}
		if params.Return != nil {
			merged.Return = params.Return
		}
		if params.Where != "" {
			merged.Where = params.Where
		}
		if params.Set != nil {
			merged.Set = params.Set
		}
		if params.Add != nil {
			merged.Add = params.Add
		}
		if params.Remove != nil {
			merged.Remove = params.Remove
		}
		if params.Delete != nil {
			merged.Delete = params.Delete
		}
		if params.Push != nil {
			merged.Push = params.Push
		}
		if params.Substitutions != nil {
			merged.Substitutions = params.Substitutions
		}
		if params.Count {
			merged.Count = params.Count
		}
		if params.Select != "" {
			merged.Select = params.Select
		}
		if params.Stats != nil {
			merged.Stats = params.Stats
		}
		if params.Capacity != "" {
			merged.Capacity = params.Capacity
		}
		if params.Batch != nil {
			merged.Batch = params.Batch
		}
		if params.Transaction != nil {
			merged.Transaction = params.Transaction
		}
		if params.Follow != nil {
			merged.Follow = params.Follow
		}
		if params.Many {
			merged.Many = params.Many
		}
		if params.Segments > 0 {
			merged.Segments = params.Segments
		}
		if params.Segment > 0 {
			merged.Segment = params.Segment
		}
		if params.PostFormat != nil {
			merged.PostFormat = params.PostFormat
		}
		if params.Client != nil {
			merged.Client = params.Client
		}
		if params.Context != nil {
			merged.Context = params.Context
		}
	}
	merged.checked = true
	// deep clone properties so we don't pollute caller's map
	clone := make(Item, len(properties))
	for k, v := range properties {
		clone[k] = v
	}
	return clone, merged
}

func (m *Model) selectIndex(params *Params) *IndexDef {
	if params != nil && params.Index != "" && params.Index != "primary" {
		if idx, ok := m.indexes[params.Index]; ok {
			return idx
		}
	}
	return m.indexes["primary"]
}

func (m *Model) needsFallback(op string, index *IndexDef, params *Params) bool {
	primary := m.indexes["primary"]
	if index != primary && op != "find" && op != "scan" {
		return true
	}
	return false
}

func (m *Model) getHashValue(rec Item, fields map[string]*preparedField, index *IndexDef) any {
	if m.generic {
		return rec[index.Hash]
	}
	for _, field := range fields {
		if field.Attribute[0] == index.Hash {
			return rec[field.Name]
		}
	}
	return nil
}

func (m *Model) getPartial(field *preparedField, params *Params) bool {
	if params != nil && params.Partial != nil {
		return *params.Partial
	}
	if field.Partial != nil {
		return *field.Partial
	}
	return m.partial
}

// ─── small utilities ─────────────────────────────────────────────────────────

func keysOnlyOp(op string) bool { return op == "delete" || op == "get" }

func reverseItems(s []Item) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func boolPtr(b bool) *bool { return &b }

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func unique(s []string) []string {
	seen := map[string]bool{}
	out := s[:0]
	for _, v := range s {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	}
	return 0
}

func toIntVal(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case int64:
		return int(n), true
	}
	return 0, false
}

func toSlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []string:
		out := make([]any, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out, true
	}
	return nil, false
}

func getPropValue(m map[string]any, path string) any {
	v := any(m)
	for _, part := range strings.Split(path, ".") {
		if cur, ok := v.(map[string]any); ok {
			v = cur[part]
		} else {
			return nil
		}
	}
	return v
}

func isConditionalFailed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "ConditionalCheckFailed") ||
		strings.Contains(msg, "TransactionCanceledException") ||
		strings.Contains(msg, "Transaction Cancelled") {
		return true
	}
	// check wrapped cause
	if ote, ok := err.(*OneTableError); ok && ote.Cause != nil {
		causeMsg := ote.Cause.Error()
		return strings.Contains(causeMsg, "ConditionalCheckFailed") ||
			strings.Contains(causeMsg, "TransactionCanceledException")
	}
	return false
}

// marshallForDynamo converts a Go Item to DynamoDB AttributeValue map.
func marshallForDynamo(item Item) (map[string]types.AttributeValue, error) {
	return attributevalue.MarshalMap(item)
}

// unmarshallFromDynamo converts DynamoDB AttributeValue map to Go Item.
func unmarshallFromDynamo(av map[string]types.AttributeValue) (Item, error) {
	var item Item
	if err := attributevalue.UnmarshalMap(av, &item); err != nil {
		return nil, err
	}
	return item, nil
}
