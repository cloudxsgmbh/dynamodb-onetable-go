# Errors

All API errors are returned as Go `error` values. Two concrete types are used:

---

## OneTableError

General runtime error. Returned by all `Model` and `Table` operations.

```go
type OneTableError struct {
    Message string
    Code    ErrorCode
    Context map[string]any // extra debugging data
    Cause   error          // wrapped underlying error
}

func (e *OneTableError) Error() string
func (e *OneTableError) Unwrap() error
```

Construct with:

```go
err := onetable.NewError("something failed",
    onetable.WithCode(onetable.ErrRuntime),
    onetable.WithContext(map[string]any{"id": id}),
    onetable.WithCause(originalErr),
)
```

---

## OneTableArgError

Invalid argument / configuration error. Returned during table/schema setup.

```go
type OneTableArgError struct {
    Message string
    Code    ErrorCode
    Context map[string]any
}

func (e *OneTableArgError) Error() string
```

Construct with:

```go
err := onetable.NewArgError("Missing required field")
```

---

## Error codes

| Constant | Value | Meaning |
|----------|-------|---------|
| `ErrArgument` | `"ArgumentError"` | Invalid argument or configuration. |
| `ErrValidation` | `"ValidationError"` | Schema validation failed (required field, enum, regex). |
| `ErrMissing` | `"MissingError"` | Required key or data is missing. |
| `ErrNonUnique` | `"NonUniqueError"` | An operation that expected one item returned multiple. |
| `ErrUnique` | `"UniqueError"` | A `unique: true` constraint was violated. |
| `ErrNotFound` | `"NotFoundError"` | Expected item does not exist. |
| `ErrRuntime` | `"RuntimeError"` | DynamoDB or other runtime error. |
| `ErrType` | `"TypeError"` | Type mismatch. |

---

## Checking error types

```go
import (
    "errors"
    onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

item, err := User.Create(ctx, props, nil)
if err != nil {
    var otErr *onetable.OneTableError
    if errors.As(err, &otErr) {
        switch otErr.Code {
        case onetable.ErrUnique:
            // handle unique constraint violation
        case onetable.ErrValidation:
            // handle validation error
            fmt.Println(otErr.Context["validation"])
        default:
            // other runtime error
        }
    }
    return err
}
```

---

## Common error scenarios

| Scenario | Code | Notes |
|----------|------|-------|
| Create with duplicate key | `ErrRuntime` | `ConditionalCheckFailedException` from DynamoDB — `Params.Exists` defaults to `false` for create. |
| Create / update with duplicate unique field | `ErrUnique` | Unique sentinel already exists; transaction was cancelled. |
| Update non-existent item (no unique fields) | `ErrRuntime` | DynamoDB `ConditionalCheckFailedException` — `Params.Exists` defaults to `true`. |
| Update non-existent item (unique fields) | `ErrNotFound` | Prior item fetched explicitly before transaction; not found. |
| Missing required field | `ErrValidation` | Check `otErr.Context["validation"]` for per-field details. |
| Invalid enum value | `ErrValidation` | Field value not in `FieldDef.Enum`. |
| Missing primary key | `ErrMissing` | Cannot build the key expression. |
| Get returns multiple items | `ErrNonUnique` | `Model.Get` without sort key; use `Find` instead. |
| Remove multiple without `Many: true` | `ErrNonUnique` | Set `Params.Many = true` to allow batch removal. |
| DynamoDB throughput exceeded | `ErrRuntime` | `ProvisionedThroughputExceededException` from AWS. |
| Transaction cancelled | `ErrRuntime` | `TransactionCanceledException` from AWS. |
