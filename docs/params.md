# Params

Most `Model` and `Table` APIs accept a `*Params` argument. Pass `nil` to accept all defaults.

```go
type Params struct { ... }
```

---

## Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Add` | `map[string]any` | — | Atomically add a numeric value to an attribute. Keys are field names, values are numbers to add. |
| `Batch` | `map[string]any` | — | Batch accumulator. Pass the same map to multiple API calls, then execute with `Table.BatchGet` / `Table.BatchWrite`. |
| `Capacity` | `string` | — | Return consumed capacity. Values: `"INDEXES"`, `"TOTAL"`, `"NONE"`. |
| `Client` | `DynamoClient` | — | Override the table-level DynamoDB client for this call only. |
| `Consistent` | `bool` | `false` | Request strongly-consistent reads. |
| `Context` | `context.Context` | — | Go `context.Context` forwarded to the AWS SDK call. Not related to the table-level property context (`TableParams.Context`). |
| `Count` | `bool` | `false` | Return only the count of matching items (not the items themselves). The count is in `Result.Count`. |
| `Delete` | `map[string]any` | — | Delete elements from a `set` attribute. Keys are field names, values are slices of items to remove from the set. |
| `Execute` | `*bool` | `true` | Set `false` to build the DynamoDB command without executing it. The command `Item` is returned instead of the result. |
| `Exists` | `*bool` | varies | `true` → item must exist (error otherwise). `false` → item must not exist (error otherwise). `nil` → no check. Default: `false` for `Create`, `true` for `Update`, `nil` for `Upsert`, `nil` for `Remove`. |
| `Fields` | `[]string` | — | Limit returned attributes. Sets `ProjectionExpression`. Names are Go field names (schema names), not DynamoDB attribute names. |
| `Follow` | `*bool` | index default | Re-fetch each item from the primary index after a query. Useful for `KEYS_ONLY` GSIs. |
| `Hidden` | `*bool` | table default | `true` → include hidden fields in the returned `Item`. `false` → exclude them explicitly. |
| `Index` | `string` | `"primary"` | Name of the index to use. |
| `Limit` | `int` | 0 (unlimited) | Maximum number of items for DynamoDB to read. Note: this is the DynamoDB scan limit, not the number of returned items after filtering. |
| `Log` | `*bool` | `false` | Force logging of this API call at `info` level. |
| `Many` | `bool` | `false` | Allow `Remove` to delete more than one matching item. |
| `MaxPages` | `int` | 1000 | Maximum number of DynamoDB query/scan pages before stopping. Prevents infinite loops on large tables. |
| `Next` | `Item` | — | Exclusive start key for forward pagination. Typically set to the `Result.Next` value from a previous call. |
| `Partial` | `*bool` | table default | Allow partial nested-object updates for this call. |
| `PostFormat` | `func(*Model, map[string]any) map[string]any` | — | Hook called with the final DynamoDB command just before execution. Return the (optionally modified) command. |
| `Prev` | `Item` | — | Exclusive start key for reverse pagination. Typically set to `Result.Prev`. Mutually exclusive with `Next`. |
| `Push` | `map[string]any` | — | Append items to a list attribute using `list_append(if_not_exists(...))`. Keys are field names, values are items to append (scalar or slice). |
| `Remove` | `[]string` | — | List of field names to remove from the item on update. |
| `Return` | `any` | varies | Controls the DynamoDB `ReturnValues` parameter. Values: `true` (alias for `"ALL_NEW"` on update/delete, `"ALL_OLD"` on delete), `false` / `"NONE"`, `"ALL_NEW"`, `"ALL_OLD"`, `"UPDATED_NEW"`, `"UPDATED_OLD"`, `"get"` (transparent `Get` after update; required for unique-field updates). `Create` always returns the created item via expression properties (DynamoDB `ReturnValues` is `NONE` internally). `Update` defaults to `"ALL_NEW"`. `Delete` defaults to `"ALL_OLD"`. |
| `Reverse` | `bool` | `false` | Reverse the sort order of query results (`ScanIndexForward = false`). |
| `Select` | `string` | — | DynamoDB `Select` parameter. `"COUNT"` returns only a count; `"ALL_ATTRIBUTES"` is the default for queries. |
| `Set` | `map[string]string` | — | Expression-based attribute updates. Keys are field names; values are DynamoDB update expressions with `${field}` and `{value}` placeholders (same syntax as Where clauses). |
| `Stats` | `*Stats` | — | Pointer to a `Stats` struct that accumulates operation metrics across paginated calls. |
| `Substitutions` | `map[string]any` | — | Named variables for use in `Where` and `Set` expressions via `@{varName}`. |
| `Transaction` | `map[string]any` | — | Transaction accumulator. Pass to multiple API calls; execute with `Table.Transact`. |
| `Where` | `string` | — | Filter or condition expression template. See [where.md](where.md). |

