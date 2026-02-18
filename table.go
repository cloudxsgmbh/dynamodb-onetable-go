/*
Package onetable – Table type.

Mirrors JS: Table.js – top-level DynamoDB table wrapper.
*/
package onetable

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	uid "github.com/cloudxsgmbh/dynamodb-onetable-go/internal/uid"
)

// DynamoClient is the interface satisfied by both the real AWS DynamoDB client
// and any test doubles / local stubs.
type DynamoClient interface {
	// Core operations
	GetItem(ctx context.Context, params *ddb.GetItemInput, optFns ...func(*ddb.Options)) (*ddb.GetItemOutput, error)
	PutItem(ctx context.Context, params *ddb.PutItemInput, optFns ...func(*ddb.Options)) (*ddb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *ddb.DeleteItemInput, optFns ...func(*ddb.Options)) (*ddb.DeleteItemOutput, error)
	UpdateItem(ctx context.Context, params *ddb.UpdateItemInput, optFns ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error)
	Query(ctx context.Context, params *ddb.QueryInput, optFns ...func(*ddb.Options)) (*ddb.QueryOutput, error)
	Scan(ctx context.Context, params *ddb.ScanInput, optFns ...func(*ddb.Options)) (*ddb.ScanOutput, error)

	// Batch
	BatchGetItem(ctx context.Context, params *ddb.BatchGetItemInput, optFns ...func(*ddb.Options)) (*ddb.BatchGetItemOutput, error)
	BatchWriteItem(ctx context.Context, params *ddb.BatchWriteItemInput, optFns ...func(*ddb.Options)) (*ddb.BatchWriteItemOutput, error)

	// Transact
	TransactGetItems(ctx context.Context, params *ddb.TransactGetItemsInput, optFns ...func(*ddb.Options)) (*ddb.TransactGetItemsOutput, error)
	TransactWriteItems(ctx context.Context, params *ddb.TransactWriteItemsInput, optFns ...func(*ddb.Options)) (*ddb.TransactWriteItemsOutput, error)

	// DDL
	CreateTable(ctx context.Context, params *ddb.CreateTableInput, optFns ...func(*ddb.Options)) (*ddb.CreateTableOutput, error)
	DeleteTable(ctx context.Context, params *ddb.DeleteTableInput, optFns ...func(*ddb.Options)) (*ddb.DeleteTableOutput, error)
	UpdateTable(ctx context.Context, params *ddb.UpdateTableInput, optFns ...func(*ddb.Options)) (*ddb.UpdateTableOutput, error)
	DescribeTable(ctx context.Context, params *ddb.DescribeTableInput, optFns ...func(*ddb.Options)) (*ddb.DescribeTableOutput, error)
	ListTables(ctx context.Context, params *ddb.ListTablesInput, optFns ...func(*ddb.Options)) (*ddb.ListTablesOutput, error)
	UpdateTimeToLive(ctx context.Context, params *ddb.UpdateTimeToLiveInput, optFns ...func(*ddb.Options)) (*ddb.UpdateTimeToLiveOutput, error)
}

// CryptoConfig configures field-level encryption.
type CryptoConfig struct {
	Password string // plaintext password → hashed to AES-256 key
	Cipher   string // e.g. "aes-256-gcm"
}

// TableParams configures a Table.
type TableParams struct {
	Name    string
	Client  DynamoClient
	Schema  *SchemaDef
	Logger  Logger // nil → default (info+error only)
	Verbose bool   // true → also log trace/data
	Hidden  bool   // return hidden fields by default
	Partial bool   // allow partial nested updates
	Warn    bool   // log warnings for schema mismatches
	Crypto  map[string]*CryptoConfig
	Context Item // table-level context (injected into every write)
	Metrics MetricsCollector
	Monitor MonitorFunc
	// Transform is called for every read/write to allow custom field transformations.
	Transform TransformFunc
	// Value is called when a field has value: true to compute a custom value.
	Value ValueFunc
}

// MetricsCollector is called after every DynamoDB operation.
type MetricsCollector interface {
	Add(model, op string, result Item, params *Params, start time.Time) error
	Flush() error
}

// MonitorFunc is an optional hook called after each DynamoDB operation.
type MonitorFunc func(model, op string, result Item, params *Params, start time.Time) error

// TransformFunc is called for read/write to allow field-level transformations.
type TransformFunc func(model *Model, op, name string, value any, properties Item) any

// ValueFunc computes a field value when field.Value == true.
type ValueFunc func(model *Model, name string, properties Item, params *Params) any

// Table represents a single DynamoDB table using the OneTable pattern.
type Table struct {
	Name string

	client DynamoClient
	log    Logger
	params *TableParams

	// schema-derived settings (set via setSchemaParams)
	typeField    string
	createdField string
	updatedField string
	separator    string
	isoDates     bool
	nulls        bool
	timestamps   any // bool | "create" | "update"
	warn         bool

	hidden  bool
	partial bool

	// crypto
	cryptoConfigs map[string]*cryptoEntry

	// table-level context applied to every write
	context Item

	// schema manager
	schemaMgr *schemaManager

	// optional metrics / monitoring
	metrics MetricsCollector
	monitor MonitorFunc
}

type cryptoEntry struct {
	name   string
	cipher string
	key    []byte // sha256 of password
}

// NewTable creates and initialises a Table instance.
func NewTable(params TableParams) (*Table, error) {
	if params.Name == "" {
		return nil, NewArgError("Missing \"name\" property")
	}

	t := &Table{
		Name:         params.Name,
		params:       &params,
		context:      Item{},
		hidden:       params.Hidden,
		partial:      params.Partial,
		warn:         params.Warn,
		typeField:    "_type",
		createdField: "created",
		updatedField: "updated",
		separator:    "#",
		isoDates:     false,
		nulls:        false,
		timestamps:   false,
		metrics:      params.Metrics,
		monitor:      params.Monitor,
	}

	// logging
	if params.Logger != nil {
		t.log = params.Logger
	} else if params.Verbose {
		t.log = verboseLogger{}
	} else {
		t.log = defaultLogger{}
	}

	// client
	if params.Client != nil {
		t.client = params.Client
	}

	// crypto
	if params.Crypto != nil {
		if err := t.initCrypto(params.Crypto); err != nil {
			return nil, err
		}
	}

	// schema manager (may be nil schema)
	t.schemaMgr = newSchemaManager(t, params.Schema)

	logTrace(t.log, "Loading OneTable", nil)
	return t, nil
}

