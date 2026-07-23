# API Reference

Complete API documentation for the `onetable` Go package.

---

## Table API

[`table.md`](table.md) — Table methods: convenience model wrappers, low-level item API, schema management, DDL, context, UID helpers, and the `DynamoClient` interface.

| Category | Methods |
|----------|---------|
| Convenience model | `Create`, `Get`, `Find`, `Update`, `Upsert`, `Remove`, `Scan` |
| Low-level item | `GetItem`, `PutItem`, `DeleteItem`, `UpdateItem`, `QueryItems`, `ScanItems` |
| Batch | `BatchGet`, `BatchWrite` |
| Transaction | `Transact` |
| Item collection | `Fetch`, `GroupByType` |
| Schema | `SetSchema`, `GetCurrentSchema`, `GetKeys`, `SaveSchema`, `ReadSchema`, `ReadSchemas`, `RemoveSchema` |
| Model registry | `GetModel`, `AddModel`, `RemoveModel`, `ListModels` |
| Context | `GetContext`, `SetContext`, `AddContext`, `ClearContext` |
| DDL | `CreateTable`, `DeleteTable`, `DescribeTable`, `Exists`, `ListTables`, `UpdateTable`, `GetTableDefinition` |
| Client/logging | `SetClient`, `GetLog`, `SetLog` |
| UID helpers | `UUID`, `ULID`, `UID` |

---

## Model API

[`model.md`](model.md) — Model methods: entity-level CRUD, scanning, and unique-field management.

| Method | DynamoDB op | Description |
|--------|-------------|-------------|
| `Create` | `PutItem` | Create a new item |
| `Get` | `GetItem` | Fetch a single item by key |
| `Find` | `Query` | Query items with key conditions and filters |
| `Update` | `UpdateItem` | Update an item (atomic ops, expressions) |
| `Upsert` | `UpdateItem` | Update-or-create |
| `Remove` | `DeleteItem` | Delete an item |
| `Scan` | `Scan` | Full-table scan filtered by model type |
| `Init` | — | Construct a default item without writing |

---

## Batch Operations

[`batch.md`](batch.md) — Accumulate multiple reads or writes into a single DynamoDB batch request.

| Method | Max items | Atomicity |
|--------|-----------|-----------|
| `BatchGet` | 100 | No |
| `BatchWrite` | 25 | No |

---

## Transaction Operations

[`transact.md`](transact.md) — Atomic all-or-nothing operations across multiple items.

| Method | Max items | Atomicity |
|--------|-----------|-----------|
| `Transact("write", ...)` | 100 | Yes |
| `Transact("get", ...)` | 100 | Yes |

---

## Related docs

| Document | Description |
|----------|-------------|
| [Schema](../schema.md) | `SchemaDef`, `IndexDef`, `FieldDef`, value templates |
| [Params](../params.md) | All operation parameters |
| [Where clauses](../where.md) | Filter and condition expression syntax |
| [Errors](../errors.md) | Error types and codes |
