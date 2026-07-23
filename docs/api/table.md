# Table API

`Table` is the top-level object representing a single DynamoDB table. It owns the schema, DynamoDB client, encryption config, table-level context, and all configuration.

Obtained via the constructor:

```go
table, err := onetable.NewTable(onetable.TableParams{
    Name:   "MyTable",
    Client: dynamoClient,
    Schema: schema,
})
```

See [Constructor](../table.md#constructor) for the full `TableParams` reference.

---

## Convenience model methods

These methods accept a model name string and delegate to the corresponding `Model` method. Useful for factory / dynamic patterns where the model is determined at runtime.

### Create

```go
func (t *Table) Create(ctx context.Context, modelName string, properties Item, params *Params) (Item, error)
```

Create an item of the named model type. See [Model.Create](model.md#create).

### Get

```go
func (t *Table) Get(ctx context.Context, modelName string, properties Item, params *Params) (Item, error)
```

Get a single item. See [Model.Get](model.md#get).

### Find

```go
func (t *Table) Find(ctx context.Context, modelName string, properties Item, params *Params) (*Result, error)
```

Query items. See [Model.Find](model.md#find).

### Update

```go
func (t *Table) Update(ctx context.Context, modelName string, properties Item, params *Params) (Item, error)
```

Update an item. See [Model.Update](model.md#update).

### Upsert

```go
func (t *Table) Upsert(ctx context.Context, modelName string, properties Item, params *Params) (Item, error)
```

Update-or-create. See [Model.Upsert](model.md#upsert).

### Remove

```go
func (t *Table) Remove(ctx context.Context, modelName string, properties Item, params *Params) (Item, error)
```

Delete an item. See [Model.Remove](model.md#remove).

### Scan

```go
func (t *Table) Scan(ctx context.Context, modelName string, properties Item, params *Params) (*Result, error)
```

Full-table scan filtered to the model type. See [Model.Scan](model.md#scan).

---

## Low-level item API

These methods bypass the schema's model system and operate directly on DynamoDB items. They mirror the underlying DynamoDB SDK operations but use `Item` maps for input/output.

### GetItem

```go
func (t *Table) GetItem(ctx context.Context, properties Item, params *Params) (Item, error)
```

Wraps DynamoDB `GetItem`. Supply the primary key attributes in `properties`.

### PutItem

```go
func (t *Table) PutItem(ctx context.Context, properties Item, params *Params) (Item, error)
```

Wraps DynamoDB `PutItem`. All provided attributes are written verbatim — no schema mapping, no validation.

### DeleteItem

```go
func (t *Table) DeleteItem(ctx context.Context, properties Item, params *Params) (Item, error)
```

Wraps DynamoDB `DeleteItem`. Supply the primary key in `properties`.

### UpdateItem

```go
func (t *Table) UpdateItem(ctx context.Context, properties Item, params *Params) (Item, error)
```

Wraps DynamoDB `UpdateItem`. Supply primary key + attributes to change in `properties`.

### QueryItems

```go
func (t *Table) QueryItems(ctx context.Context, properties Item, params *Params) (*Result, error)
```

Wraps DynamoDB `Query`. Provide the hash key in `properties`; optionally provide a sort key condition.

**Sort key conditions** — set the sort key to a map with an operator key:

```go
// begins_with
result, err := table.QueryItems(ctx, onetable.Item{
    "pk": "user#account1",
    "sk": map[string]any{"begins": "user#"},
}, nil)

// between (inclusive)
result, err = table.QueryItems(ctx, onetable.Item{
    "pk": "order#acct1",
    "sk": map[string]any{"between": []any{1000, 2000}},
}, nil)

// comparison operators: "<", "<=", "=", ">=", ">"
result, err = table.QueryItems(ctx, onetable.Item{
    "pk": "invoice#acct1",
    "sk": map[string]any{"<=": 1000},
}, nil)
```

Non-key properties build a `FilterExpression` applied server-side after reading. More complex filters can be expressed with `Params.Where`:

```go
result, err := table.QueryItems(ctx, onetable.Item{"pk": "order#acct1"}, &onetable.Params{
    Where: `${sk} <= {1000}`,
})
```

**Relevant params:** `Index`, `Limit`, `Next`, `Prev`, `Reverse`, `MaxPages`, `Fields`, `Where`, `Consistent`, `Parse`, `Execute`.

### ScanItems

```go
func (t *Table) ScanItems(ctx context.Context, properties Item, params *Params) (*Result, error)
```

Wraps DynamoDB `Scan`. Returns all items in the table matching the optional filter.

**Parallel scan** — use `Params.Segment` and `Params.Segments`:

```go
const segments = 4
results := make([]*onetable.Result, segments)
var wg sync.WaitGroup

// Segment 0: no Segment param (0 is zero-value, treated as unset)
wg.Add(1)
go func() {
    defer wg.Done()
    results[0], _ = table.ScanItems(ctx, nil, &onetable.Params{Segments: segments})
}()

// Segments 1..3: explicit Segment param
for seg := 1; seg < segments; seg++ {
    wg.Add(1)
    go func(s int) {
        defer wg.Done()
        results[s], _ = table.ScanItems(ctx, nil, &onetable.Params{
            Segment:  s,
            Segments: segments,
        })
    }(seg)
}
wg.Wait()
```

> **Note:** `Params.Segment = 0` is the zero value for `int` and is treated as "not set". To scan segment 0, omit `Segment` entirely.

**Relevant params:** `Where`, `Limit`, `Next`, `Prev`, `Reverse`, `MaxPages`, `Fields`, `Consistent`, `Parse`, `Execute`, `Segment`, `Segments`.

---

## Batch operations

Batches accumulate multiple API calls into a single DynamoDB batch request. Build a batch by passing a shared `map[string]any` via `Params.Batch`, then execute.

### BatchGet

```go
func (t *Table) BatchGet(ctx context.Context, batch map[string]any, params *Params) (any, error)
```

Execute a prepared batch-get operation. When `params.Parse` is `true`, returns `[]Item` parsed through model schemas. Otherwise returns the raw DynamoDB response.

Automatically retries unprocessed items with exponential back-off.

```go
batch := map[string]any{}
Account.Get(ctx, onetable.Item{"id": accountId}, &onetable.Params{Batch: batch})
User.Get(ctx, onetable.Item{"id": userId},       &onetable.Params{Batch: batch})
items, err := table.BatchGet(ctx, batch, &onetable.Params{Parse: true})
```

### BatchWrite

```go
func (t *Table) BatchWrite(ctx context.Context, batch map[string]any, params *Params) (bool, error)
```

Execute a prepared batch-write operation. Returns `true` on success. Automatically retries unprocessed items.

```go
batch := map[string]any{}
User.Create(ctx, onetable.Item{"name": "Bob"}, &onetable.Params{Batch: batch})
User.Remove(ctx, onetable.Item{"id": oldId},   &onetable.Params{Batch: batch})
ok, err := table.BatchWrite(ctx, batch, nil)
```

See [Batch Operations](batch.md) for detailed usage and limitations.

---

## Transactions

Transactions atomically read or write multiple items. Build a transaction by passing a shared `map[string]any` via `Params.Transaction`, then execute.

### Transact

```go
func (t *Table) Transact(ctx context.Context, op string, transaction map[string]any, params *Params) (any, error)
```

Execute a prepared transaction. `op` must be `"get"` or `"write"`.

**Write transaction:**

```go
tx := map[string]any{}
Account.Create(ctx, onetable.Item{"id": "a1", "name": "Acme"}, &onetable.Params{Transaction: tx})
User.Create(ctx, onetable.Item{"id": "u1", "name": "Alice"},   &onetable.Params{Transaction: tx})
_, err := table.Transact(ctx, "write", tx, nil)
```

**Get transaction** — returns `[]Item` when `params.Parse` is `true`:

```go
tx = map[string]any{}
table.Get(ctx, "Account", onetable.Item{"id": "a1"}, &onetable.Params{Transaction: tx})
table.Get(ctx, "User",    onetable.Item{"id": "u1"}, &onetable.Params{Transaction: tx})
items, err := table.Transact(ctx, "get", tx, &onetable.Params{Parse: true})
grouped := table.GroupByType(items.([]onetable.Item), nil)
accounts := grouped["Account"]
users    := grouped["User"]
```

See [Transaction Operations](transact.md) for detailed usage and limitations.

---

## GroupByType

```go
func (t *Table) GroupByType(items []Item, params *Params) map[string][]Item
```

Group a flat slice of items by their model type (using the `TypeField` attribute, default `_type`). Returns a map keyed by model name.

```go
items, err := table.Transact(ctx, "get", tx, &onetable.Params{Parse: true})
grouped := table.GroupByType(items.([]onetable.Item), nil)
for model, modelItems := range grouped {
    fmt.Printf("%s: %d items\n", model, len(modelItems))
}
```

---

## Fetch

```go
func (t *Table) Fetch(ctx context.Context, models []string, properties Item, params *Params) (map[string][]Item, error)
```

Fetch an item-collection of multiple model types that share the same primary partition key. Issues a single `Query` with a `FilterExpression` matching any of the requested model types, then groups the result by type.

```go
items, err := table.Fetch(ctx,
    []string{"User", "Product"},
    onetable.Item{"pk": "account#acme"},
    nil,
)
users    := items["User"]
products := items["Product"]
```

`properties` must supply the hash key (and optionally sort key prefix) shared by all the requested models.

---

## Schema methods

### SetSchema

```go
func (t *Table) SetSchema(ctx context.Context, schema *SchemaDef) (map[string]*IndexDef, error)
```

Replace the active schema. Returns the resolved index map.

When `schema` is `nil` the current in-memory schema is cleared and index definitions are re-discovered from DynamoDB via `DescribeTable`.

### GetCurrentSchema

```go
func (t *Table) GetCurrentSchema() *SchemaDef
```

Return the schema currently in use.

### GetKeys

```go
func (t *Table) GetKeys(ctx context.Context) (map[string]*IndexDef, error)
```

Return a map of index definitions keyed by index name (`"primary"`, `"gs1"`, …). Each `IndexDef` has `Hash` and `Sort` attribute names.

### SaveSchema

```go
func (t *Table) SaveSchema(ctx context.Context, schema *SchemaDef) error
```

Persist a schema to the DynamoDB table using the `_Schema` model. If `schema` is `nil`, the current in-memory schema is saved with the name `"Current"`.

### ReadSchema

```go
func (t *Table) ReadSchema(ctx context.Context) (*SchemaDef, error)
```

Read the `"Current"` schema item previously stored by `SaveSchema`. Returns `nil, nil` if no schema has been saved.

### ReadSchemas

```go
func (t *Table) ReadSchemas(ctx context.Context) ([]*SchemaDef, error)
```

Return all schema items stored in the table (all saved versions).

### RemoveSchema

```go
func (t *Table) RemoveSchema(ctx context.Context, schema *SchemaDef) error
```

Delete a previously saved schema item. `schema.Name` must match the name used when saving.

---

## Model registry

### GetModel

```go
func (t *Table) GetModel(name string) (*Model, error)
```

Return the `Model` for the given schema model name. Returns an error if the model is not defined in the schema.

### AddModel

```go
func (t *Table) AddModel(name string, fields FieldMap)
```

Dynamically add a model to the table. Uses the table's existing index definitions.

### RemoveModel

```go
func (t *Table) RemoveModel(name string) error
```

Remove a model from the in-memory schema. Does not affect the DynamoDB table.

### ListModels

```go
func (t *Table) ListModels() []string
```

Return the names of all models currently registered in the table schema.

---

## Context

Table-level context properties are merged into every write operation (similar to a global default). They can be overridden per-call via `Params.Context`.

### GetContext

```go
func (t *Table) GetContext() Item
```

Return the current table-level context map.

### SetContext

```go
func (t *Table) SetContext(ctx Item, merge bool) *Table
```

Set the table context. If `merge` is `true`, the supplied properties are blended with the existing context. If `false`, the context is replaced entirely.

### AddContext

```go
func (t *Table) AddContext(ctx Item) *Table
```

Merge additional properties into the existing context (equivalent to `SetContext(ctx, true)`).

### ClearContext

```go
func (t *Table) ClearContext() *Table
```

Remove all table-level context properties.

---

## Client and logging

### SetClient

```go
func (t *Table) SetClient(client DynamoClient)
```

Replace the DynamoDB client after construction. Useful for swapping in a test double or rotating credentials.

### GetLog

```go
func (t *Table) GetLog() Logger
```

Return the `Logger` currently in use by the table.

### SetLog

```go
func (t *Table) SetLog(logger Logger)
```

Replace the `Logger` after construction.

---

## DDL operations

### CreateTable

```go
func (t *Table) CreateTable(ctx context.Context) error
```

Create the DynamoDB table based on the schema index definitions. Uses `PAY_PER_REQUEST` billing by default.

### DeleteTable

```go
func (t *Table) DeleteTable(ctx context.Context, confirmation string) error
```

Permanently delete the DynamoDB table. `confirmation` must be the exact string `"DeleteTableForever"`.

### DescribeTable

```go
func (t *Table) DescribeTable(ctx context.Context) (Item, error)
```

Return the raw `DescribeTable` response as an `Item` map.

### Exists

```go
func (t *Table) Exists(ctx context.Context) (bool, error)
```

Return `true` if the table exists in the current region.

### ListTables

```go
func (t *Table) ListTables(ctx context.Context) ([]string, error)
```

Return all table names in the current region.

### UpdateTable

```go
func (t *Table) UpdateTable(ctx context.Context, params *UpdateTableParams) error
```

Create, remove, or update a Global Secondary Index on an existing table.

```go
type UpdateTableParams struct {
    Create      *UpdateTableIndex
    Remove      *UpdateTableIndex
    Update      *UpdateTableIndex
    Provisioned *types.ProvisionedThroughput
}

type UpdateTableIndex struct {
    Name        string
    Hash        string
    Sort        string
    Project     any    // "all" | "keys" | []string
    Provisioned *types.ProvisionedThroughput
}
```

```go
// Create a GSI
err = table.UpdateTable(ctx, &onetable.UpdateTableParams{
    Create: &onetable.UpdateTableIndex{
        Name: "gs1",
        Hash: "gs1pk",
        Sort: "gs1sk",
    },
})

// Remove a GSI
err = table.UpdateTable(ctx, &onetable.UpdateTableParams{
    Remove: &onetable.UpdateTableIndex{Name: "gs1"},
})
```

> LSIs cannot be created or deleted after table creation — `UpdateTable` rejects LSI create attempts.

### GetTableDefinition

```go
func (t *Table) GetTableDefinition(provisioned *types.ProvisionedThroughput) *TableDefinition
```

Build a `TableDefinition` from the schema (attribute definitions, key schema, GSI/LSI list). Useful when you need to create the table with custom provisioned capacity or inspect the derived schema.

---

## UID helpers

### UUID

```go
func (t *Table) UUID() string
```

Generate a UUID v4-format string (fast, non-cryptographic).

### ULID

```go
func (t *Table) ULID() string
```

Generate a [ULID](https://github.com/ulid/spec) — time-sortable, cryptographically random, URL-safe. Preferred over UUID for primary keys.

### UID

```go
func (t *Table) UID(size int) string
```

Generate a random URL-safe string of `size` characters.

---

## DynamoClient interface

Any struct satisfying this interface can be used as the DynamoDB client:

```go
type DynamoClient interface {
    GetItem(ctx, *GetItemInput, ...func(*Options)) (*GetItemOutput, error)
    PutItem(ctx, *PutItemInput, ...func(*Options)) (*PutItemOutput, error)
    DeleteItem(ctx, *DeleteItemInput, ...func(*Options)) (*DeleteItemOutput, error)
    UpdateItem(ctx, *UpdateItemInput, ...func(*Options)) (*UpdateItemOutput, error)
    Query(ctx, *QueryInput, ...func(*Options)) (*QueryOutput, error)
    Scan(ctx, *ScanInput, ...func(*Options)) (*ScanOutput, error)
    BatchGetItem(ctx, *BatchGetItemInput, ...func(*Options)) (*BatchGetItemOutput, error)
    BatchWriteItem(ctx, *BatchWriteItemInput, ...func(*Options)) (*BatchWriteItemOutput, error)
    TransactGetItems(ctx, *TransactGetItemsInput, ...func(*Options)) (*TransactGetItemsOutput, error)
    TransactWriteItems(ctx, *TransactWriteItemsInput, ...func(*Options)) (*TransactWriteItemsOutput, error)
    CreateTable(ctx, *CreateTableInput, ...func(*Options)) (*CreateTableOutput, error)
    DeleteTable(ctx, *DeleteTableInput, ...func(*Options)) (*DeleteTableOutput, error)
    UpdateTable(ctx, *UpdateTableInput, ...func(*Options)) (*UpdateTableOutput, error)
    DescribeTable(ctx, *DescribeTableInput, ...func(*Options)) (*DescribeTableOutput, error)
    ListTables(ctx, *ListTablesInput, ...func(*Options)) (*ListTablesOutput, error)
    UpdateTimeToLive(ctx, *UpdateTimeToLiveInput, ...func(*Options)) (*UpdateTimeToLiveOutput, error)
}
```

The real `*dynamodb.Client` from `github.com/aws/aws-sdk-go-v2/service/dynamodb` satisfies this interface.