// ─── Schema params ────────────────────────────────────────────────────────────

func (t *Table) setSchemaParams(p *SchemaParams) {
	if p == nil {
		return
	}
	if p.CreatedField != "" {
		t.createdField = p.CreatedField
	}
	if p.UpdatedField != "" {
		t.updatedField = p.UpdatedField
	}
	if p.TypeField != "" {
		t.typeField = p.TypeField
	}
	if p.Separator != "" {
		t.separator = p.Separator
	}
	t.isoDates = p.IsoDates
	t.nulls = p.Nulls
	if p.Timestamps != nil {
		t.timestamps = p.Timestamps
	}
	t.warn = p.Warn
}

func (t *Table) getSchemaParams() SchemaParams {
	return SchemaParams{
		CreatedField: t.createdField,
		UpdatedField: t.updatedField,
		TypeField:    t.typeField,
		Separator:    t.separator,
		IsoDates:     t.isoDates,
		Nulls:        t.nulls,
		Timestamps:   t.timestamps,
		Warn:         t.warn,
	}
}

// ─── Public schema API ────────────────────────────────────────────────────────

func (t *Table) SetSchema(schema *SchemaDef) map[string]*IndexDef {
	return t.schemaMgr.SetSchema(schema)
}

func (t *Table) GetCurrentSchema() *SchemaDef {
	return t.schemaMgr.GetCurrentSchema()
}

func (t *Table) GetKeys(ctx context.Context) (map[string]*IndexDef, error) {
	return t.schemaMgr.GetKeys(ctx, false)
}

func (t *Table) GetModel(name string) (*Model, error) {
	return t.schemaMgr.GetModel(name, false)
}

func (t *Table) AddModel(name string, fields FieldMap) {
	t.schemaMgr.AddModel(name, fields)
}

func (t *Table) RemoveModel(name string) error {
	return t.schemaMgr.RemoveModel(name)
}

func (t *Table) ListModels() []string {
	return t.schemaMgr.ListModels()
}

// ─── Context ──────────────────────────────────────────────────────────────────

func (t *Table) GetContext() Item { return t.context }

func (t *Table) SetContext(ctx Item, merge bool) *Table {
	if merge {
		for k, v := range ctx {
			t.context[k] = v
		}
	} else {
		t.context = ctx
	}
	return t
}

func (t *Table) AddContext(ctx Item) *Table {
	for k, v := range ctx {
		t.context[k] = v
	}
	return t
}

func (t *Table) ClearContext() *Table {
	t.context = Item{}
	return t
}

// ─── High-level model-factory API ────────────────────────────────────────────

func (t *Table) Create(ctx context.Context, modelName string, properties Item, params *Params) (Item, error) {
	m, err := t.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return m.Create(ctx, properties, params)
}

func (t *Table) Find(ctx context.Context, modelName string, properties Item, params *Params) (*Result, error) {
	m, err := t.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return m.Find(ctx, properties, params)
}

func (t *Table) Get(ctx context.Context, modelName string, properties Item, params *Params) (Item, error) {
	m, err := t.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return m.Get(ctx, properties, params)
}

func (t *Table) Remove(ctx context.Context, modelName string, properties Item, params *Params) (Item, error) {
	m, err := t.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return m.Remove(ctx, properties, params)
}

func (t *Table) Scan(ctx context.Context, modelName string, properties Item, params *Params) (*Result, error) {
	m, err := t.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return m.Scan(ctx, properties, params)
}

func (t *Table) Update(ctx context.Context, modelName string, properties Item, params *Params) (Item, error) {
	m, err := t.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return m.Update(ctx, properties, params)
}

func (t *Table) Upsert(ctx context.Context, modelName string, properties Item, params *Params) (Item, error) {
	m, err := t.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return m.Upsert(ctx, properties, params)
}

// ─── Low-level item API (mirrors JS table.getItem / putItem etc.) ─────────────

func (t *Table) GetItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	return t.schemaMgr.genericModel.getItem(ctx, properties, params)
}

func (t *Table) PutItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	return t.schemaMgr.genericModel.putItem(ctx, properties, params)
}

func (t *Table) DeleteItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	return t.schemaMgr.genericModel.deleteItem(ctx, properties, params)
}

func (t *Table) QueryItems(ctx context.Context, properties Item, params *Params) (*Result, error) {
	return t.schemaMgr.genericModel.queryItems(ctx, properties, params)
}

func (t *Table) ScanItems(ctx context.Context, properties Item, params *Params) (*Result, error) {
	return t.schemaMgr.genericModel.scanItems(ctx, properties, params)
}

func (t *Table) UpdateItem(ctx context.Context, properties Item, params *Params) (Item, error) {
	return t.schemaMgr.genericModel.updateItem(ctx, properties, params)
}

// ─── Batch operations ─────────────────────────────────────────────────────────

