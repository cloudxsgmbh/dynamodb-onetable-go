# OneTable (Go)

Go port of `dynamodb-onetable` (single-table DynamoDB library). API mirrors JS version. Uses AWS SDK v2.

## Install

```bash
go get github.com/cloudxs/onetable
```

## Quick start

```go
import (
    "context"

    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    ot "github.com/cloudxs/onetable"
)

client := dynamodb.New(dynamodb.Options{ /* config */ })

table, err := ot.NewTable(ot.TableParams{
    Name:   "MyTable",
    Client: client,
    Schema: MySchema,
})
if err != nil {
    // handle
}

user, err := table.Create(context.Background(), "User", ot.Item{
    "name":  "Peter Smith",
    "email": "peter@example.com",
}, nil)

_ = user
```

## Schema

```go
var MySchema = &ot.SchemaDef{
    Format:  "onetable:1.1.0",
    Version: "0.0.1",
    Indexes: map[string]*ot.IndexDef{
        "primary": {Hash: "pk", Sort: "sk"},
        "gs1":     {Hash: "gs1pk", Sort: "gs1sk", Project: "all"},
    },
    Models: map[string]ot.ModelDef{
        "Account": {
            "pk":    {Type: ot.FieldTypeString, Value: "account#${id}"},
            "sk":    {Type: ot.FieldTypeString, Value: "account#"},
            "id":    {Type: ot.FieldTypeString, Generate: "ulid"},
            "name":  {Type: ot.FieldTypeString, Required: true},
            "status": {Type: ot.FieldTypeString, Default: "active"},
        },
        "User": {
            "pk":    {Type: ot.FieldTypeString, Value: "account#${accountId}"},
            "sk":    {Type: ot.FieldTypeString, Value: "user#${id}"},
            "id":    {Type: ot.FieldTypeString, Generate: "ulid"},
            "email": {Type: ot.FieldTypeString, Required: true},
            "gs1pk": {Type: ot.FieldTypeString, Value: "user-email#${email}"},
            "gs1sk": {Type: ot.FieldTypeString, Value: "user#"},
        },
    },
    Params: &ot.SchemaParams{
        IsoDates:   true,
        Timestamps: true,
    },
}
```

## Common operations

```go
ctx := context.Background()

// Get
user, _ := table.Get(ctx, "User", ot.Item{"id": "01H..."}, nil)

// Update
user, _ = table.Update(ctx, "User", ot.Item{"id": "01H...", "status": "active"}, nil)

// Upsert (no existence check)
user, _ = table.Upsert(ctx, "User", ot.Item{"email": "a@b", "name": "Ann"}, nil)

// Find / Scan
result, _ := table.Find(ctx, "User", ot.Item{"accountId": "acc"}, nil)
_ = result.Items

// Secondary index
user, _ = table.Get(ctx, "User", ot.Item{"email": "a@b"}, &ot.Params{Index: "gs1"})
```

## Batch / Transact

```go
batch := map[string]any{}
_, _ = table.Create(ctx, "User", ot.Item{"name": "A", "email": "a@b"}, &ot.Params{Batch: batch})
_, _ = table.Create(ctx, "User", ot.Item{"name": "B", "email": "b@b"}, &ot.Params{Batch: batch})
_, _ = table.BatchWrite(ctx, batch, nil)

trx := map[string]any{}
_, _ = table.Update(ctx, "User", ot.Item{"id": "01H...", "status": "active"}, &ot.Params{Transaction: trx})
_, _ = table.Transact(ctx, "write", trx, nil)
```

## Unique fields

```go
// Schema field: {Unique: true}
user, err := table.Create(ctx, "User", ot.Item{
    "name": "Judy",
    "email": "judy@example.com",
}, nil)
```

Duplicate unique values return `ErrUnique`.

## Context

```go
table.SetContext(ot.Item{"accountId": "acc"}, false)
user, _ := table.Create(ctx, "User", ot.Item{"name": "A", "email": "a@b"}, nil)
```

## Tests

All tests run against an in-memory mock (no DynamoDB required).

```bash
go test ./...
```

## Docs

JS docs apply conceptually: https://doc.onetable.io/ (API names are Go-idiomatic).