/*
Package onetable – schema types.

Mirrors JS: Schema.js field/index/schema definitions.
*/
package onetable

// FieldType names match JS ValidTypes.
type FieldType string

const (
	FieldTypeArray       FieldType = "array"
	FieldTypeArrayBuffer FieldType = "arraybuffer"
	FieldTypeBinary      FieldType = "binary"
	FieldTypeBoolean     FieldType = "boolean"
	FieldTypeBuffer      FieldType = "buffer"
	FieldTypeDate        FieldType = "date"
	FieldTypeNumber      FieldType = "number"
	FieldTypeObject      FieldType = "object"
	FieldTypeSet         FieldType = "set"
	FieldTypeString      FieldType = "string"
)

var validFieldTypes = map[FieldType]bool{
	FieldTypeArray: true, FieldTypeArrayBuffer: true, FieldTypeBinary: true,
	FieldTypeBoolean: true, FieldTypeBuffer: true, FieldTypeDate: true,
	FieldTypeNumber: true, FieldTypeObject: true, FieldTypeSet: true,
	FieldTypeString: true,
}

// IndexDef describes a primary or secondary index.
type IndexDef struct {
	Hash    string `json:"hash,omitempty"`
	Sort    string `json:"sort,omitempty"`
	Type    string `json:"type,omitempty"`    // "local" for LSI
	Project any    `json:"project,omitempty"` // "all"|"keys"|[]string
	Follow  bool   `json:"follow,omitempty"`
}

// FieldDef is a single field definition inside a model.
// All fields are optional to allow partial schema definitions.
type FieldDef struct {
	Type     FieldType `json:"type,omitempty"`
	Required bool      `json:"required,omitempty"`
	Hidden   *bool     `json:"hidden,omitempty"` // pointer: nil = unset
	Default  any       `json:"default,omitempty"`
	Value    string    `json:"value,omitempty"` // template e.g. "${_type}#${id}"
	Generate string    `json:"generate,omitempty"` // "uuid"|"ulid"|"uid"|"uid(n)"
	Validate string    `json:"validate,omitempty"` // regex string "/pat/flags"
	Enum     []string  `json:"enum,omitempty"`
	Map      string    `json:"map,omitempty"` // "attr" or "attr.sub"
	Encode   any       `json:"encode,omitempty"`
	Crypt    bool      `json:"crypt,omitempty"`
	IsoDates *bool     `json:"isoDates,omitempty"`
	Nulls    *bool     `json:"nulls,omitempty"`
	Unique   bool      `json:"unique,omitempty"`
	Scope    string    `json:"scope,omitempty"`
	TTL      bool      `json:"ttl,omitempty"`
	Fixed    bool      `json:"fixed,omitempty"`
	Partial  *bool     `json:"partial,omitempty"`
	Filter   *bool     `json:"filter,omitempty"` // false disables field from filter expressions
	Schema   FieldMap  `json:"schema,omitempty"` // nested schema
	Items    *ItemsDef `json:"items,omitempty"`  // for array element schema
}

// ItemsDef describes the schema of array elements.
type ItemsDef struct {
	Schema FieldMap `json:"schema,omitempty"`
}

// FieldMap is a map of field name → definition.
type FieldMap map[string]*FieldDef

// ModelDef is the schema for one model (entity type).
type ModelDef = FieldMap

// SchemaParams holds table-level behavioural flags.
type SchemaParams struct {
	CreatedField string `json:"createdField,omitempty"`
	UpdatedField string `json:"updatedField,omitempty"`
	TypeField    string `json:"typeField,omitempty"`
	Separator    string `json:"separator,omitempty"`
	IsoDates     bool   `json:"isoDates,omitempty"`
	Nulls        bool   `json:"nulls,omitempty"`
	Timestamps   any    `json:"timestamps,omitempty"` // bool | "create" | "update"
	Warn         bool   `json:"warn,omitempty"`
}

// SchemaDef is the top-level schema object passed to Table.
type SchemaDef struct {
	Format  string               `json:"format,omitempty"`
	Version string               `json:"version"`
	Indexes map[string]*IndexDef `json:"indexes"`
	Models  map[string]ModelDef  `json:"models"`
	Params  *SchemaParams        `json:"params,omitempty"`
	Process map[string]any       `json:"process,omitempty"`
	Queries map[string]any       `json:"queries,omitempty"`
	Name    string               `json:"name,omitempty"`
}

// prepared field (internal, built from FieldDef during model prep)

// preparedField is the runtime representation of a schema field.
// Built once during Model preparation, read-only afterwards.
type preparedField struct {
	// identity
	Name string
	Def  *FieldDef

	// resolved type
	Type FieldType

	// DynamoDB attribute mapping: [attrName] or [attrName, subProp]
	Attribute []string

	// index tracking
	IsIndexed bool // this field is a key in some index
	IsPrimary bool // it's the primary hash or sort

	// flags (resolved from Def + table defaults)
	Hidden   bool
	Required bool
	Nulls    bool
	IsoDates bool
	Partial  *bool // nil = use table default

	// value template (non-empty means computed)
	ValueTemplate string

	// encode: [attrName, separator, index]
	Encode []any

	// nested block (for object/array with sub-schema)
	Block *fieldBlock

	IsArray bool // array-type with items schema
}

// fieldBlock groups a set of prepared fields with their dependency order.
type fieldBlock struct {
	Fields map[string]*preparedField
	Deps   []*preparedField // ordered for template evaluation
}
