# Model API

`Model` represents a single entity type stored in the DynamoDB table. Instances are obtained via `Table.GetModel`:

```go
User, err := table.GetModel("User")
```

All methods accept a `context.Context` as first argument and an optional `*Params` as last argument. Passing `nil` for `params` uses sensible defaults. See [Params](../params.md) for the full parameter reference.

API errors return `*OneTableError`. See [Errors](../errors.md).

---

## Result type

`Find` and `Scan` return a `*Result`:

```go
type Result struct {
    Items []Item // returned items
    Next  Item   // non-nil: more pages exist; pass as Params.Next
    Prev  Item   // non-nil: first-page key; pass as Params.Prev
    Count int    // set when Params.Count == true
}
```

---

## Create

```go
func (m *Model) Create(ctx context.Context, properties Item, params *Params) (Item, error)
```

Create a new item. Wraps DynamoDB `PutItem`.

- Default `Params.Exists = false` — fails if an item with the same key already exists.
- Set `Params.Exists = nil` to allow overwriting an existing item.
- All `Required` schema fields must be present; field validation and enum checks are applied.
- Fields with `Generate: "ulid"` (or `"uuid"`, `"uid"`) are auto-populated if not supplied.
- Fields with `Value` templates are auto-computed.
- If any field has `Unique: true`, the create is wrapped in a DynamoDB transaction that also reserves the unique value. If the unique constraint fails, `ErrUnique` is returned.
- Encrypted fields (`Crypt: true`) are transparently encrypted before writing.

```go
user, err := User.Create(ctx, onetable.Item{
    "name":  "Alice",
    "email": "alice@example.com",
    "role":  "admin",
}, nil)
```

Returns the created item with all schema defaults and generated values applied. Hidden fields are not returned unless `Params.Hidden` is set.

### Batch / Transaction accumulation

Pass `Params.Batch` or `Params.Transaction` to accumulate this operation into a batch or transaction instead of executing immediately:

```go
batch := map[string]any{}
User.Create(ctx, onetable.Item{"name": "Bob"}, &onetable.Params{Batch: batch})
User.Create(ctx, onetable.Item{"name": "Carol"}, &onetable.Params{Batch: batch})
ok, err := table.BatchWrite(ctx, batch, nil)
```

---

## Get

```go
func (m *Model) Get(ctx context.Context, properties Item, params *Params) (Item, error)
```

Fetch a single item by its primary key. Wraps DynamoDB `GetItem`.

```go
user, err := User.Get(ctx, onetable.Item{"id": "01ABCDEF"}, nil)
```

If additional non-key properties are supplied, a `Find` is executed first to locate the item (fallback path). If the `Find` returns more than one result, `ErrNonUnique` is returned.

Returns `nil, nil` when the item does not exist (no error).

**Set / list attributes** are returned as `[]any`. Convert to a typed slice:

```go
recipientsAny := user["recipients"].([]any)
recipients := make([]string, 0, len(recipientsAny))
for _, v := range recipientsAny {
    if s, ok := v.(string); ok {
        recipients = append(recipients, s)
    }
}
```

**Relevant params:** `Fields`, `Consistent`, `Follow`, `Hidden`, `Index`.

---

## Find

```go
func (m *Model) Find(ctx context.Context, properties Item, params *Params) (*Result, error)
```

Query items. Wraps DynamoDB `Query`.

Supply the hash key and optionally a sort key or sort key condition:

```go
// All users for an account
result, err := User.Find(ctx, onetable.Item{"accountId": "acct1"}, nil)

// Sort key begins_with
result, err = User.Find(ctx, onetable.Item{
    "accountId": "acct1",
    "id": map[string]any{"begins": "user#"},
}, nil)

// Sort key operators: "<", "<=", "=", ">=", ">", "between"
result, err = Invoice.Find(ctx, onetable.Item{
    "accountId": "acct1",
    "amount":    map[string]any{"between": []any{100, 500}},
}, nil)
```

**Sort key operators:** `<`, `<=`, `=`, `>=`, `>`, `begins` (or `begins_with`), `between`

Additional non-key properties in `properties` are used as a `FilterExpression`. More complex filters can be expressed with `Params.Where`:

```go
result, err := User.Find(ctx, onetable.Item{}, &onetable.Params{
    Where: `(${role} = {admin}) and (${status} = {active})`,
})
```