---

## Exists defaults by operation

| Operation | Default `Exists` | Behaviour |
|-----------|-----------------|-----------|
| `Create` | `false` | Fails if an item with the same key already exists. |
| `Get` | `nil` | Returns `nil, nil` (no error) when not found. |
| `Update` | `true` | Fails if the item does not exist (DynamoDB conditional check → `ErrRuntime`; for unique-field items: `ErrNotFound`). |
| `Upsert` | `nil` | Creates if missing, updates if found. |
| `Remove` | `nil` | Silently succeeds even if item not found. Set to `true` to error on missing (unique-field path only). |

---

## Stats

Passed via `Params.Stats`. Populated during `Find` and `Scan` operations:

```go
type Stats struct {
    Count    int     // items returned
    Scanned  int     // items scanned (before filtering)
    Capacity float64 // consumed capacity units
}
```

```go
stats := &onetable.Stats{}
result, err := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{
    Stats: stats,
})
fmt.Printf("scanned %d, returned %d, capacity %.1f\n", stats.Scanned, stats.Count, stats.Capacity)
```

---

## Set expression syntax

`Params.Set` values follow the same template syntax as Where clauses:

```go
Set: map[string]string{
    // increment a counter
    "loginCount": "${loginCount} + {1}",
    // set a field to a literal value
    "status": "{active}",
    // conditional set (DynamoDB if_not_exists)
    "created": "if_not_exists(${created}, {0})",
    // list_append with substitution
    "tags": "list_append(if_not_exists(${tags}, @{empty}), @{newTags})",
},
Substitutions: map[string]any{
    "empty":   []any{},
    "newTags": []any{"go", "dynamodb"},
},
```

`${field}` → `ExpressionAttributeNames` alias  
`{value}` → `ExpressionAttributeValues` entry (typed automatically)  
`@{var}` → `ExpressionAttributeValues` entry from `Substitutions`

---

## Transform

`TableParams.Transform` is a function called for every field on every read and write:

```go
type TransformFunc func(model *Model, op, name string, value any, properties Item) any
```

`op` is `"read"` or `"write"`. Return the (optionally modified) value.

---

## Parallel scan note

`Params.Segment = 0` is the zero value for `int` and is treated as "not set" — it is omitted from the DynamoDB request. To scan segment 0 in a parallel scan, omit `Segment` entirely (or call without the `Segment` field) and provide only `Segments`:

```go
// Segment 0 — no Segment param
result, _ = table.ScanItems(ctx, nil, &onetable.Params{Segments: 4})

// Segments 1, 2, 3 — explicit Segment param
for seg := 1; seg < 4; seg++ {
    result, _ = table.ScanItems(ctx, nil, &onetable.Params{Segment: seg, Segments: 4})
}
```

## Pagination example

```go
var next onetable.Item
for {
    result, err := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{
        Limit: 25,
        Next:  next,
        Where: "${status} = {active}",
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
