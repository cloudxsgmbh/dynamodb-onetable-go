/*
Package onetable – schema manager.

Mirrors JS: Schema.js – lifecycle, standard models, DynamoDB key discovery.
*/
package onetable

import (
	"context"
	"fmt"
	"strings"
)

const (
	genericModelName   = "_Generic"
	migrationModelName = "_Migration"
	schemaModelName    = "_Schema"
	uniqueModelName    = "_Unique"
	migrationKey       = "_migration"
	schemaKey          = "_schema"
	schemaFormat       = "onetable:1.1.0"
)

// schemaManager holds the active schema state for a Table.
type schemaManager struct {
	table      *Table
	indexes    map[string]*IndexDef
	models     map[string]*Model
	definition *SchemaDef
	params     SchemaParams
	keyTypes   map[string]string // attrName → "string"|"number"
	process    map[string]any

	// internal models (not in schema.models directly)
	uniqueModel    *Model
	genericModel   *Model
	schemaModel    *Model
	migrationModel *Model
}

func newSchemaManager(table *Table, schema *SchemaDef) *schemaManager {
	sm := &schemaManager{
		table:    table,
		models:   map[string]*Model{},
		keyTypes: map[string]string{},
	}
	sm.params = table.getSchemaParams()
	sm.setSchemaInner(schema)
	return sm
}

func (sm *schemaManager) setSchemaInner(schema *SchemaDef) {
	sm.models = map[string]*Model{}
	sm.indexes = nil
	if schema == nil {
		return
	}
	sm.validateSchema(schema)
	sm.definition = schema
	sm.indexes = schema.Indexes

	if schema.Params != nil {
		sm.table.setSchemaParams(schema.Params)
	}
	sm.params = sm.table.getSchemaParams()

	for name, modelDef := range schema.Models {
		if name == schemaModelName || name == migrationModelName {
			continue
		}
		sm.models[name] = newModel(sm.table, name, modelOptions{Fields: modelDef, Indexes: sm.indexes})
	}
	sm.createStandardModels()
	sm.process = schema.Process
}

func (sm *schemaManager) validateSchema(schema *SchemaDef) {
	if schema.Version == "" {
		panic("schema is missing a version")
	}
	if schema.Indexes == nil {
		panic("schema is missing indexes")
	}
	primary, ok := schema.Indexes["primary"]
	if !ok {
		panic("schema is missing a primary index")
	}
	var lsiCount int
	for name, idx := range schema.Indexes {
		if name == "primary" {
			continue
		}
		if idx.Type == "local" {
			if idx.Hash != "" && idx.Hash != primary.Hash {
				panic(fmt.Sprintf(`LSI "%s" should not define a different hash than primary`, name))
			}
			if idx.Sort == "" {
				panic(fmt.Sprintf(`LSI "%s" must define a sort attribute`, name))
			}
			idx.Hash = primary.Hash
			lsiCount++
		} else if idx.Hash == "" {
			// treat as LSI if no hash defined
			idx.Type = "local"
			idx.Hash = primary.Hash
			lsiCount++
		}
	}
	if lsiCount > 5 {
		panic("schema has too many LSIs (max 5)")
	}
}

func (sm *schemaManager) createStandardModels() {
	sm.createUniqueModel()
	sm.createGenericModel()
	sm.createSchemaModel()
	sm.createMigrationModel()
}

func (sm *schemaManager) createUniqueModel() {
	primary := sm.indexes["primary"]
	t := sm.keyTypes[primary.Hash]
	if t == "" {
		t = "string"
	}
	fields := FieldMap{
		primary.Hash: {Type: FieldType(t)},
	}
	if primary.Sort != "" {
		ts := sm.keyTypes[primary.Sort]
		if ts == "" {
			ts = "string"
		}
		fields[primary.Sort] = &FieldDef{Type: FieldType(ts)}
	}
	sm.uniqueModel = newModel(sm.table, uniqueModelName, modelOptions{
		Fields:     fields,
		Timestamps: false,
		Indexes:    sm.indexes,
	})
}