When called without a sort key on a model that has a sort-key value template, OneTable synthesises a `begins_with` condition from the leading static portion of the template. For example, if the sort key is `sk: {Value: "card#${id}"}`, calling `Find` with just the hash key uses `begins_with sk, "card#"`.

### Pagination

`Result.Next` is non-nil when more pages exist:

```go
var next onetable.Item
for {
    result, err := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{
        Limit: 20,
        Next:  next,
    })
    if err != nil {
        return err
    }
    for _, user := range result.Items {
        process(user)
    }
    next = result.Next
    if next == nil {
        break
    }
}
```

Reverse pagination via `Result.Prev` / `Params.Prev`:

```go
page1, _ := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{Limit: 20})
page2, _ := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{Limit: 20, Next: page1.Next})
prevPage, _ := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{Limit: 20, Prev: page2.Prev})
```

**Count only** — set `Params.Count = true`; the result count is in `Result.Count`:

```go
result, err := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{Count: true})
fmt.Printf("total users: %d\n", result.Count)
```

**Relevant params:** `Index`, `Limit`, `Next`, `Prev`, `Reverse`, `MaxPages`, `Fields`, `Where`, `Consistent`, `Follow`, `Hidden`, `Count`, `Stats`.

---

## Update

```go
func (m *Model) Update(ctx context.Context, properties Item, params *Params) (Item, error)
```

Update an existing item. Wraps DynamoDB `UpdateItem`.

- Default `Params.Exists = true` — fails with `ErrNotFound` if the item does not exist.
- Only supply the key plus the fields you want to change; unmentioned fields are left untouched.
- If any field being updated has `Unique: true`, a transaction is used to atomically swap the unique sentinel.

```go
updated, err := User.Update(ctx, onetable.Item{
    "id":   "01ABCDEF",
    "name": "Alice Smith",
}, nil)
```

### Atomic operations via Params

```go
// Atomically add to a numeric field
User.Update(ctx, onetable.Item{"id": id}, &onetable.Params{
    Add: map[string]any{"points": 10},
})

// Remove attributes
User.Update(ctx, onetable.Item{"id": id}, &onetable.Params{
    Remove: []string{"suspended", "tempFlag"},
})

// Delete items from a set attribute
User.Update(ctx, onetable.Item{"id": id}, &onetable.Params{
    Delete: map[string]any{"roles": []any{"guest"}},
})

// Expression-based set
User.Update(ctx, onetable.Item{"id": id}, &onetable.Params{
    Set: map[string]string{
        "balance": "${balance} + {100}",
    },
})

// list_append using substitutions
User.Update(ctx, onetable.Item{"id": id}, &onetable.Params{
    Set: map[string]string{
        "contacts": "list_append(if_not_exists(contacts, @{emptyList}), @{newContacts})",
    },
    Substitutions: map[string]any{
        "emptyList":   []any{},
        "newContacts": []any{"+15551234567"},
    },
})
```

### Return control

`Params.Return` controls what is returned:

| Value | Behaviour |
|-------|-----------|
| `"ALL_NEW"` (default) | Full item after update |
| `"NONE"` | No return value |
| `"get"` | Transparent `Get` after update (required for unique-field updates) |
| `"ALL_OLD"` | Item before update |
| `"UPDATED_OLD"` | Only updated attributes before update |
| `"UPDATED_NEW"` | Only updated attributes after update |

```go
// Unique-field update — must use "get" to retrieve the updated item
updated, err := User.Update(ctx, onetable.Item{
    "id":    id,
    "email": "newemail@example.com",
}, &onetable.Params{
    Return: "get",
})
```

### Upsert via Exists

Set `Params.Exists = nil` to create the item if it doesn't exist (upsert):

```go
// Creates if missing, updates if found
item, err := User.Update(ctx, onetable.Item{
    "id":    id,
    "name":  "Bob",
    "email": "bob@example.com",
}, &onetable.Params{
    Exists: nil,
})
```

---

## Upsert

```go
func (m *Model) Upsert(ctx context.Context, properties Item, params *Params) (Item, error)
```

Update-or-create. Identical to `Update` except `Params.Exists` defaults to `nil` (no existence check). If the item does not exist it is created; if it exists it is updated. Supply all fields required for a create when using upsert.

```go
item, err := User.Upsert(ctx, onetable.Item{
    "id":    "01ABCDEF",
    "name":  "Bob",
    "email": "bob@example.com",
}, nil)
```

