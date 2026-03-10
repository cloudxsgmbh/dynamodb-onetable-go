# Schema

The schema is the central configuration object that describes your table's indexes and data models. Pass it to `NewTable` via `TableParams.Schema`, or apply it later with `Table.SetSchema`.

```go
schema := &onetable.SchemaDef{
    Version: "0.0.1",
    Indexes: map[string]*onetable.IndexDef{ ... },
    Models:  map[string]onetable.ModelDef{ ... },
    Params:  &onetable.SchemaParams{ ... },
}
```

---

## SchemaDef

```go
type SchemaDef struct {
    Format  string
    Version string
    Indexes map[string]*IndexDef
    Models  map[string]ModelDef
    Params  *SchemaParams
    Name    string
}
```

| Field | Description |
|-------|-------------|
| `Version` | Schema version string (informational). |
| `Indexes` | Index definitions; must include at least `"primary"`. |
| `Models` | Map of model name → `ModelDef` (which is `FieldMap`). |
| `Params` | Table-level behavioural defaults. |
| `Name` | Optional schema name (used when persisting schemas). |
| `Format` | Optional format identifier. |

---

## IndexDef

```go
type IndexDef struct {
    Hash    string
    Sort    string
    Type    string // "local" for LSI; omit for GSI
    Project any    // "all" | "keys" | []string
    Follow  bool
}
```

| Field | Description |
|-------|-------------|
| `Hash` | DynamoDB attribute name for the partition key. |
| `Sort` | DynamoDB attribute name for the sort key (omit for hash-only indexes). |
| `Type` | Set to `"local"` for a Local Secondary Index. |
| `Project` | Projection: `"all"` (default), `"keys"` (KEYS_ONLY), or a `[]string` of attribute names (INCLUDE). |
| `Follow` | If `true`, automatically re-fetch from the primary index after a query on this index (useful for KEYS_ONLY indexes). |

**Example:**

```go
Indexes: map[string]*onetable.IndexDef{
    "primary": {Hash: "pk", Sort: "sk"},
    "gs1":     {Hash: "gs1pk", Sort: "gs1sk", Project: "all"},
    "gs2":     {Hash: "gs2pk", Project: "keys"},     // hash-only GSI, KEYS_ONLY
    "ls1":     {Hash: "pk", Sort: "ls1sk", Type: "local"},
},
```

---

## SchemaParams

Table-level behavioural defaults. Applied when the schema is loaded.

