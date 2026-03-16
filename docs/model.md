# Model API

`Model` represents a single entity type stored in the DynamoDB table. Instances are obtained via `Table.GetModel`:

```go
User, err := table.GetModel("User")
```

All methods accept a `context.Context` as first argument and an optional `*Params` as last argument. Passing `nil` for `params` uses sensible defaults. See [params.md](params.md) for the full parameter reference.

API errors return `*OneTableError`. See [errors.md](errors.md).

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
- All `required` schema fields must be present; field validation and enum checks are applied.
- Fields with `generate: "ulid"` (or `"uuid"`, `"uid"`) are auto-populated if not supplied.
- Fields with `value` templates are auto-computed.
- If any field has `unique: true`, the create is wrapped in a DynamoDB transaction that also reserves the unique value. If the unique constraint fails, `ErrUnique` is returned.
- Encrypted fields (`crypt: true`) are transparently encrypted before writing.

```go
user, err := User.Create(ctx, onetable.Item{
    "name":  "Alice",
    "email": "alice@example.com",
    "role":  "admin",
}, nil)
```

Returns the created item with all schema defaults and generated values applied. Hidden fields are not returned unless `Params.Hidden` is set.

---

## Get

```go
func (m *Model) Get(ctx context.Context, properties Item, params *Params) (Item, error)
```

Fetch a single item by its primary key. Wraps DynamoDB `GetItem`.

Supply the key attributes (or the fields from which the key can be derived via value templates):

```go
user, err := User.Get(ctx, onetable.Item{"id": "01ABCDEF"}, nil)
```

**Set / list attributes** are returned as `[]any` in the raw `Item`. Convert to a typed slice:

```go
recipientsAny := user["recipients"].([]any)
recipients := make([]string, 0, len(recipientsAny))
for _, v := range recipientsAny {
    if s, ok := v.(string); ok {
        recipients = append(recipients, s)
    }
}
```

If additional non-key properties are supplied, a `Find` is executed first to locate the item (fallback path). If the `Find` returns more than one result, `ErrNonUnique` is returned.

Returns `nil, nil` when the item does not exist (no error).

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

When called without a sort key on a model that has a sort-key value template, OneTable synthesises a `begins_with` condition from the leading static portion of the template.

Additional non-key properties in `properties` are used as a `FilterExpression`. More complex filters can be expressed with `Params.Where`.

**Pagination** — `Result.Next` is non-nil when more pages exist:

```go
var next onetable.Item
for {
    result, err := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{
        Limit: 20,
        Next:  next,
    })
    // process result.Items …
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

**Count only** — set `Params.Count = true`; the result count is in `Result.Count`.

**Relevant params:** `Index`, `Limit`, `Next`, `Prev`, `Reverse`, `MaxPages`, `Fields`, `Where`, `Consistent`, `Follow`, `Hidden`, `Count`, `Stats`, `Segments`, `Segment`.

---

## Update

```go
func (m *Model) Update(ctx context.Context, properties Item, params *Params) (Item, error)
```

Update an existing item. Wraps DynamoDB `UpdateItem`.

- Default `Params.Exists = true` — fails with `ErrNotFound` if the item does not exist.
- Only supply the key plus the fields you want to change; unmentioned fields are left untouched.
- If any field being updated has `unique: true`, a transaction is used to atomically swap the unique sentinel.

```go
updated, err := User.Update(ctx, onetable.Item{
    "id":   "01ABCDEF",
    "name": "Alice Smith",
}, nil)
```

**Atomic operations via Params:**

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

// Expression-based set (supports DynamoDB update functions)
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

`Params.Return` controls what is returned:
- `"ALL_NEW"` (default) — the full item after update.
- `"NONE"` — no return value.
- `"get"` — perform a transparent `Get` after the update and return the result (required when the item has unique fields, since DynamoDB transactions do not return updated values).

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

Supply the primary key:

```go
_, err := User.Remove(ctx, onetable.Item{"id": "01ABCDEF"}, nil)
```

If additional non-key properties are supplied, a `Find` is performed first to locate the item. Set `Params.Many = true` to allow removing multiple matching items; by default, removing more than one item returns `ErrNonUnique`.

If the item has `unique` fields, a transaction is used to remove the unique sentinels alongside the item.

Returns the deleted item (`ALL_OLD` from DynamoDB). Returns `nil` when the item did not exist.

To assert the item existed before removal, set `Params.Exists = true` — `ErrNotFound` is returned when unique-field items are missing; for plain items, DynamoDB's conditional check will fail with `ErrRuntime`.

---

## Scan

```go
func (m *Model) Scan(ctx context.Context, properties Item, params *Params) (*Result, error)
```

Full-table scan filtered to items of this model's type. Wraps DynamoDB `Scan`.

Properties are used as a filter expression. Unlike `Find`, scan reads the entire table; for large datasets consider a GSI on the type field instead.

```go
result, err := User.Scan(ctx, onetable.Item{"role": "admin"}, nil)
```

Parallel scan:

```go
var wg sync.WaitGroup
results := make([]*onetable.Result, 4)
for seg := 0; seg < 4; seg++ {
    wg.Add(1)
    go func(s int) {
        defer wg.Done()
        results[s], _ = User.Scan(ctx, nil, &onetable.Params{Segment: s, Segments: 4})
    }(seg)
}
wg.Wait()
```

---

## Init

```go
func (m *Model) Init(ctx context.Context, properties Item, params *Params) (Item, error)
```

Construct a local item map populated with all schema defaults, generated values and computed value templates — without writing anything to DynamoDB. All undefined fields are set to `nil`. Useful for building partial objects or default form values.

```go
template, err := User.Init(ctx, onetable.Item{}, nil)
// template has id (generated), pk, sk, and nil for all other fields
```

---

## Low-level item methods

The following methods bypass high-level schema processing (no type-filter injection, no auto-timestamps, no hidden-field stripping). They mirror the underlying DynamoDB operations directly.

| Method | DynamoDB op |
|--------|-------------|
| `putItem(ctx, properties, params)` | `PutItem` |
| `getItem(ctx, properties, params)` | `GetItem` |
| `deleteItem(ctx, properties, params)` | `DeleteItem` |
| `updateItem(ctx, properties, params)` | `UpdateItem` |
| `queryItems(ctx, properties, params)` | `Query` |
| `scanItems(ctx, properties, params)` | `Scan` |

> These are unexported. Use `Table.GetItem`, `Table.PutItem`, etc. for direct low-level access.

---

## Unique fields

When a schema field has `Unique: true`, OneTable enforces uniqueness by writing a sentinel item with primary key `_unique#<Scope>#<Model>#<Attr>#<Value>` in the same transaction as the main item.

- **Create** — unique sentinels are created atomically; `ErrUnique` on conflict.
- **Update** — old sentinel deleted and new one created atomically; `ErrUnique` on conflict.
- **Remove** — sentinels are deleted alongside the item.
- `Params.Return = "get"` is needed on updates to get back the full updated item when unique fields are involved.
