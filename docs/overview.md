# DynamoDB OneTable Go — API Overview

Package `onetable` is a Go port of the [JavaScript OneTable](https://github.com/sensedeep/dynamodb-onetable) library. It wraps the AWS SDK v2 DynamoDB client and provides a high-level, schema-driven API for working with a single DynamoDB table.

## Core concepts

| Concept | Description |
|---------|-------------|
| **Table** | Top-level object. Owns the schema, client, context and all configuration. |
| **Model** | Represents one entity type stored in the table. Obtained via `table.GetModel(name)`. |
| **Schema** | Declarative definition of indexes, models and their fields (`SchemaDef`). |
| **Item** | `map[string]any` — the universal property map used for inputs and outputs. |
| **Params** | Optional `*Params` argument that controls every API call (pagination, conditions, etc.). |

## Packages

```
github.com/cloudxsgmbh/dynamodb-onetable-go   # main package
```

## Quick start

```go
import (
    "context"
    onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

schema := &onetable.SchemaDef{
    Version: "0.0.1",
    Indexes: map[string]*onetable.IndexDef{
        "primary": {Hash: "pk", Sort: "sk"},
    },
    Models: map[string]onetable.ModelDef{
        "User": {
            "pk":    {Type: "string", Value: "user#${id}"},
            "sk":    {Type: "string", Value: "user#${id}"},
            "id":    {Type: "string", Generate: "ulid"},
            "name":  {Type: "string", Required: true},
            "email": {Type: "string", Required: true, Unique: true},
        },
    },
    Params: &onetable.SchemaParams{Timestamps: true},
}

table, err := onetable.NewTable(onetable.TableParams{
    Name:   "MyTable",
    Client: dynamoClient, // aws-sdk-go-v2 DynamoDB client
    Schema: schema,
})

User, err := table.GetModel("User")
user, err := User.Create(ctx, onetable.Item{"name": "Alice", "email": "alice@example.com"}, nil)
```

## Documents

| Document | Contents |
|----------|----------|
| [table.md](table.md) | `Table` — constructor, context, DDL, low-level item API, batch/transact |
| [model.md](model.md) | `Model` — Create / Get / Find / Update / Upsert / Remove / Scan |
| [schema.md](schema.md) | Schema types: `SchemaDef`, `IndexDef`, `FieldDef`, `SchemaParams` |
| [params.md](params.md) | `Params` — all operation parameters explained |
| [where.md](where.md) | Where-clause syntax for filter and condition expressions |
| [errors.md](errors.md) | `OneTableError`, `OneTableArgError`, error codes |