```go
type SchemaParams struct {
    CreatedField string // attribute name for creation timestamp (default "created")
    UpdatedField string // attribute name for update timestamp  (default "updated")
    TypeField    string // attribute name for model type         (default "_type")
    Separator    string // separator used in value templates     (default "#")
    IsoDates     bool   // true → dates stored as ISO-8601 strings; false → epoch ms (int64)
    Nulls        bool   // true → write null fields; false → omit them
    Timestamps   any    // true | false | "create" | "update"
    Warn         bool   // log warnings for schema mismatches
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `CreatedField` | `"created"` | Name of the auto-managed creation-timestamp field. |
| `UpdatedField` | `"updated"` | Name of the auto-managed update-timestamp field. |
| `TypeField` | `"_type"` | Attribute that stores the model name (hidden by default). |
| `Separator` | `"#"` | Character used to join components in value templates. |
| `IsoDates` | `false` | Store dates as RFC3339 strings (`true`) or epoch milliseconds (`false`). |
| `Nulls` | `false` | Write `null` attributes to DynamoDB (`true`) or remove them (`false`). |
| `Timestamps` | `false` | `true` — manage both `created` and `updated`. `"create"` — only `created`. `"update"` — only `updated`. |
| `Warn` | `false` | Log warnings when schema validation detects mismatches. |

---

## FieldDef

Every model field is described by a `*FieldDef`:

```go
type FieldDef struct {
    Type     FieldType
    Required bool
    Hidden   *bool    // nil = inherit table default
    Default  any
    Value    string   // value template, e.g. "${_type}#${id}"
    Generate string   // "uuid" | "ulid" | "uid" | "uid(n)"
    Validate string   // regex, e.g. "/^[a-z]+$/i"
    Enum     []string
    Map      string   // DynamoDB attribute mapping, e.g. "pk" or "data.email"
    Encode   any      // packed encoding: [attrName, separator, index]
    Crypt    bool
    IsoDates *bool    // override table IsoDates for this field
    Nulls    *bool    // override table Nulls for this field
    Unique   bool
    Scope    string   // template for unique scope (default: global)
    TTL      bool     // treat as a DynamoDB TTL attribute (epoch seconds)
    Fixed    bool
    Partial  *bool    // override table Partial for nested objects
    Filter   *bool    // false → exclude from filter expressions
    Schema   FieldMap // nested schema for object/array fields
    Items    *ItemsDef // schema for array element type
}
```

### FieldType values

| Constant | DynamoDB type |
|----------|---------------|
| `"string"` | S |
| `"number"` | N |
| `"boolean"` | BOOL |
| `"date"` | N (epoch ms) or S (ISO-8601) |
| `"object"` | M |
| `"array"` | L |
| `"set"` | SS / NS |
| `"buffer"` | B |
| `"binary"` | B |
| `"arraybuffer"` | B |

### Field properties

| Property | Type | Description |
|----------|------|-------------|
| `Type` | `FieldType` | Data type. Defaults to `"string"`. |
| `Required` | `bool` | `true` → error if not present on create/update. |
| `Hidden` | `*bool` | `true` → field is stored in DynamoDB but not returned in read results. Useful for index key attributes. |
| `Default` | `any` | Default value applied on create if the field is absent. |
| `Value` | `string` | Value template (see below). The field is computed from other properties; callers should not set it directly. Fields with a `Value` template are **hidden by default** (as if `Hidden: true`) unless `Hidden` is explicitly set to `false`. |
| `Generate` | `string` | Auto-generate: `"ulid"`, `"uuid"`, `"uid"`, `"uid(n)"`. Applied on create. `uid` defaults to length 10. |
| `Validate` | `string` | Regex validation pattern, e.g. `"/^\\d+$/"` or `"^\\d+$"`. |
| `Enum` | `[]string` | Allowed values. Validation error if the value is not in the list. |
| `Map` | `string` | Maps this Go field name to a different DynamoDB attribute name, or a `"attr.subprop"` path for packed attributes. |
| `Encode` | `any` | Packed encoding: store multiple fields in one attribute, separated by a delimiter. Format: `[attrName, separator, index]`. |
| `Crypt` | `bool` | Encrypt/decrypt this field transparently using the table crypto config. |
| `IsoDates` | `*bool` | Override the table-level `IsoDates` setting for this date field. |
| `Nulls` | `*bool` | Override the table-level `Nulls` setting for this field. |
| `Unique` | `bool` | Enforce uniqueness across all items via a transparent transaction. |
| `Scope` | `string` | Value template for the unique-constraint scope (limits uniqueness to a domain, e.g. per-account). |
| `TTL` | `bool` | Treat as a DynamoDB TTL attribute; value is stored/returned as Unix epoch seconds. |
| `Partial` | `*bool` | For nested objects: whether partial updates are allowed by default. |
| `Filter` | `*bool` | Set `false` to exclude this field from filter expressions. |
| `Schema` | `FieldMap` | Nested field schema for `object` or `array` fields. |
| `Items` | `*ItemsDef` | Schema for individual array elements (use `Items.Schema`). |

---

## Value templates

Value templates (`FieldDef.Value`) let you compute key and attribute values from other properties at runtime. They use `${propertyName}` substitutions:

```go
"pk":    {Type: "string", Value: "user#${id}"},           // hidden by default (has Value template)
"sk":    {Type: "string", Value: "user#${id}"},           // hidden by default
"gs1pk": {Type: "string", Value: "account#${accountId}"}, // hidden by default
```

Templates support optional padding:

```go
// zero-pad `seq` to 8 characters
"sk": {Value: "order#${seq:8:0}"},
```

On `Find` calls, when a template cannot be fully resolved (missing properties), OneTable truncates at the first unresolvable variable and synthesises a `begins_with` sort-key condition automatically.

The reserved variable `${_type}` expands to the model name.

---

## ModelDef (FieldMap)

A model definition is simply a `FieldMap` — a `map[string]*FieldDef`:

```go
type ModelDef = FieldMap
type FieldMap  = map[string]*FieldDef
```

```go
// helper (boolPtr is unexported in onetable; define your own or use a local var)
func bp(b bool) *bool { return &b }

Models: map[string]onetable.ModelDef{
    "User": {
        // Value-template fields are hidden by default; Hidden can be omitted here
        "pk":        {Type: "string", Value: "user#${id}"},
        "sk":        {Type: "string", Value: "user#${id}"},
        "id":        {Type: "string", Generate: "ulid"},
        "name":      {Type: "string", Required: true},
        "email":     {Type: "string", Required: true, Unique: true},
        "role":      {Type: "string", Enum: []string{"admin", "user", "guest"}, Default: "user"},
        "status":    {Type: "string"},
        "created":   {Type: "date"},
        "updated":   {Type: "date"},
        "gs1pk":     {Type: "string", Value: "user#"},           // hidden by default
        "gs1sk":     {Type: "string", Value: "user#${email}"},   // hidden by default
    },
},
```

---

## Nested schemas

Use `FieldDef.Schema` (for objects) or `FieldDef.Items.Schema` (for array elements) to define sub-schemas:

```go
"address": {
    Type: "object",
    Schema: onetable.FieldMap{
        "street":  {Type: "string"},
        "city":    {Type: "string", Required: true},
        "country": {Type: "string", Default: "US"},
    },
},
"tags": {
    Type: "array",
    Items: &onetable.ItemsDef{
        Schema: onetable.FieldMap{
            "key":   {Type: "string", Required: true},
            "value": {Type: "string"},
        },
    },
},
```
