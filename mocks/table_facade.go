// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func (m *MockTable) SetSchema(ctx context.Context, schema *onetable.SchemaDef) (map[string]*onetable.IndexDef, error) {
	return m.Schema.SetSchema(ctx, schema)
}

func (m *MockTable) GetCurrentSchema() *onetable.SchemaDef {
	return m.Schema.GetCurrentSchema()
}

func (m *MockTable) GetKeys(ctx context.Context) (map[string]*onetable.IndexDef, error) {
	return m.Schema.GetKeys(ctx)
}

func (m *MockTable) GetModel(name string) (*onetable.Model, error) {
	return m.Schema.GetModel(name)
}

func (m *MockTable) AddModel(name string, fields onetable.FieldMap) {
	m.Schema.AddModel(name, fields)
}

func (m *MockTable) RemoveModel(name string) error {
	return m.Schema.RemoveModel(name)
}

func (m *MockTable) ListModels() []string {
	return m.Schema.ListModels()
}

func (m *MockTable) SetClient(client onetable.DynamoClient) {
	m.Schema.SetClient(client)
}

func (m *MockTable) GetLog() onetable.Logger {
	return m.Schema.GetLog()
}

func (m *MockTable) SetLog(logger onetable.Logger) {
	m.Schema.SetLog(logger)
}

func (m *MockTable) SaveSchema(ctx context.Context, schema *onetable.SchemaDef) error {
	return m.Admin.SaveSchema(ctx, schema)
}

func (m *MockTable) ReadSchema(ctx context.Context) (*onetable.SchemaDef, error) {
	return m.Admin.ReadSchema(ctx)
}

func (m *MockTable) ReadSchemas(ctx context.Context) ([]*onetable.SchemaDef, error) {
	return m.Admin.ReadSchemas(ctx)
}

func (m *MockTable) RemoveSchema(ctx context.Context, schema *onetable.SchemaDef) error {
	return m.Admin.RemoveSchema(ctx, schema)
}

func (m *MockTable) GetContext() onetable.Item {
	return m.Context.GetContext()
}

func (m *MockTable) SetContext(ctx onetable.Item, merge bool) *onetable.Table {
	return m.Context.SetContext(ctx, merge)
}

func (m *MockTable) AddContext(ctx onetable.Item) *onetable.Table {
	return m.Context.AddContext(ctx)
}

func (m *MockTable) ClearContext() *onetable.Table {
	return m.Context.ClearContext()
}

func (m *MockTable) Create(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.Create(ctx, modelName, properties, params)
}

func (m *MockTable) Find(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	return m.Items.Find(ctx, modelName, properties, params)
}

func (m *MockTable) Get(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.Get(ctx, modelName, properties, params)
}

func (m *MockTable) Remove(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.Remove(ctx, modelName, properties, params)
}

func (m *MockTable) Scan(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	return m.Items.Scan(ctx, modelName, properties, params)
}

func (m *MockTable) Update(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.Update(ctx, modelName, properties, params)
}

func (m *MockTable) Upsert(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.Upsert(ctx, modelName, properties, params)
}

func (m *MockTable) GetItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.GetItem(ctx, properties, params)
}

func (m *MockTable) PutItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.PutItem(ctx, properties, params)
}

func (m *MockTable) DeleteItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.DeleteItem(ctx, properties, params)
}

func (m *MockTable) QueryItems(ctx context.Context, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	return m.Items.QueryItems(ctx, properties, params)
}

func (m *MockTable) ScanItems(ctx context.Context, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	return m.Items.ScanItems(ctx, properties, params)
}

func (m *MockTable) UpdateItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	return m.Items.UpdateItem(ctx, properties, params)
}

func (m *MockTable) BatchGet(ctx context.Context, batch map[string]any, params *onetable.Params) (any, error) {
	return m.Batch.BatchGet(ctx, batch, params)
}

func (m *MockTable) BatchWrite(ctx context.Context, batch map[string]any, params *onetable.Params) (bool, error) {
	return m.Batch.BatchWrite(ctx, batch, params)
}

func (m *MockTable) Transact(ctx context.Context, op string, transaction map[string]any, params *onetable.Params) (any, error) {
	return m.Batch.Transact(ctx, op, transaction, params)
}

func (m *MockTable) GroupByType(items []onetable.Item, params *onetable.Params) map[string][]onetable.Item {
	return m.Items.GroupByType(items, params)
}

func (m *MockTable) Fetch(ctx context.Context, models []string, properties onetable.Item, params *onetable.Params) (map[string][]onetable.Item, error) {
	return m.Items.Fetch(ctx, models, properties, params)
}

func (m *MockTable) CreateTable(ctx context.Context) error {
	return m.Admin.CreateTable(ctx)
}

func (m *MockTable) DeleteTable(ctx context.Context, confirmation string) error {
	return m.Admin.DeleteTable(ctx, confirmation)
}

func (m *MockTable) DescribeTable(ctx context.Context) (onetable.Item, error) {
	return m.Admin.DescribeTable(ctx)
}

func (m *MockTable) Exists(ctx context.Context) (bool, error) {
	return m.Admin.Exists(ctx)
}

func (m *MockTable) ListTables(ctx context.Context) ([]string, error) {
	return m.Admin.ListTables(ctx)
}

func (m *MockTable) GetTableDefinition(provisioned *types.ProvisionedThroughput) *onetable.TableDefinition {
	return m.Admin.GetTableDefinition(provisioned)
}

func (m *MockTable) UpdateTable(ctx context.Context, params *onetable.UpdateTableParams) error {
	return m.Admin.UpdateTable(ctx, params)
}

func (m *MockTable) UUID() string {
	return m.Items.UUID()
}

func (m *MockTable) ULID() string {
	return m.Items.ULID()
}

func (m *MockTable) UID(size int) string {
	return m.Items.UID(size)
}