func (sm *schemaManager) createGenericModel() {
	primary := sm.indexes["primary"]
	t := sm.keyTypes[primary.Hash]
	if t == "" {
		t = "string"
	}
	fields := FieldMap{
		primary.Hash: {Type: FieldType(t)},
	}
	if primary.Sort != "" {
		ts := sm.keyTypes[primary.Sort]
		if ts == "" {
			ts = "string"
		}
		fields[primary.Sort] = &FieldDef{Type: FieldType(ts)}
	}
	sm.genericModel = newModel(sm.table, genericModelName, modelOptions{
		Fields:     fields,
		Timestamps: false,
		Generic:    true,
		Indexes:    sm.indexes,
	})
}

func (sm *schemaManager) createSchemaModel() {
	primary := sm.indexes["primary"]
	hidden := true
	fields := FieldMap{
		primary.Hash: {Type: FieldTypeString, Required: true, Value: schemaKey},
		"format":     {Type: FieldTypeString, Required: true},
		"indexes":    {Type: FieldTypeObject, Required: true},
		"name":       {Type: FieldTypeString, Required: true},
		"models":     {Type: FieldTypeObject, Required: true},
		"params":     {Type: FieldTypeObject, Required: true},
		"queries":    {Type: FieldTypeObject, Required: true},
		"process":    {Type: FieldTypeObject},
		"version":    {Type: FieldTypeString, Required: true},
	}
	if primary.Sort != "" {
		fields[primary.Sort] = &FieldDef{
			Type:     FieldTypeString,
			Required: true,
			Value:    schemaKey + ":${name}",
			Hidden:   &hidden,
		}
	}
	sm.schemaModel = newModel(sm.table, schemaModelName, modelOptions{Fields: fields, Indexes: sm.indexes})
	sm.models[schemaModelName] = sm.schemaModel
}

func (sm *schemaManager) createMigrationModel() {
	primary := sm.indexes["primary"]
	fields := FieldMap{
		primary.Hash:  {Type: FieldTypeString, Value: migrationKey},
		"date":        {Type: FieldTypeDate, Required: true},
		"description": {Type: FieldTypeString, Required: true},
		"path":        {Type: FieldTypeString, Required: true},
		"version":     {Type: FieldTypeString, Required: true},
		"status":      {Type: FieldTypeString},
	}
	if primary.Sort != "" {
		fields[primary.Sort] = &FieldDef{
			Type:  FieldTypeString,
			Value: migrationKey + ":${version}:${date}",
		}
	}
	sm.migrationModel = newModel(sm.table, migrationModelName, modelOptions{Fields: fields, Indexes: sm.indexes})
	sm.models[migrationModelName] = sm.migrationModel
}

// SetSchema replaces the active schema.
func (sm *schemaManager) SetSchema(schema *SchemaDef) map[string]*IndexDef {
	sm.setSchemaInner(schema)
	return sm.indexes
}

// GetKeys reads the DynamoDB table description to discover index keys when no
// schema was provided.
func (sm *schemaManager) GetKeys(ctx context.Context, refresh bool) (map[string]*IndexDef, error) {
	if sm.indexes != nil && !refresh {
		return sm.indexes, nil
	}
	info, err := sm.table.DescribeTable(ctx)
	if err != nil {
		return nil, err
	}
	tbl := info["Table"].(map[string]any)

	for _, def := range tbl["AttributeDefinitions"].([]any) {
		d := def.(map[string]any)
		name := d["AttributeName"].(string)
		at := d["AttributeType"].(string)
		if at == "N" {
			sm.keyTypes[name] = "number"
		} else {
			sm.keyTypes[name] = "string"
		}
	}

	indexes := map[string]*IndexDef{"primary": {}}
	for _, ks := range tbl["KeySchema"].([]any) {
		k := ks.(map[string]any)
		if strings.ToLower(k["KeyType"].(string)) == "hash" {
			indexes["primary"].Hash = k["AttributeName"].(string)
		} else {
			indexes["primary"].Sort = k["AttributeName"].(string)
		}
	}
	if gsis, ok := tbl["GlobalSecondaryIndexes"].([]any); ok {
		for _, g := range gsis {
			gi := g.(map[string]any)
			name := gi["IndexName"].(string)
			idx := &IndexDef{}
			for _, ks := range gi["KeySchema"].([]any) {
				k := ks.(map[string]any)
				if strings.ToLower(k["KeyType"].(string)) == "hash" {
					idx.Hash = k["AttributeName"].(string)
				} else {
					idx.Sort = k["AttributeName"].(string)
				}
			}
			indexes[name] = idx
		}
	}
	sm.indexes = indexes
	sm.createStandardModels()
	return indexes, nil
}

