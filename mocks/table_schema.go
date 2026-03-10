// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import (
	"context"

	onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

type MockTableSchema struct {
	SetSchemaFunc          func(context.Context, *onetable.SchemaDef) (map[string]*onetable.IndexDef, error)
	SetSchemaCalls         []SetSchemaCall
	SetSchemaResult        map[string]*onetable.IndexDef
	SetSchemaError         error
	GetCurrentSchemaFunc   func() *onetable.SchemaDef
	GetCurrentSchemaCalls  []GetCurrentSchemaCall
	GetCurrentSchemaResult *onetable.SchemaDef
	GetKeysFunc            func(context.Context) (map[string]*onetable.IndexDef, error)
	GetKeysCalls           []GetKeysCall
	GetKeysResult          map[string]*onetable.IndexDef
	GetKeysError           error
	GetModelFunc           func(string) (*onetable.Model, error)
	GetModelCalls          []GetModelCall
	GetModelResult         *onetable.Model
	GetModelError          error
	AddModelFunc           func(string, onetable.FieldMap)
	AddModelCalls          []AddModelCall
	RemoveModelFunc        func(string) error
	RemoveModelCalls       []RemoveModelCall
	RemoveModelError       error
	ListModelsFunc         func() []string
	ListModelsCalls        []ListModelsCall
	ListModelsResult       []string
	SetClientFunc          func(onetable.DynamoClient)
	SetClientCalls         []SetClientCall
	GetLogFunc             func() onetable.Logger
	GetLogCalls            []GetLogCall
	GetLogResult           onetable.Logger
	SetLogFunc             func(onetable.Logger)
	SetLogCalls            []SetLogCall
}

type SetSchemaCall struct {
	Ctx    context.Context
	Schema *onetable.SchemaDef
}

type GetCurrentSchemaCall struct{}

type GetKeysCall struct {
	Ctx context.Context
}

type GetModelCall struct {
	Name string
}

type AddModelCall struct {
	Name   string
	Fields onetable.FieldMap
}

type RemoveModelCall struct {
	Name string
}

type ListModelsCall struct{}

type SetClientCall struct {
	Client onetable.DynamoClient
}

type GetLogCall struct{}

type SetLogCall struct {
	Logger onetable.Logger
}

func (m *MockTableSchema) SetSchema(ctx context.Context, schema *onetable.SchemaDef) (map[string]*onetable.IndexDef, error) {
	m.SetSchemaCalls = append(m.SetSchemaCalls, SetSchemaCall{Ctx: ctx, Schema: schema})
	if m.SetSchemaFunc != nil {
		return m.SetSchemaFunc(ctx, schema)
	}
	return m.SetSchemaResult, m.SetSchemaError
}

func (m *MockTableSchema) GetCurrentSchema() *onetable.SchemaDef {
	m.GetCurrentSchemaCalls = append(m.GetCurrentSchemaCalls, GetCurrentSchemaCall{})
	if m.GetCurrentSchemaFunc != nil {
		return m.GetCurrentSchemaFunc()
	}
	return m.GetCurrentSchemaResult
}

func (m *MockTableSchema) GetKeys(ctx context.Context) (map[string]*onetable.IndexDef, error) {
	m.GetKeysCalls = append(m.GetKeysCalls, GetKeysCall{Ctx: ctx})
	if m.GetKeysFunc != nil {
		return m.GetKeysFunc(ctx)
	}
	return m.GetKeysResult, m.GetKeysError
}

func (m *MockTableSchema) GetModel(name string) (*onetable.Model, error) {
	m.GetModelCalls = append(m.GetModelCalls, GetModelCall{Name: name})
	if m.GetModelFunc != nil {
		return m.GetModelFunc(name)
	}
	return m.GetModelResult, m.GetModelError
}

func (m *MockTableSchema) AddModel(name string, fields onetable.FieldMap) {
	m.AddModelCalls = append(m.AddModelCalls, AddModelCall{Name: name, Fields: fields})
	if m.AddModelFunc != nil {
		m.AddModelFunc(name, fields)
	}
}

func (m *MockTableSchema) RemoveModel(name string) error {
	m.RemoveModelCalls = append(m.RemoveModelCalls, RemoveModelCall{Name: name})
	if m.RemoveModelFunc != nil {
		return m.RemoveModelFunc(name)
	}
	return m.RemoveModelError
}

func (m *MockTableSchema) ListModels() []string {
	m.ListModelsCalls = append(m.ListModelsCalls, ListModelsCall{})
	if m.ListModelsFunc != nil {
		return m.ListModelsFunc()
	}
	return m.ListModelsResult
}

func (m *MockTableSchema) SetClient(client onetable.DynamoClient) {
	m.SetClientCalls = append(m.SetClientCalls, SetClientCall{Client: client})
	if m.SetClientFunc != nil {
		m.SetClientFunc(client)
	}
}

func (m *MockTableSchema) GetLog() onetable.Logger {
	m.GetLogCalls = append(m.GetLogCalls, GetLogCall{})
	if m.GetLogFunc != nil {
		return m.GetLogFunc()
	}
	return m.GetLogResult
}

func (m *MockTableSchema) SetLog(logger onetable.Logger) {
	m.SetLogCalls = append(m.SetLogCalls, SetLogCall{Logger: logger})
	if m.SetLogFunc != nil {
		m.SetLogFunc(logger)
	}
}