func (t *Table) BatchGet(ctx context.Context, batch map[string]any, params *Params) (any, error) {
	if len(batch) == 0 {
		return []Item{}, nil
	}
	if params == nil {
		params = &Params{}
	}

	ritems, _ := batch["RequestItems"].(map[string]any)
	def, _ := ritems[t.Name].(map[string]any)

	if params.Fields != nil {
		// build projection expression
		expr, err := newExpression(t.schemaMgr.genericModel, "batchGet", Item{}, params)
		if err == nil {
			cmd, _ := expr.command()
			if pe, ok := cmd["ProjectionExpression"]; ok {
				def["ProjectionExpression"] = pe
			}
			if en, ok := cmd["ExpressionAttributeNames"]; ok {
				def["ExpressionAttributeNames"] = en
			}
		}
	}
	if def != nil {
		def["ConsistentRead"] = params.Consistent
	}

	var result any
	if params.Parse {
		result = []Item{}
	} else {
		result = map[string]any{"Responses": map[string]any{}}
	}

	retries := 0
	for {
		data, err := t.execute(ctx, genericModelName, "batchGet", batch, Item{}, params)
		if err != nil {
			return nil, err
		}
		if data != nil {
			if responses, ok := data["Responses"].(map[string]any); ok {
				for key, items := range responses {
					for _, rawItem := range toAnySlice(items) {
						itemMap, _ := rawItem.(map[string]any)
						if params.Parse {
							item := t.unmarshallItem(itemMap)
							typeName, _ := item[t.typeField].(string)
							if typeName == "" {
								typeName = "_unknown"
							}
							if m := t.schemaMgr.models[typeName]; m != nil && m != t.schemaMgr.uniqueModel {
								result = append(result.([]Item), m.transformReadItem("get", item, Item{}, params, nil))
							}
						} else {
							resp := result.(map[string]any)["Responses"].(map[string]any)
							list, _ := resp[key].([]any)
							resp[key] = append(list, rawItem)
						}
					}
				}
			}
			if unprocessed, ok := data["UnprocessedItems"].(map[string]any); ok && len(unprocessed) > 0 {
				batch["RequestItems"] = unprocessed
				if params.Batch != nil {
					return nil, nil
				}
				if retries > 11 {
					return nil, fmt.Errorf("too many unprocessed items after retries")
				}
				time.Sleep(time.Duration(10*(1<<retries)) * time.Millisecond)
				retries++
				continue
			}
		}
		break
	}
	return result, nil
}

func (t *Table) BatchWrite(ctx context.Context, batch map[string]any, params *Params) (bool, error) {
	if len(batch) == 0 {
		return true, nil
	}
	if params == nil {
		params = &Params{}
	}
	retries := 0
	for {
		data, err := t.execute(ctx, genericModelName, "batchWrite", batch, Item{}, params)
		if err != nil {
			return false, err
		}
		if data != nil {
			if unprocessed, ok := data["UnprocessedItems"].(map[string]any); ok && len(unprocessed) > 0 {
				batch["RequestItems"] = unprocessed
				if retries > 11 {
					return false, fmt.Errorf("too many unprocessed items after retries")
				}
				time.Sleep(time.Duration(10*(1<<retries)) * time.Millisecond)
				retries++
				continue
			}
		}
		break
	}
	return true, nil
}

// ─── Transact ─────────────────────────────────────────────────────────────────

func (t *Table) Transact(ctx context.Context, op string, transaction map[string]any, params *Params) (any, error) {
	if params == nil {
		params = &Params{}
	}
	if params.Execute != nil && !*params.Execute {
		return transaction, nil
	}

	var dynOp string
	if op == "write" {
		dynOp = "transactWrite"
	} else {
		dynOp = "transactGet"
	}

	result, err := t.execute(ctx, genericModelName, dynOp, transaction, Item{}, params)
	if err != nil {
		return nil, err
	}

	if op == "get" && params.Parse {
		if responses, ok := result["Responses"].([]any); ok {
			items := []Item{}
			for _, r := range responses {
				if rm, ok := r.(map[string]any); ok {
					if rawItem, ok := rm["Item"].(map[string]any); ok {
						item := t.unmarshallItem(rawItem)
						typeName, _ := item[t.typeField].(string)
						if typeName == "" {
							typeName = "_unknown"
						}
						if m := t.schemaMgr.models[typeName]; m != nil && m != t.schemaMgr.uniqueModel {
							items = append(items, m.transformReadItem("get", item, Item{}, params, nil))
						}
					}
				}
			}
			return items, nil
		}
	}
	return result, nil
}

// ─── GroupByType ──────────────────────────────────────────────────────────────

func (t *Table) GroupByType(items []Item, params *Params) map[string][]Item {
	if params == nil {
		params = &Params{}
	}
	result := map[string][]Item{}
	for _, item := range items {
		typeName, _ := item[t.typeField].(string)
		if typeName == "" {
			typeName = "_unknown"
		}
		m := t.schemaMgr.models[typeName]
		var prepared Item
		if params.Hidden != nil && !*params.Hidden && m != nil {
			prepared = Item{}
			for k, v := range item {
				if f, ok := m.block.Fields[k]; !ok || !f.Hidden {
					prepared[k] = v
				}
			}
		} else {
			prepared = item
		}
		result[typeName] = append(result[typeName], prepared)
	}
	return result
}

// ─── DDL ──────────────────────────────────────────────────────────────────────

const confirmRemoveTable = "DeleteTableForever"

// CreateTable creates the DynamoDB table from the schema index definitions.
func (t *Table) CreateTable(ctx context.Context) error {
	def := t.GetTableDefinition(nil)

	input := &ddb.CreateTableInput{
		TableName:            &t.Name,
		AttributeDefinitions: def.AttributeDefinitions,
		KeySchema:            def.KeySchema,
		BillingMode:          def.BillingMode,
	}
	if len(def.GlobalSecondaryIndexes) > 0 {
		input.GlobalSecondaryIndexes = def.GlobalSecondaryIndexes
	}
	if len(def.LocalSecondaryIndexes) > 0 {
		input.LocalSecondaryIndexes = def.LocalSecondaryIndexes
	}

	_, err := t.client.CreateTable(ctx, input)
	return err
}

// DeleteTable permanently deletes the DynamoDB table.
func (t *Table) DeleteTable(ctx context.Context, confirmation string) error {
	if confirmation != confirmRemoveTable {
		return NewArgError(fmt.Sprintf(`Missing required confirmation "%s"`, confirmRemoveTable))
	}
	_, err := t.client.DeleteTable(ctx, &ddb.DeleteTableInput{TableName: &t.Name})
	return err
}

// DescribeTable returns the raw table description from AWS.
func (t *Table) DescribeTable(ctx context.Context) (Item, error) {
	out, err := t.client.DescribeTable(ctx, &ddb.DescribeTableInput{TableName: &t.Name})
	if err != nil {
		return nil, err
	}
	// Convert to Item via JSON for simplicity
	b, _ := json.Marshal(out)
	var result Item
	json.Unmarshal(b, &result) //nolint:errcheck
	return result, nil
}