// AddModel adds a model to the schema at runtime.
func (sm *schemaManager) AddModel(name string, fields FieldMap) {
	sm.models[name] = newModel(sm.table, name, modelOptions{Fields: fields})
}

// ListModels returns all model names.
func (sm *schemaManager) ListModels() []string {
	names := make([]string, 0, len(sm.models))
	for k := range sm.models {
		names = append(names, k)
	}
	return names
}

// GetModel retrieves a model by name.
func (sm *schemaManager) GetModel(name string, nothrow bool) (*Model, error) {
	if name == "" {
		if nothrow {
			return nil, nil
		}
		return nil, fmt.Errorf("undefined model name")
	}
	m := sm.models[name]
	if m == nil {
		if name == uniqueModelName {
			return sm.uniqueModel, nil
		}
		if nothrow {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot find model %s", name)
	}
	return m, nil
}

// RemoveModel deletes a model from the registry.
func (sm *schemaManager) RemoveModel(name string) error {
	if _, ok := sm.models[name]; !ok {
		return fmt.Errorf("cannot find model %s", name)
	}
	delete(sm.models, name)
	return nil
}

// GetCurrentSchema returns the current schema definition with resolved params.
func (sm *schemaManager) GetCurrentSchema() *SchemaDef {
	if sm.definition == nil {
		return nil
	}
	copy := *sm.definition
	p := sm.params
	copy.Params = &p
	copy.Process = sm.process
	return &copy
}

// SaveSchema persists the schema to the DynamoDB table.
func (sm *schemaManager) SaveSchema(ctx context.Context, schema *SchemaDef) error {
	if sm.indexes == nil {
		if _, err := sm.GetKeys(ctx, false); err != nil {
			return err
		}
	}
	if schema == nil {
		schema = sm.GetCurrentSchema()
	}
	if schema == nil {
		return fmt.Errorf("no schema to save")
	}
	if schema.Name == "" {
		schema.Name = "Current"
	}
	if schema.Version == "" {
		schema.Version = "0.0.1"
	}
	schema.Format = schemaFormat
	if schema.Indexes == nil {
		schema.Indexes = sm.indexes
	}
	if schema.Queries == nil {
		schema.Queries = map[string]any{}
	}

	_, err := sm.schemaModel.Create(ctx, Item{
		"name":    schema.Name,
		"version": schema.Version,
		"format":  schema.Format,
		"indexes": schema.Indexes,
		"models":  schema.Models,
		"params":  schema.Params,
		"queries": schema.Queries,
		"process": schema.Process,
	}, &Params{Exists: nil})
	return err
}

// ReadSchema reads the current schema from the table.
func (sm *schemaManager) ReadSchema(ctx context.Context) (*SchemaDef, error) {
	if sm.indexes == nil {
		if _, err := sm.GetKeys(ctx, false); err != nil {
			return nil, err
		}
	}
	primary := sm.indexes["primary"]
	props := Item{primary.Hash: schemaKey}
	if primary.Sort != "" {
		props[primary.Sort] = schemaKey + ":Current"
	}
	item, err := sm.table.GetItem(ctx, props, &Params{Hidden: boolPtr(true), Parse: true})
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return itemToSchemaDef(item), nil
}

// itemToSchemaDef is a best-effort conversion from a raw Item to SchemaDef.
func itemToSchemaDef(item Item) *SchemaDef {
	s := &SchemaDef{}
	if v, ok := item["name"].(string); ok {
		s.Name = v
	}
	if v, ok := item["version"].(string); ok {
		s.Version = v
	}
	if v, ok := item["format"].(string); ok {
		s.Format = v
	}
	return s
}


