# Where Clauses

`Params.Where` is a convenient way to express DynamoDB filter and conditional expressions without manually managing `ExpressionAttributeNames` and `ExpressionAttributeValues`.

OneTable parses the where clause, extracts attribute references and values, and populates the DynamoDB expression maps automatically.

---

## Syntax

| Placeholder | Usage | Expands to |
|-------------|-------|------------|
| `${fieldName}` | Attribute reference | An `ExpressionAttributeNames` alias (handles reserved words) |
| `{value}` | Inline literal | An `ExpressionAttributeValues` entry, typed automatically |
| `@{varName}` | Variable substitution | An `ExpressionAttributeValues` entry from `Params.Substitutions` |

**Field names** are Go/schema property names (not DynamoDB attribute names). OneTable resolves any `map:` mappings automatically.

**Numeric literals** inside `{}` are typed as DynamoDB `N`. Wrap in quotes to force string: `{"42"}`.

---

## Examples

### Simple equality filter

```go
result, err := User.Find(ctx, onetable.Item{"accountId": id}, &onetable.Params{
    Where: `${status} = {active}`,
})
```

### Multiple conditions

```go
result, err := User.Find(ctx, onetable.Item{}, &onetable.Params{
    Where: `(${role} = {admin}) and (${status} = {active})`,
})
```

### Using substitutions

```go
result, err := User.Find(ctx, onetable.Item{}, &onetable.Params{
    Where: `(${role} = {admin}) and (${status} = @{currentStatus})`,
    Substitutions: map[string]any{
        "currentStatus": "active",
    },
})
```

### Splat / IN operator

Use `@{...varName}` to expand a slice into individual `ExpressionAttributeValues` entries for `IN`:

```go
result, err := User.Find(ctx, onetable.Item{}, &onetable.Params{
    Where: `${role} IN (@{...roles})`,
    Substitutions: map[string]any{
        "roles": []any{"admin", "owner", "editor"},
    },
})
```

### Attribute existence

```go
// Items where "deletedAt" is not set
result, err := User.Find(ctx, onetable.Item{}, &onetable.Params{
    Where: `attribute_not_exists(${deletedAt})`,
})
```

### Conditional update / create

`Params.Where` also works as a condition expression for `Create`, `Update` and `Remove`:

```go
// Only update if the current version matches
_, err := Order.Update(ctx, onetable.Item{"id": id, "status": "shipped"}, &onetable.Params{
    Where: `${version} = {3}`,
})
```

### Sort key filter on queryItems / find

```go
result, err := table.QueryItems(ctx, onetable.Item{"pk": "order#acct1"}, &onetable.Params{
    Where: `${sk} <= {1000}`,
})
```

---

## Supported operators

| Category | Operators |
|----------|-----------|
| Comparison | `<`, `<=`, `=`, `<>`, `>=`, `>` |
| Logical | `AND`, `OR`, `NOT` |
| Range | `BETWEEN` |
| Membership | `IN` |
| String | `begins_with(attr, substr)` |
| Contains | `contains(attr, value)`, `not_contains(attr, value)` |
| Existence | `attribute_exists(attr)`, `attribute_not_exists(attr)` |
| Type check | `attribute_type(attr, type)` |
| Size | `size(attr)` |
| Grouping | `()` |

> `<>` is DynamoDB's not-equals operator (not `!=`).

See the [AWS Comparison Expression Reference](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.OperatorsAndFunctions.html) for full details.

---

## Notes

- Property names inside `${}` are the **schema field names**, not the DynamoDB attribute names. OneTable resolves `map:` renames automatically.
- Where clauses on non-key fields in `Find` / `Scan` produce a `FilterExpression` (applied after reading, before returning). They do not reduce the amount of data DynamoDB reads; `Params.Limit` still counts raw scanned items.
- Where clauses on key fields are not supported via `Where`; use sort-key condition maps instead (see [table.md — QueryItems](table.md#queryitems)).
