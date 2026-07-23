# Batch Operations

DynamoDB batch APIs allow you to read or write multiple items in a single request, which is more efficient than individual operations. OneTable provides `BatchGet` and `BatchWrite` on the `Table` type.

---

## How batches work

Batches are built incrementally by passing a shared `map[string]any` accumulator via `Params.Batch` to individual Model API calls. The calls are **not executed** — they are recorded into the batch map. You then execute the entire batch with a single `Table.BatchGet` or `Table.BatchWrite` call.

---

## BatchGet

```go
func (t *Table) BatchGet(ctx context.Context, batch map[string]any, params *Params) (any, error)
```

Execute a prepared batch-get operation. Reads up to 100 items in a single DynamoDB `BatchGetItem` request.

### Building a batch-get

```go
batch := map[string]any{}

// Accumulate Get operations — none of these hit DynamoDB yet
Account.Get(ctx, onetable.Item{"id": "acct1"}, &onetable.Params{Batch: batch})
User.Get(ctx, onetable.Item{"id": "user1"},   &onetable.Params{Batch: batch})
User.Get(ctx, onetable.Item{"id": "user2"},   &onetable.Params{Batch: batch})

// Execute the batch
result, err := table.BatchGet(ctx, batch, &onetable.Params{Parse: true})
if err != nil {
    log.Fatal(err)
}

// result is []Item when Parse=true
items := result.([]onetable.Item)
for _, item := range items {
    fmt.Println(item)
}
```

### Without parsing

When `Params.Parse` is `false` (the default), the raw DynamoDB response is returned:

```go
result, err := table.BatchGet(ctx, batch, nil)
// result is map[string]any{"Responses": map[string]any{...}}
raw := result.(map[string]any)
responses := raw["Responses"].(map[string]any)
```

### Consistent reads

Set `Params.Consistent` to `true` for strongly-consistent reads:

```go
Account.Get(ctx, onetable.Item{"id": "acct1"}, &onetable.Params{
    Batch:     batch,
    Consistent: true,
})
```

### Field projection

Use `Params.Fields` to limit returned attributes. Note: for batch operations, fields must use **DynamoDB attribute names**, not schema property names:

```go
User.Get(ctx, onetable.Item{"id": "user1"}, &onetable.Params{
    Batch:  batch,
    Fields: []string{"pk", "sk", "name", "email"},
})
```

### Limitations

- Maximum **100 items** per batch (DynamoDB limit).
- Maximum **16 MB** total request size (DynamoDB limit).
- All items in a `BatchGet` must be reads (no writes mixed in).
- Unprocessed items are automatically retried with exponential back-off (up to 12 rounds).

---

## BatchWrite

```go
func (t *Table) BatchWrite(ctx context.Context, batch map[string]any, params *Params) (bool, error)
```

Execute a prepared batch-write operation. Writes up to 25 items in a single DynamoDB `BatchWriteItem` request.

### Building a batch-write

```go
batch := map[string]any{}

// Accumulate Create (Put) and Remove (Delete) operations
User.Create(ctx, onetable.Item{"name": "Bob", "email": "bob@example.com"},   &onetable.Params{Batch: batch})
User.Create(ctx, onetable.Item{"name": "Carol", "email": "carol@example.com"}, &onetable.Params{Batch: batch})
User.Remove(ctx, onetable.Item{"id": "oldUserId"}, &onetable.Params{Batch: batch})

// Execute the batch
ok, err := table.BatchWrite(ctx, batch, nil)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("batch write success: %v\n", ok)
```

### Mixing puts and deletes

A single batch can contain both `Create` (put) and `Remove` (delete) operations:

```go
batch := map[string]any{}

// Puts
Product.Create(ctx, onetable.Item{"id": "p1", "name": "Widget"}, &onetable.Params{Batch: batch})
Product.Create(ctx, onetable.Item{"id": "p2", "name": "Gadget"}, &onetable.Params{Batch: batch})

// Deletes
Product.Remove(ctx, onetable.Item{"id": "p3"}, &onetable.Params{Batch: batch})

ok, err := table.BatchWrite(ctx, batch, nil)
```

### Limitations

- Maximum **25 items** per batch (DynamoDB limit).
- Maximum **16 MB** total request size (DynamoDB limit).
- Cannot mix reads and writes in the same batch.
- Unprocessed items are automatically retried with exponential back-off (up to 12 rounds).

---

## Complete example

```go
func batchExample(ctx context.Context, table *onetable.Table) error {
    User, err := table.GetModel("User")
    if err != nil {
        return err
    }

    // Create some users
    batch := map[string]any{}
    User.Create(ctx, onetable.Item{"name": "Alice", "email": "alice@example.com"}, &onetable.Params{Batch: batch})
    User.Create(ctx, onetable.Item{"name": "Bob",   "email": "bob@example.com"},   &onetable.Params{Batch: batch})
    User.Create(ctx, onetable.Item{"name": "Carol", "email": "carol@example.com"}, &onetable.Params{Batch: batch})

    ok, err := table.BatchWrite(ctx, batch, nil)
    if err != nil {
        return fmt.Errorf("batch write: %w", err)
    }
    if !ok {
        return fmt.Errorf("batch write failed")
    }

    // Now read them all back
    batch = map[string]any{}
    User.Get(ctx, onetable.Item{"name": "Alice"}, &onetable.Params{Batch: batch})
    User.Get(ctx, onetable.Item{"name": "Bob"},   &onetable.Params{Batch: batch})
    User.Get(ctx, onetable.Item{"name": "Carol"}, &onetable.Params{Batch: batch})

    result, err := table.BatchGet(ctx, batch, &onetable.Params{Parse: true})
    if err != nil {
        return fmt.Errorf("batch get: %w", err)
    }

    items := result.([]onetable.Item)
    for _, item := range items {
        fmt.Printf("user: %v\n", item["name"])
    }

    return nil
}
```

---

## Transaction alternative

If you need atomicity (all-or-nothing), use [Transactions](transact.md) instead of batches. Batches do **not** provide atomicity — individual items may succeed while others fail. See [transact.md](transact.md) for details.
