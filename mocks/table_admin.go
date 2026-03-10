// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

type MockTableAdmin struct {
	SaveSchemaFunc           func(context.Context, *onetable.SchemaDef) error
	SaveSchemaCalls          []SaveSchemaCall
	SaveSchemaError          error
	ReadSchemaFunc           func(context.Context) (*onetable.SchemaDef, error)
	ReadSchemaCalls          []ReadSchemaCall
	ReadSchemaResult         *onetable.SchemaDef
	ReadSchemaError          error
	ReadSchemasFunc          func(context.Context) ([]*onetable.SchemaDef, error)
	ReadSchemasCalls         []ReadSchemasCall
	ReadSchemasResult        []*onetable.SchemaDef
	ReadSchemasError         error
	RemoveSchemaFunc         func(context.Context, *onetable.SchemaDef) error
	RemoveSchemaCalls        []RemoveSchemaCall
	RemoveSchemaError        error
	CreateTableFunc          func(context.Context) error
	CreateTableCalls         []CreateTableCall
	CreateTableError         error
	DeleteTableFunc          func(context.Context, string) error
	DeleteTableCalls         []DeleteTableCall
	DeleteTableError         error
	DescribeTableFunc        func(context.Context) (onetable.Item, error)
	DescribeTableCalls       []DescribeTableCall
	DescribeTableResult      onetable.Item
	DescribeTableError       error
	ExistsFunc               func(context.Context) (bool, error)
	ExistsCalls              []ExistsCall
	ExistsResult             bool
	ExistsError              error
	ListTablesFunc           func(context.Context) ([]string, error)
	ListTablesCalls          []ListTablesCall
	ListTablesResult         []string
	ListTablesError          error
	GetTableDefinitionFunc   func(*types.ProvisionedThroughput) *onetable.TableDefinition
	GetTableDefinitionCalls  []GetTableDefinitionCall
	GetTableDefinitionResult *onetable.TableDefinition
	UpdateTableFunc          func(context.Context, *onetable.UpdateTableParams) error
	UpdateTableCalls         []UpdateTableCall
	UpdateTableError         error
}

type SaveSchemaCall struct {
	Ctx    context.Context
	Schema *onetable.SchemaDef
}

type ReadSchemaCall struct {
	Ctx context.Context
}

type ReadSchemasCall struct {
	Ctx context.Context
}

type RemoveSchemaCall struct {
	Ctx    context.Context
	Schema *onetable.SchemaDef
}

type CreateTableCall struct {
	Ctx context.Context
}

type DeleteTableCall struct {
	Ctx          context.Context
	Confirmation string
}

type DescribeTableCall struct {
	Ctx context.Context
}

type ExistsCall struct {
	Ctx context.Context
}

type ListTablesCall struct {
	Ctx context.Context
}

type GetTableDefinitionCall struct {
	Provisioned *types.ProvisionedThroughput
}

type UpdateTableCall struct {
	Ctx    context.Context
	Params *onetable.UpdateTableParams
}

func (m *MockTableAdmin) SaveSchema(ctx context.Context, schema *onetable.SchemaDef) error {
	m.SaveSchemaCalls = append(m.SaveSchemaCalls, SaveSchemaCall{Ctx: ctx, Schema: schema})
	if m.SaveSchemaFunc != nil {
		return m.SaveSchemaFunc(ctx, schema)
	}
	return m.SaveSchemaError
}

func (m *MockTableAdmin) ReadSchema(ctx context.Context) (*onetable.SchemaDef, error) {
	m.ReadSchemaCalls = append(m.ReadSchemaCalls, ReadSchemaCall{Ctx: ctx})
	if m.ReadSchemaFunc != nil {
		return m.ReadSchemaFunc(ctx)
	}
	return m.ReadSchemaResult, m.ReadSchemaError
}

func (m *MockTableAdmin) ReadSchemas(ctx context.Context) ([]*onetable.SchemaDef, error) {
	m.ReadSchemasCalls = append(m.ReadSchemasCalls, ReadSchemasCall{Ctx: ctx})
	if m.ReadSchemasFunc != nil {
		return m.ReadSchemasFunc(ctx)
	}
	return m.ReadSchemasResult, m.ReadSchemasError
}

func (m *MockTableAdmin) RemoveSchema(ctx context.Context, schema *onetable.SchemaDef) error {
	m.RemoveSchemaCalls = append(m.RemoveSchemaCalls, RemoveSchemaCall{Ctx: ctx, Schema: schema})
	if m.RemoveSchemaFunc != nil {
		return m.RemoveSchemaFunc(ctx, schema)
	}
	return m.RemoveSchemaError
}

func (m *MockTableAdmin) CreateTable(ctx context.Context) error {
	m.CreateTableCalls = append(m.CreateTableCalls, CreateTableCall{Ctx: ctx})
	if m.CreateTableFunc != nil {
		return m.CreateTableFunc(ctx)
	}
	return m.CreateTableError
}

func (m *MockTableAdmin) DeleteTable(ctx context.Context, confirmation string) error {
	m.DeleteTableCalls = append(m.DeleteTableCalls, DeleteTableCall{Ctx: ctx, Confirmation: confirmation})
	if m.DeleteTableFunc != nil {
		return m.DeleteTableFunc(ctx, confirmation)
	}
	return m.DeleteTableError
}

func (m *MockTableAdmin) DescribeTable(ctx context.Context) (onetable.Item, error) {
	m.DescribeTableCalls = append(m.DescribeTableCalls, DescribeTableCall{Ctx: ctx})
	if m.DescribeTableFunc != nil {
		return m.DescribeTableFunc(ctx)
	}
	return m.DescribeTableResult, m.DescribeTableError
}

func (m *MockTableAdmin) Exists(ctx context.Context) (bool, error) {
	m.ExistsCalls = append(m.ExistsCalls, ExistsCall{Ctx: ctx})
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx)
	}
	return m.ExistsResult, m.ExistsError
}

func (m *MockTableAdmin) ListTables(ctx context.Context) ([]string, error) {
	m.ListTablesCalls = append(m.ListTablesCalls, ListTablesCall{Ctx: ctx})
	if m.ListTablesFunc != nil {
		return m.ListTablesFunc(ctx)
	}
	return m.ListTablesResult, m.ListTablesError
}

func (m *MockTableAdmin) GetTableDefinition(provisioned *types.ProvisionedThroughput) *onetable.TableDefinition {
	m.GetTableDefinitionCalls = append(m.GetTableDefinitionCalls, GetTableDefinitionCall{Provisioned: provisioned})
	if m.GetTableDefinitionFunc != nil {
		return m.GetTableDefinitionFunc(provisioned)
	}
	return m.GetTableDefinitionResult
}

func (m *MockTableAdmin) UpdateTable(ctx context.Context, params *onetable.UpdateTableParams) error {
	m.UpdateTableCalls = append(m.UpdateTableCalls, UpdateTableCall{Ctx: ctx, Params: params})
	if m.UpdateTableFunc != nil {
		return m.UpdateTableFunc(ctx, params)
	}
	return m.UpdateTableError
}