// Exists returns true if the DynamoDB table is present.
func (t *Table) Exists(ctx context.Context) (bool, error) {
	tables, err := t.ListTables(ctx)
	if err != nil {
		return false, err
	}
	for _, name := range tables {
		if name == t.Name {
			return true, nil
		}
	}
	return false, nil
}

// ListTables returns all table names in the region.
func (t *Table) ListTables(ctx context.Context) ([]string, error) {
	out, err := t.client.ListTables(ctx, &ddb.ListTablesInput{})
	if err != nil {
		return nil, err
	}
	return out.TableNames, nil
}

// TableDefinition is the Go equivalent of the JS getTableDefinition result.
type TableDefinition struct {
	AttributeDefinitions   []types.AttributeDefinition
	KeySchema              []types.KeySchemaElement
	BillingMode            types.BillingMode
	GlobalSecondaryIndexes []types.GlobalSecondaryIndex
	LocalSecondaryIndexes  []types.LocalSecondaryIndex
	ProvisionedThroughput  *types.ProvisionedThroughput
}

func (t *Table) GetTableDefinition(provisioned *types.ProvisionedThroughput) *TableDefinition {
	def := &TableDefinition{}
	if provisioned != nil &&
		(provisioned.ReadCapacityUnits == nil || *provisioned.ReadCapacityUnits == 0) &&
		(provisioned.WriteCapacityUnits == nil || *provisioned.WriteCapacityUnits == 0) {
		def.BillingMode = types.BillingModePayPerRequest
	} else if provisioned != nil {
		def.BillingMode = types.BillingModeProvisioned
		def.ProvisionedThroughput = provisioned
	} else {
		def.BillingMode = types.BillingModePayPerRequest
	}

	attributes := map[string]bool{}
	indexes := t.schemaMgr.indexes
	if indexes == nil {
		panic("cannot create table without schema indexes")
	}

	for name, idx := range indexes {
		var keys []types.KeySchemaElement
		if name == "primary" {
			def.KeySchema = keys[:0]
			keys = def.KeySchema
		} else {
			projType := types.ProjectionTypeAll
			var nonKeyAttrs []string
			switch p := idx.Project.(type) {
			case []string:
				projType = types.ProjectionTypeInclude
				nonKeyAttrs = p
			case string:
				if p == "keys" {
					projType = types.ProjectionTypeKeysOnly
				}
			}

			proj := types.Projection{ProjectionType: projType}
			if len(nonKeyAttrs) > 0 {
				proj.NonKeyAttributes = nonKeyAttrs
			}
			gsi := types.GlobalSecondaryIndex{
				IndexName:  strPtr(name),
				Projection: &proj,
			}
			if provisioned != nil {
				gsi.ProvisionedThroughput = provisioned
			}
			def.GlobalSecondaryIndexes = append(def.GlobalSecondaryIndexes, gsi)
			// keys slice points into the GSI
			keys = gsi.KeySchema
		}

		if idx.Hash != "" {
			keys = append(keys, types.KeySchemaElement{
				AttributeName: strPtr(idx.Hash),
				KeyType:       types.KeyTypeHash,
			})
			if !attributes[idx.Hash] {
				at := types.ScalarAttributeTypeS
				if t.getAttributeType(idx.Hash) == "number" {
					at = types.ScalarAttributeTypeN
				}
				def.AttributeDefinitions = append(def.AttributeDefinitions,
					types.AttributeDefinition{AttributeName: strPtr(idx.Hash), AttributeType: at})
				attributes[idx.Hash] = true
			}
		}
		if idx.Sort != "" {
			keys = append(keys, types.KeySchemaElement{
				AttributeName: strPtr(idx.Sort),
				KeyType:       types.KeyTypeRange,
			})
			if !attributes[idx.Sort] {
				at := types.ScalarAttributeTypeS
				if t.getAttributeType(idx.Sort) == "number" {
					at = types.ScalarAttributeTypeN
				}
				def.AttributeDefinitions = append(def.AttributeDefinitions,
					types.AttributeDefinition{AttributeName: strPtr(idx.Sort), AttributeType: at})
				attributes[idx.Sort] = true
			}
		}
	}
	return def
}

func (t *Table) getAttributeType(name string) string {
	for _, m := range t.schemaMgr.models {
		if f, ok := m.block.Fields[name]; ok {
			return string(f.Type)
		}
	}
	return "string"
}

// ─── execute ──────────────────────────────────────────────────────────────────