---

## Remove

```go
func (m *Model) Remove(ctx context.Context, properties Item, params *Params) (Item, error)
```

Delete an item. Wraps DynamoDB `DeleteItem`.

```go
_, err := User.Remove(ctx, onetable.Item{"id": "01ABCDEF"}, nil)
```

If additional non-key properties are supplied, a `Find` is performed first to locate the item. Set `Params.Many = true` to allow removing multiple matching items; by default, removing more than one item returns `ErrNonUnique`.

If the item has `Unique` fields, a transaction is used to remove the unique sentinels alongside the item.

Returns the deleted item (`ALL_OLD` from DynamoDB). Returns `nil` when the item did not exist.

To assert the item existed before removal, set `Params.Exists = true` — `ErrNotFound` is returned when the item is missing.

---

## Scan

```go
func (m *Model) Scan(ctx context.Context, properties Item, params *Params) (*Result, error)
```

Full-table scan filtered to items of this model's type. Wraps DynamoDB `Scan`.

Properties are used as a filter expression. Unlike `Find`, scan reads the entire table; for large datasets consider a GSI on the type field instead.

```go
result, err := User.Scan(ctx, onetable.Item{"role": "admin"}, nil)
for _, user := range result.Items {
    process(user)
}
```

### Parallel scan

```go
var wg sync.WaitGroup
results := make([]*onetable.Result, 4)
for seg := 0; seg < 4; seg++ {
    wg.Add(1)
    go func(s int) {
        defer wg.Done()
        results[s], _ = User.Scan(ctx, nil, &onetable.Params{
            Segment:  s,
            Segments: 4,
        })
    }(seg)
}
wg.Wait()
```

> **Note:** `Params.Segment = 0` is the zero value and is treated as "not set". See [Parallel scan note](../params.md#parallel-scan-note).

**Relevant params:** `Where`, `Limit`, `Next`, `Prev`, `Reverse`, `MaxPages`, `Fields`, `Consistent`, `Follow`, `Hidden`, `Count`, `Stats`, `Segment`, `Segments`.

---

## Init

```go
func (m *Model) Init(ctx context.Context, properties Item, params *Params) (Item, error)
```

Construct a local item map populated with all schema defaults, generated values, and computed value templates — without writing anything to DynamoDB. All undefined fields are set to `nil`.

```go
template, err := User.Init(ctx, onetable.Item{}, nil)
// template has id (generated), pk, sk, and nil for all other fields
```

Useful for building partial objects or default form values. Be careful using these with `Create` — you should define values for all required attributes.

---

## Unique fields

When a schema field has `Unique: true`, OneTable enforces uniqueness by writing a sentinel item with primary key `_unique#<Scope>#<Model>#<Attr>#<Value>` in the same transaction as the main item.

- **Create** — unique sentinels are created atomically; `ErrUnique` on conflict.
- **Update** — old sentinel deleted and new one created atomically; `ErrUnique` on conflict.
- **Remove** — sentinels are deleted alongside the item.
- `Params.Return = "get"` is needed on updates to get back the full updated item when unique fields are involved (DynamoDB transactions do not return updated values).

### Scoped uniqueness

Use `FieldDef.Scope` to limit uniqueness to a domain:

```go
"email": {
    Type:   "string",
    Unique: true,
    Scope:  "account#${accountId}",  // unique per account, not globally
}
```

---

## Batch / Transaction accumulation

Every Model method (`Create`, `Get`, `Find`, `Update`, `Upsert`, `Remove`, `Scan`) accepts `Params.Batch` and `Params.Transaction` to accumulate the operation instead of executing it immediately.

### Batch accumulation

```go
batch := map[string]any{}
User.Create(ctx, onetable.Item{"name": "Bob"},   &onetable.Params{Batch: batch})
User.Create(ctx, onetable.Item{"name": "Carol"}, &onetable.Params{Batch: batch})
ok, err := table.BatchWrite(ctx, batch, nil)
```

### Transaction accumulation

```go
tx := map[string]any{}
Account.Create(ctx, onetable.Item{"id": "a1"}, &onetable.Params{Transaction: tx})
User.Create(ctx, onetable.Item{"id": "u1", "accountId": "a1"}, &onetable.Params{Transaction: tx})
_, err := table.Transact(ctx, "write", tx, nil)
```

See [Batch Operations](batch.md) and [Transaction Operations](transact.md) for details.