// execute dispatches a DynamoDB operation and returns a normalised result Item.
func (t *Table) execute(ctx context.Context, modelName, op string, cmd Item, properties Item, params *Params) (Item, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()

	client := t.client
	if params != nil && params.Client != nil {
		client = params.Client
	}
	if client == nil {
		return nil, NewArgError("Table has no DynamoDB client configured")
	}

	logInfo(t.log, fmt.Sprintf(`OneTable "%s" "%s"`, op, modelName), map[string]any{"cmd": cmd, "op": op})

	var result Item
	var execErr error

	switch op {
	case "get":
		input, err := buildGetInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.GetItem(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		if out.Item != nil {
			item, err := unmarshallFromDynamo(out.Item)
			if err != nil {
				return nil, err
			}
			result = Item{"Item": item}
		} else {
			result = Item{}
		}

	case "put":
		input, err := buildPutInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.PutItem(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		if out.Attributes != nil {
			item, err := unmarshallFromDynamo(out.Attributes)
			if err != nil {
				return nil, err
			}
			result = Item{"Attributes": item}
		} else {
			result = Item{}
		}

	case "delete":
		input, err := buildDeleteInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.DeleteItem(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		if out.Attributes != nil {
			item, err := unmarshallFromDynamo(out.Attributes)
			if err != nil {
				return nil, err
			}
			result = Item{"Attributes": item}
		} else {
			result = Item{}
		}

	case "update":
		input, err := buildUpdateInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.UpdateItem(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		if out.Attributes != nil {
			item, err := unmarshallFromDynamo(out.Attributes)
			if err != nil {
				return nil, err
			}
			result = Item{"Attributes": item}
		} else {
			result = Item{}
		}

	case "find":
		input, err := buildQueryInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.Query(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		items, err := unmarshalListOfMaps(out.Items)
		if err != nil {
			return nil, err
		}
		result = Item{
			"Items": items,
			"Count": int(out.Count),
		}
		if out.LastEvaluatedKey != nil {
			lek, err := unmarshallFromDynamo(out.LastEvaluatedKey)
			if err == nil {
				result["LastEvaluatedKey"] = lek
			}
		}

	case "scan":
		input, err := buildScanInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.Scan(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		items, err := unmarshalListOfMaps(out.Items)
		if err != nil {
			return nil, err
		}
		result = Item{
			"Items":        items,
			"Count":        int(out.Count),
			"ScannedCount": int(out.ScannedCount),
		}
		if out.LastEvaluatedKey != nil {
			lek, err := unmarshallFromDynamo(out.LastEvaluatedKey)
			if err == nil {
				result["LastEvaluatedKey"] = lek
			}
		}

	case "batchGet":
		input, err := buildBatchGetInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.BatchGetItem(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		respMap := map[string]any{}
		for tbl, avItems := range out.Responses {
			items, err := unmarshalListOfMaps(avItems)
			if err != nil {
				return nil, err
			}
			respMap[tbl] = items
		}
		result = Item{"Responses": respMap}
		if len(out.UnprocessedKeys) > 0 {
			result["UnprocessedItems"] = out.UnprocessedKeys
		}

	case "batchWrite":
		input, err := buildBatchWriteInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.BatchWriteItem(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		result = Item{}
		if len(out.UnprocessedItems) > 0 {
			result["UnprocessedItems"] = out.UnprocessedItems
		}

	case "transactGet":
		input, err := buildTransactGetInput(cmd)
		if err != nil {
			return nil, err
		}
		out, err := client.TransactGetItems(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		responses := make([]any, len(out.Responses))
		for i, r := range out.Responses {
			if r.Item != nil {
				item, err := unmarshallFromDynamo(r.Item)
				if err == nil {
					responses[i] = map[string]any{"Item": item}
				}
			}
		}
		result = Item{"Responses": responses}

	case "transactWrite":
		input, err := buildTransactWriteInput(cmd)
		if err != nil {
			return nil, err
		}
		_, err = client.TransactWriteItems(ctx, input)
		if err != nil {
			execErr = err
			break
		}
		result = Item{}

	default:
		return nil, NewArgError("Unknown operation: " + op)
	}

	if execErr != nil {
		errMsg := execErr.Error()
		if strings.Contains(errMsg, "ConditionalCheckFailedException") && op == "put" {
			return nil, NewError(fmt.Sprintf(`Conditional create failed for "%s"`, modelName),
				WithCode(ErrRuntime), WithCause(execErr))
		}
		if strings.Contains(errMsg, "ProvisionedThroughputExceededException") {
			return nil, NewError("Provisioning Throughput Exception", WithCode(ErrRuntime), WithCause(execErr))
		}
		if strings.Contains(errMsg, "TransactionCanceledException") {
			return nil, NewError("Transaction Cancelled", WithCode(ErrRuntime), WithCause(execErr))
		}
		return nil, NewError(fmt.Sprintf(`OneTable execute failed "%s" for "%s": %s`, op, modelName, errMsg),
			WithCode(ErrRuntime), WithCause(execErr))
	}

	// metrics / monitoring
	if t.metrics != nil {
		t.metrics.Add(modelName, op, result, params, start) //nolint:errcheck
	}
	if t.monitor != nil {
		t.monitor(modelName, op, result, params, start) //nolint:errcheck
	}

	return result, nil
}

// ─── crypto ───────────────────────────────────────────────────────────────────

func (t *Table) initCrypto(cfg map[string]*CryptoConfig) error {
	t.cryptoConfigs = map[string]*cryptoEntry{}
	for name, c := range cfg {
		h := sha256.Sum256([]byte(c.Password))
		t.cryptoConfigs[name] = &cryptoEntry{
			name:   name,
			cipher: c.Cipher,
			key:    h[:],
		}
	}
	return nil
}

func (t *Table) encrypt(text string) (string, error) {
	if text == "" {
		return text, nil
	}
	if t.cryptoConfigs == nil {
		return "", NewArgError("No crypto config defined")
	}
	entry := t.cryptoConfigs["primary"]
	if entry == nil {
		return "", NewArgError("No primary crypto config")
	}
	block, err := aes.NewCipher(entry.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return fmt.Sprintf("primary::%x:%s", nonce, encoded), nil
}

func (t *Table) decrypt(text string) (string, error) {
	if text == "" {
		return text, nil
	}
	parts := strings.SplitN(text, ":", 4)
	if len(parts) < 4 {
		return text, nil
	}
	name := parts[0]
	if t.cryptoConfigs == nil {
		return "", NewArgError("No crypto config defined")
	}
	entry := t.cryptoConfigs[name]
	if entry == nil {
		return "", NewArgError(fmt.Sprintf("No crypto config for %q", name))
	}
	data, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(entry.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// ─── marshall / unmarshall helpers ────────────────────────────────────────────

// unmarshallItem converts a raw DynamoDB attribute value map into a plain Go Item.
// If the input is already a plain map (i.e. not marshalled), it is returned as-is.
func (t *Table) unmarshallItem(raw map[string]any) Item {
	if raw == nil {
		return nil
	}
	// attempt to detect DynamoDB typed format {"S": "value"} by checking first value
	for _, v := range raw {
		if _, ok := v.(map[string]any); ok {
			// likely marshalled – attempt unmarshal
			avMap := make(map[string]types.AttributeValue, len(raw))
			for k, val := range raw {
				avMap[k] = anyToAV(val)
			}
			item, err := unmarshallFromDynamo(avMap)
			if err == nil {
				return item
			}
		}
		break
	}
	return raw
}

// anyToAV is a best-effort conversion of a raw any to an AttributeValue.
// Only called for the DynamoDB-typed-format case.
func anyToAV(v any) types.AttributeValue {
	switch tv := v.(type) {
	case map[string]any:
		if s, ok := tv["S"].(string); ok {
			return &types.AttributeValueMemberS{Value: s}
		}
		if n, ok := tv["N"].(string); ok {
			return &types.AttributeValueMemberN{Value: n}
		}
		if b, ok := tv["BOOL"].(bool); ok {
			return &types.AttributeValueMemberBOOL{Value: b}
		}
		if _, ok := tv["NULL"]; ok {
			return &types.AttributeValueMemberNULL{Value: true}
		}
		// fallback: marshal inner map
		inner := make(map[string]types.AttributeValue, len(tv))
		for k, val := range tv {
			inner[k] = anyToAV(val)
		}
		return &types.AttributeValueMemberM{Value: inner}
	case string:
		return &types.AttributeValueMemberS{Value: tv}
	case float64:
		return &types.AttributeValueMemberN{Value: fmt.Sprintf("%v", tv)}
	case bool:
		return &types.AttributeValueMemberBOOL{Value: tv}
	case nil:
		return &types.AttributeValueMemberNULL{Value: true}
	default:
		// marshal via attributevalue for complex types
		av, _ := attributevalue.Marshal(tv)
		if av != nil {
			return av
		}
		return &types.AttributeValueMemberNULL{Value: true}
	}
}

// ─── UID helpers ──────────────────────────────────────────────────────────────

func (t *Table) generate(gen string) any {
	switch gen {
	case "uuid":
		return t.UUID()
	case "ulid":
		return t.ULID()
	case "uid":
		return t.UID(10)
	default:
		if strings.HasPrefix(gen, "uid(") {
			n := 10
			fmt.Sscanf(gen, "uid(%d)", &n)
			return t.UID(n)
		}
		return t.UUID()
	}
}

func (t *Table) UUID() string {
	return uid.UUID()
}

func (t *Table) ULID() string {
	return uid.New().String()
}

func (t *Table) UID(size int) string {
	return uid.UID(size)
}

// ─── DynamoDB command builders ────────────────────────────────────────────────

// These helpers convert the generic Item command map to typed AWS SDK inputs.

func buildGetInput(cmd Item) (*ddb.GetItemInput, error) {
	input := &ddb.GetItemInput{}
	if tn, ok := cmd["TableName"].(string); ok {
		input.TableName = &tn
	}
	if key, ok := cmd["Key"].(map[string]types.AttributeValue); ok {
		input.Key = key
	} else if keyItem, ok := cmd["Key"].(Item); ok {
		k, err := marshallForDynamo(keyItem)
		if err != nil {
			return nil, err
		}
		input.Key = k
	}
	if cr, ok := cmd["ConsistentRead"].(bool); ok {
		input.ConsistentRead = &cr
	}
	if pe, ok := cmd["ProjectionExpression"].(string); ok {
		input.ProjectionExpression = &pe
	}
	if en, ok := cmd["ExpressionAttributeNames"].(map[string]string); ok {
		input.ExpressionAttributeNames = en
	}
	return input, nil
}

func buildPutInput(cmd Item) (*ddb.PutItemInput, error) {
	input := &ddb.PutItemInput{}
	if tn, ok := cmd["TableName"].(string); ok {
		input.TableName = &tn
	}
	if item, ok := cmd["Item"].(map[string]types.AttributeValue); ok {
		input.Item = item
	} else if itemMap, ok := cmd["Item"].(Item); ok {
		marshalled, err := marshallForDynamo(itemMap)
		if err != nil {
			return nil, err
		}
		input.Item = marshalled
	}
	if ce, ok := cmd["ConditionExpression"].(string); ok {
		input.ConditionExpression = &ce
	}
	if en, ok := cmd["ExpressionAttributeNames"].(map[string]string); ok {
		input.ExpressionAttributeNames = en
	}
	if ev, ok := cmd["ExpressionAttributeValues"].(map[string]types.AttributeValue); ok {
		input.ExpressionAttributeValues = ev
	}
	if rv, ok := cmd["ReturnValues"].(string); ok {
		input.ReturnValues = types.ReturnValue(rv)
	}
	return input, nil
}

func buildDeleteInput(cmd Item) (*ddb.DeleteItemInput, error) {
	input := &ddb.DeleteItemInput{}
	if tn, ok := cmd["TableName"].(string); ok {
		input.TableName = &tn
	}
	if key, ok := cmd["Key"].(map[string]types.AttributeValue); ok {
		input.Key = key
	} else if keyItem, ok := cmd["Key"].(Item); ok {
		k, err := marshallForDynamo(keyItem)
		if err != nil {
			return nil, err
		}
		input.Key = k
	}
	if ce, ok := cmd["ConditionExpression"].(string); ok {
		input.ConditionExpression = &ce
	}
	if en, ok := cmd["ExpressionAttributeNames"].(map[string]string); ok {
		input.ExpressionAttributeNames = en
	}
	if ev, ok := cmd["ExpressionAttributeValues"].(map[string]types.AttributeValue); ok {
		input.ExpressionAttributeValues = ev
	}
	if rv, ok := cmd["ReturnValues"].(string); ok {
		input.ReturnValues = types.ReturnValue(rv)
	}
	return input, nil
}

func buildUpdateInput(cmd Item) (*ddb.UpdateItemInput, error) {
	input := &ddb.UpdateItemInput{}
	if tn, ok := cmd["TableName"].(string); ok {
		input.TableName = &tn
	}
	if key, ok := cmd["Key"].(map[string]types.AttributeValue); ok {
		input.Key = key
	} else if keyItem, ok := cmd["Key"].(Item); ok {
		k, err := marshallForDynamo(keyItem)
		if err != nil {
			return nil, err
		}
		input.Key = k
	}
	if ue, ok := cmd["UpdateExpression"].(string); ok {
		input.UpdateExpression = &ue
	}
	if ce, ok := cmd["ConditionExpression"].(string); ok {
		input.ConditionExpression = &ce
	}
	if en, ok := cmd["ExpressionAttributeNames"].(map[string]string); ok {
		input.ExpressionAttributeNames = en
	}
	if ev, ok := cmd["ExpressionAttributeValues"].(map[string]types.AttributeValue); ok {
		input.ExpressionAttributeValues = ev
	}
	if rv, ok := cmd["ReturnValues"].(string); ok {
		input.ReturnValues = types.ReturnValue(rv)
	}
	return input, nil
}

func buildQueryInput(cmd Item) (*ddb.QueryInput, error) {
	input := &ddb.QueryInput{}
	if tn, ok := cmd["TableName"].(string); ok {
		input.TableName = &tn
	}
	if kce, ok := cmd["KeyConditionExpression"].(string); ok {
		input.KeyConditionExpression = &kce
	}
	if fe, ok := cmd["FilterExpression"].(string); ok {
		input.FilterExpression = &fe
	}
	if pe, ok := cmd["ProjectionExpression"].(string); ok {
		input.ProjectionExpression = &pe
	}
	if en, ok := cmd["ExpressionAttributeNames"].(map[string]string); ok {
		input.ExpressionAttributeNames = en
	}
	if ev, ok := cmd["ExpressionAttributeValues"].(map[string]types.AttributeValue); ok {
		input.ExpressionAttributeValues = ev
	}
	if in, ok := cmd["IndexName"].(string); ok && in != "" {
		input.IndexName = &in
	}
	if limit, ok := toIntAny(cmd["Limit"]); ok {
		l := int32(limit)
		input.Limit = &l
	}
	if cr, ok := cmd["ConsistentRead"].(bool); ok {
		input.ConsistentRead = &cr
	}
	if sif, ok := cmd["ScanIndexForward"].(bool); ok {
		input.ScanIndexForward = &sif
	}
	if esk, ok := cmd["ExclusiveStartKey"].(map[string]types.AttributeValue); ok {
		input.ExclusiveStartKey = esk
	}
	if sel, ok := cmd["Select"].(string); ok {
		input.Select = types.Select(sel)
	}
	return input, nil
}

func buildScanInput(cmd Item) (*ddb.ScanInput, error) {
	input := &ddb.ScanInput{}
	if tn, ok := cmd["TableName"].(string); ok {
		input.TableName = &tn
	}
	if fe, ok := cmd["FilterExpression"].(string); ok {
		input.FilterExpression = &fe
	}
	if pe, ok := cmd["ProjectionExpression"].(string); ok {
		input.ProjectionExpression = &pe
	}
	if en, ok := cmd["ExpressionAttributeNames"].(map[string]string); ok {
		input.ExpressionAttributeNames = en
	}
	if ev, ok := cmd["ExpressionAttributeValues"].(map[string]types.AttributeValue); ok {
		input.ExpressionAttributeValues = ev
	}
	if in, ok := cmd["IndexName"].(string); ok && in != "" {
		input.IndexName = &in
	}
	if limit, ok := toIntAny(cmd["Limit"]); ok {
		l := int32(limit)
		input.Limit = &l
	}
	if cr, ok := cmd["ConsistentRead"].(bool); ok {
		input.ConsistentRead = &cr
	}
	if esk, ok := cmd["ExclusiveStartKey"].(map[string]types.AttributeValue); ok {
		input.ExclusiveStartKey = esk
	}
	if seg, ok := toIntAny(cmd["Segment"]); ok {
		s := int32(seg)
		input.Segment = &s
	}
	if ts, ok := toIntAny(cmd["TotalSegments"]); ok {
		s := int32(ts)
		input.TotalSegments = &s
	}
	if sel, ok := cmd["Select"].(string); ok {
		input.Select = types.Select(sel)
	}
	return input, nil
}

func unmarshalListOfMaps(list []map[string]types.AttributeValue) ([]Item, error) {
	items := make([]Item, 0, len(list))
	for _, av := range list {
		item, err := unmarshallFromDynamo(av)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func toIntAny(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

func strPtr(s string) *string { return &s }

// toAnySlice converts []any, []Item (= []map[string]any), etc. to []any.
func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []map[string]any: // also catches []Item since Item = map[string]any
		out := make([]any, len(s))
		for i, m := range s {
			out[i] = m
		}
		return out
	}
	return nil
}

// ─── batch / transact input builders ────────────────────────────────────────
// These convert the generic Item command maps (which hold types.AttributeValue
// values directly) into properly-typed AWS SDK inputs without JSON round-trips.

func extractAVMap(m map[string]any, key string) map[string]types.AttributeValue {
	switch v := m[key].(type) {
	case map[string]types.AttributeValue:
		return v
	}
	return nil
}

func extractAVMapItem(m map[string]any, key string) map[string]types.AttributeValue {
	return extractAVMap(m, key)
}

func extractString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func extractStringPtr(m map[string]any, key string) *string {
	s, ok := m[key].(string)
	if !ok || s == "" {
		return nil
	}
	return &s
}

func extractStringMap(m map[string]any, key string) map[string]string {
	v, _ := m[key].(map[string]string)
	return v
}

func extractAVMapValues(m map[string]any, key string) map[string]types.AttributeValue {
	v, _ := m[key].(map[string]types.AttributeValue)
	return v
}

// cmdToMap converts an Item to map[string]any (they are the same type but help readability).
func cmdToMap(cmd Item) map[string]any { return map[string]any(cmd) }

// buildTransactWriteInput builds a TransactWriteItemsInput from the generic transaction map.
// The transaction map has the shape: {"TransactItems": [{"Put": cmd}, {"Update": cmd}, ...]}
func buildTransactWriteInput(cmd Item) (*ddb.TransactWriteItemsInput, error) {
	input := &ddb.TransactWriteItemsInput{}
	rawItems, _ := cmd["TransactItems"].([]any)
	for _, raw := range rawItems {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		var ti types.TransactWriteItem
		if putRaw, ok := entry["Put"]; ok {
			p, _ := putRaw.(Item)
			if p == nil {
				p, _ = putRaw.(map[string]any)
			}
			putIn, err := buildPutInput(p)
			if err != nil {
				return nil, err
			}
			ti.Put = &types.Put{
				TableName:                 putIn.TableName,
				Item:                      putIn.Item,
				ConditionExpression:       putIn.ConditionExpression,
				ExpressionAttributeNames:  putIn.ExpressionAttributeNames,
				ExpressionAttributeValues: putIn.ExpressionAttributeValues,
			}
		} else if updateRaw, ok := entry["Update"]; ok {
			u, _ := updateRaw.(Item)
			if u == nil {
				u, _ = updateRaw.(map[string]any)
			}
			updIn, err := buildUpdateInput(u)
			if err != nil {
				return nil, err
			}
			ti.Update = &types.Update{
				TableName:                 updIn.TableName,
				Key:                       updIn.Key,
				UpdateExpression:          updIn.UpdateExpression,
				ConditionExpression:       updIn.ConditionExpression,
				ExpressionAttributeNames:  updIn.ExpressionAttributeNames,
				ExpressionAttributeValues: updIn.ExpressionAttributeValues,
			}
		} else if deleteRaw, ok := entry["Delete"]; ok {
			d, _ := deleteRaw.(Item)
			if d == nil {
				d, _ = deleteRaw.(map[string]any)
			}
			delIn, err := buildDeleteInput(d)
			if err != nil {
				return nil, err
			}
			ti.Delete = &types.Delete{
				TableName:                 delIn.TableName,
				Key:                       delIn.Key,
				ConditionExpression:       delIn.ConditionExpression,
				ExpressionAttributeNames:  delIn.ExpressionAttributeNames,
				ExpressionAttributeValues: delIn.ExpressionAttributeValues,
			}
		} else if checkRaw, ok := entry["ConditionCheck"]; ok {
			c, _ := checkRaw.(Item)
			if c == nil {
				c, _ = checkRaw.(map[string]any)
			}
			var key map[string]types.AttributeValue
			if k, ok := c["Key"].(map[string]types.AttributeValue); ok {
				key = k
			}
			ti.ConditionCheck = &types.ConditionCheck{
				TableName:                 extractStringPtr(c, "TableName"),
				Key:                       key,
				ConditionExpression:       extractStringPtr(c, "ConditionExpression"),
				ExpressionAttributeNames:  extractStringMap(c, "ExpressionAttributeNames"),
				ExpressionAttributeValues: extractAVMapValues(c, "ExpressionAttributeValues"),
			}
		}
		input.TransactItems = append(input.TransactItems, ti)
	}
	return input, nil
}

// buildTransactGetInput builds a TransactGetItemsInput from the generic transaction map.
func buildTransactGetInput(cmd Item) (*ddb.TransactGetItemsInput, error) {
	input := &ddb.TransactGetItemsInput{}
	rawItems, _ := cmd["TransactItems"].([]any)
	for _, raw := range rawItems {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if getRaw, ok := entry["Get"]; ok {
			g, _ := getRaw.(Item)
			if g == nil {
				g, _ = getRaw.(map[string]any)
			}
			getIn, err := buildGetInput(g)
			if err != nil {
				return nil, err
			}
			ti := types.TransactGetItem{
				Get: &types.Get{
					TableName:                getIn.TableName,
					Key:                      getIn.Key,
					ProjectionExpression:     getIn.ProjectionExpression,
					ExpressionAttributeNames: getIn.ExpressionAttributeNames,
				},
			}
			input.TransactItems = append(input.TransactItems, ti)
		}
	}
	return input, nil
}

// buildBatchGetInput builds a BatchGetItemInput from the generic batch map.
// The batch map has shape: {"RequestItems": {"tableName": {"Keys": [...], "ConsistentRead": bool}}}
func buildBatchGetInput(cmd Item) (*ddb.BatchGetItemInput, error) {
	input := &ddb.BatchGetItemInput{RequestItems: map[string]types.KeysAndAttributes{}}
	ritems, _ := cmd["RequestItems"].(map[string]any)
	for tbl, rawEntry := range ritems {
		entry, _ := rawEntry.(map[string]any)
		if entry == nil {
			continue
		}
		ka := types.KeysAndAttributes{}
		if keys, ok := entry["Keys"].([]any); ok {
			for _, k := range keys {
				switch kv := k.(type) {
				case map[string]types.AttributeValue:
					ka.Keys = append(ka.Keys, kv)
				}
			}
		}
		if cr, ok := entry["ConsistentRead"].(bool); ok {
			ka.ConsistentRead = &cr
		}
		if pe, ok := entry["ProjectionExpression"].(string); ok && pe != "" {
			ka.ProjectionExpression = &pe
		}
		if en, ok := entry["ExpressionAttributeNames"].(map[string]string); ok {
			ka.ExpressionAttributeNames = en
		}
		input.RequestItems[tbl] = ka
	}
	return input, nil
}

// buildBatchWriteInput builds a BatchWriteItemInput from the generic batch map.
// The batch map has shape:
//
//	{"RequestItems": {"tableName": [{"PutRequest": cmd}, {"DeleteRequest": cmd}, ...]}}
func buildBatchWriteInput(cmd Item) (*ddb.BatchWriteItemInput, error) {
	input := &ddb.BatchWriteItemInput{RequestItems: map[string][]types.WriteRequest{}}
	ritems, _ := cmd["RequestItems"].(map[string]any)
	for tbl, rawList := range ritems {
		list, _ := rawList.([]any)
		var reqs []types.WriteRequest
		for _, rawReq := range list {
			reqEntry, _ := rawReq.(map[string]any)
			if reqEntry == nil {
				continue
			}
			var wr types.WriteRequest
			if putRaw, ok := reqEntry["PutRequest"]; ok {
				p, _ := putRaw.(Item)
				if p == nil {
					p, _ = putRaw.(map[string]any)
				}
				putIn, err := buildPutInput(p)
				if err != nil {
					return nil, err
				}
				wr.PutRequest = &types.PutRequest{Item: putIn.Item}
			} else if delRaw, ok := reqEntry["DeleteRequest"]; ok {
				d, _ := delRaw.(Item)
				if d == nil {
					d, _ = delRaw.(map[string]any)
				}
				delIn, err := buildDeleteInput(d)
				if err != nil {
					return nil, err
				}
				wr.DeleteRequest = &types.DeleteRequest{Key: delIn.Key}
			}
			reqs = append(reqs, wr)
		}
		input.RequestItems[tbl] = reqs
	}
	return input, nil
}
