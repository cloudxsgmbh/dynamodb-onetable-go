// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import (
	"context"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// MockDynamoClient is a stubbed DynamoDB client with call tracking.
type MockDynamoClient struct {
	GetItemFunc              func(context.Context, *ddb.GetItemInput, ...func(*ddb.Options)) (*ddb.GetItemOutput, error)
	GetItemCalls             []DynamoGetItemCall
	GetItemResult            *ddb.GetItemOutput
	GetItemError             error
	PutItemFunc              func(context.Context, *ddb.PutItemInput, ...func(*ddb.Options)) (*ddb.PutItemOutput, error)
	PutItemCalls             []DynamoPutItemCall
	PutItemResult            *ddb.PutItemOutput
	PutItemError             error
	DeleteItemFunc           func(context.Context, *ddb.DeleteItemInput, ...func(*ddb.Options)) (*ddb.DeleteItemOutput, error)
	DeleteItemCalls          []DynamoDeleteItemCall
	DeleteItemResult         *ddb.DeleteItemOutput
	DeleteItemError          error
	UpdateItemFunc           func(context.Context, *ddb.UpdateItemInput, ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error)
	UpdateItemCalls          []DynamoUpdateItemCall
	UpdateItemResult         *ddb.UpdateItemOutput
	UpdateItemError          error
	QueryFunc                func(context.Context, *ddb.QueryInput, ...func(*ddb.Options)) (*ddb.QueryOutput, error)
	QueryCalls               []DynamoQueryCall
	QueryResult              *ddb.QueryOutput
	QueryError               error
	ScanFunc                 func(context.Context, *ddb.ScanInput, ...func(*ddb.Options)) (*ddb.ScanOutput, error)
	ScanCalls                []DynamoScanCall
	ScanResult               *ddb.ScanOutput
	ScanError                error
	BatchGetItemFunc         func(context.Context, *ddb.BatchGetItemInput, ...func(*ddb.Options)) (*ddb.BatchGetItemOutput, error)
	BatchGetItemCalls        []DynamoBatchGetItemCall
	BatchGetItemResult       *ddb.BatchGetItemOutput
	BatchGetItemError        error
	BatchWriteItemFunc       func(context.Context, *ddb.BatchWriteItemInput, ...func(*ddb.Options)) (*ddb.BatchWriteItemOutput, error)
	BatchWriteItemCalls      []DynamoBatchWriteItemCall
	BatchWriteItemResult     *ddb.BatchWriteItemOutput
	BatchWriteItemError      error
	TransactGetItemsFunc     func(context.Context, *ddb.TransactGetItemsInput, ...func(*ddb.Options)) (*ddb.TransactGetItemsOutput, error)
	TransactGetItemsCalls    []DynamoTransactGetItemsCall
	TransactGetItemsResult   *ddb.TransactGetItemsOutput
	TransactGetItemsError    error
	TransactWriteItemsFunc   func(context.Context, *ddb.TransactWriteItemsInput, ...func(*ddb.Options)) (*ddb.TransactWriteItemsOutput, error)
	TransactWriteItemsCalls  []DynamoTransactWriteItemsCall
	TransactWriteItemsResult *ddb.TransactWriteItemsOutput
	TransactWriteItemsError  error
	CreateTableFunc          func(context.Context, *ddb.CreateTableInput, ...func(*ddb.Options)) (*ddb.CreateTableOutput, error)
	CreateTableCalls         []DynamoCreateTableCall
	CreateTableResult        *ddb.CreateTableOutput
	CreateTableError         error
	DeleteTableFunc          func(context.Context, *ddb.DeleteTableInput, ...func(*ddb.Options)) (*ddb.DeleteTableOutput, error)
	DeleteTableCalls         []DynamoDeleteTableCall
	DeleteTableResult        *ddb.DeleteTableOutput
	DeleteTableError         error
	UpdateTableFunc          func(context.Context, *ddb.UpdateTableInput, ...func(*ddb.Options)) (*ddb.UpdateTableOutput, error)
	UpdateTableCalls         []DynamoUpdateTableCall
	UpdateTableResult        *ddb.UpdateTableOutput
	UpdateTableError         error
	DescribeTableFunc        func(context.Context, *ddb.DescribeTableInput, ...func(*ddb.Options)) (*ddb.DescribeTableOutput, error)
	DescribeTableCalls       []DynamoDescribeTableCall
	DescribeTableResult      *ddb.DescribeTableOutput
	DescribeTableError       error
	ListTablesFunc           func(context.Context, *ddb.ListTablesInput, ...func(*ddb.Options)) (*ddb.ListTablesOutput, error)
	ListTablesCalls          []DynamoListTablesCall
	ListTablesResult         *ddb.ListTablesOutput
	ListTablesError          error
	UpdateTimeToLiveFunc     func(context.Context, *ddb.UpdateTimeToLiveInput, ...func(*ddb.Options)) (*ddb.UpdateTimeToLiveOutput, error)
	UpdateTimeToLiveCalls    []DynamoUpdateTimeToLiveCall
	UpdateTimeToLiveResult   *ddb.UpdateTimeToLiveOutput
	UpdateTimeToLiveError    error
}

// NewMockDynamoClient creates a new MockDynamoClient.
func NewMockDynamoClient() *MockDynamoClient {
	return &MockDynamoClient{}
}

type DynamoGetItemCall struct {
	Ctx    context.Context
	Params *ddb.GetItemInput
	OptFns []func(*ddb.Options)
}

type DynamoPutItemCall struct {
	Ctx    context.Context
	Params *ddb.PutItemInput
	OptFns []func(*ddb.Options)
}

type DynamoDeleteItemCall struct {
	Ctx    context.Context
	Params *ddb.DeleteItemInput
	OptFns []func(*ddb.Options)
}

type DynamoUpdateItemCall struct {
	Ctx    context.Context
	Params *ddb.UpdateItemInput
	OptFns []func(*ddb.Options)
}

type DynamoQueryCall struct {
	Ctx    context.Context
	Params *ddb.QueryInput
	OptFns []func(*ddb.Options)
}

type DynamoScanCall struct {
	Ctx    context.Context
	Params *ddb.ScanInput
	OptFns []func(*ddb.Options)
}

type DynamoBatchGetItemCall struct {
	Ctx    context.Context
	Params *ddb.BatchGetItemInput
	OptFns []func(*ddb.Options)
}

type DynamoBatchWriteItemCall struct {
	Ctx    context.Context
	Params *ddb.BatchWriteItemInput
	OptFns []func(*ddb.Options)
}

type DynamoTransactGetItemsCall struct {
	Ctx    context.Context
	Params *ddb.TransactGetItemsInput
	OptFns []func(*ddb.Options)
}

type DynamoTransactWriteItemsCall struct {
	Ctx    context.Context
	Params *ddb.TransactWriteItemsInput
	OptFns []func(*ddb.Options)
}

type DynamoCreateTableCall struct {
	Ctx    context.Context
	Params *ddb.CreateTableInput
	OptFns []func(*ddb.Options)
}

type DynamoDeleteTableCall struct {
	Ctx    context.Context
	Params *ddb.DeleteTableInput
	OptFns []func(*ddb.Options)
}

type DynamoUpdateTableCall struct {
	Ctx    context.Context
	Params *ddb.UpdateTableInput
	OptFns []func(*ddb.Options)
}

type DynamoDescribeTableCall struct {
	Ctx    context.Context
	Params *ddb.DescribeTableInput
	OptFns []func(*ddb.Options)
}

type DynamoListTablesCall struct {
	Ctx    context.Context
	Params *ddb.ListTablesInput
	OptFns []func(*ddb.Options)
}

type DynamoUpdateTimeToLiveCall struct {
	Ctx    context.Context
	Params *ddb.UpdateTimeToLiveInput
	OptFns []func(*ddb.Options)
}

func (m *MockDynamoClient) GetItem(ctx context.Context, params *ddb.GetItemInput, optFns ...func(*ddb.Options)) (*ddb.GetItemOutput, error) {
	m.GetItemCalls = append(m.GetItemCalls, DynamoGetItemCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.GetItemFunc != nil {
		return m.GetItemFunc(ctx, params, optFns...)
	}
	return m.GetItemResult, m.GetItemError
}

func (m *MockDynamoClient) PutItem(ctx context.Context, params *ddb.PutItemInput, optFns ...func(*ddb.Options)) (*ddb.PutItemOutput, error) {
	m.PutItemCalls = append(m.PutItemCalls, DynamoPutItemCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.PutItemFunc != nil {
		return m.PutItemFunc(ctx, params, optFns...)
	}
	return m.PutItemResult, m.PutItemError
}

func (m *MockDynamoClient) DeleteItem(ctx context.Context, params *ddb.DeleteItemInput, optFns ...func(*ddb.Options)) (*ddb.DeleteItemOutput, error) {
	m.DeleteItemCalls = append(m.DeleteItemCalls, DynamoDeleteItemCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.DeleteItemFunc != nil {
		return m.DeleteItemFunc(ctx, params, optFns...)
	}
	return m.DeleteItemResult, m.DeleteItemError
}

func (m *MockDynamoClient) UpdateItem(ctx context.Context, params *ddb.UpdateItemInput, optFns ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error) {
	m.UpdateItemCalls = append(m.UpdateItemCalls, DynamoUpdateItemCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.UpdateItemFunc != nil {
		return m.UpdateItemFunc(ctx, params, optFns...)
	}
	return m.UpdateItemResult, m.UpdateItemError
}

func (m *MockDynamoClient) Query(ctx context.Context, params *ddb.QueryInput, optFns ...func(*ddb.Options)) (*ddb.QueryOutput, error) {
	m.QueryCalls = append(m.QueryCalls, DynamoQueryCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, params, optFns...)
	}
	return m.QueryResult, m.QueryError
}

func (m *MockDynamoClient) Scan(ctx context.Context, params *ddb.ScanInput, optFns ...func(*ddb.Options)) (*ddb.ScanOutput, error) {
	m.ScanCalls = append(m.ScanCalls, DynamoScanCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.ScanFunc != nil {
		return m.ScanFunc(ctx, params, optFns...)
	}
	return m.ScanResult, m.ScanError
}

func (m *MockDynamoClient) BatchGetItem(ctx context.Context, params *ddb.BatchGetItemInput, optFns ...func(*ddb.Options)) (*ddb.BatchGetItemOutput, error) {
	m.BatchGetItemCalls = append(m.BatchGetItemCalls, DynamoBatchGetItemCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.BatchGetItemFunc != nil {
		return m.BatchGetItemFunc(ctx, params, optFns...)
	}
	return m.BatchGetItemResult, m.BatchGetItemError
}

func (m *MockDynamoClient) BatchWriteItem(ctx context.Context, params *ddb.BatchWriteItemInput, optFns ...func(*ddb.Options)) (*ddb.BatchWriteItemOutput, error) {
	m.BatchWriteItemCalls = append(m.BatchWriteItemCalls, DynamoBatchWriteItemCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.BatchWriteItemFunc != nil {
		return m.BatchWriteItemFunc(ctx, params, optFns...)
	}
	return m.BatchWriteItemResult, m.BatchWriteItemError
}

func (m *MockDynamoClient) TransactGetItems(ctx context.Context, params *ddb.TransactGetItemsInput, optFns ...func(*ddb.Options)) (*ddb.TransactGetItemsOutput, error) {
	m.TransactGetItemsCalls = append(m.TransactGetItemsCalls, DynamoTransactGetItemsCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.TransactGetItemsFunc != nil {
		return m.TransactGetItemsFunc(ctx, params, optFns...)
	}
	return m.TransactGetItemsResult, m.TransactGetItemsError
}

func (m *MockDynamoClient) TransactWriteItems(ctx context.Context, params *ddb.TransactWriteItemsInput, optFns ...func(*ddb.Options)) (*ddb.TransactWriteItemsOutput, error) {
	m.TransactWriteItemsCalls = append(m.TransactWriteItemsCalls, DynamoTransactWriteItemsCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.TransactWriteItemsFunc != nil {
		return m.TransactWriteItemsFunc(ctx, params, optFns...)
	}
	return m.TransactWriteItemsResult, m.TransactWriteItemsError
}

func (m *MockDynamoClient) CreateTable(ctx context.Context, params *ddb.CreateTableInput, optFns ...func(*ddb.Options)) (*ddb.CreateTableOutput, error) {
	m.CreateTableCalls = append(m.CreateTableCalls, DynamoCreateTableCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.CreateTableFunc != nil {
		return m.CreateTableFunc(ctx, params, optFns...)
	}
	return m.CreateTableResult, m.CreateTableError
}

func (m *MockDynamoClient) DeleteTable(ctx context.Context, params *ddb.DeleteTableInput, optFns ...func(*ddb.Options)) (*ddb.DeleteTableOutput, error) {
	m.DeleteTableCalls = append(m.DeleteTableCalls, DynamoDeleteTableCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.DeleteTableFunc != nil {
		return m.DeleteTableFunc(ctx, params, optFns...)
	}
	return m.DeleteTableResult, m.DeleteTableError
}

func (m *MockDynamoClient) UpdateTable(ctx context.Context, params *ddb.UpdateTableInput, optFns ...func(*ddb.Options)) (*ddb.UpdateTableOutput, error) {
	m.UpdateTableCalls = append(m.UpdateTableCalls, DynamoUpdateTableCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.UpdateTableFunc != nil {
		return m.UpdateTableFunc(ctx, params, optFns...)
	}
	return m.UpdateTableResult, m.UpdateTableError
}

func (m *MockDynamoClient) DescribeTable(ctx context.Context, params *ddb.DescribeTableInput, optFns ...func(*ddb.Options)) (*ddb.DescribeTableOutput, error) {
	m.DescribeTableCalls = append(m.DescribeTableCalls, DynamoDescribeTableCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.DescribeTableFunc != nil {
		return m.DescribeTableFunc(ctx, params, optFns...)
	}
	return m.DescribeTableResult, m.DescribeTableError
}

func (m *MockDynamoClient) ListTables(ctx context.Context, params *ddb.ListTablesInput, optFns ...func(*ddb.Options)) (*ddb.ListTablesOutput, error) {
	m.ListTablesCalls = append(m.ListTablesCalls, DynamoListTablesCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.ListTablesFunc != nil {
		return m.ListTablesFunc(ctx, params, optFns...)
	}
	return m.ListTablesResult, m.ListTablesError
}

func (m *MockDynamoClient) UpdateTimeToLive(ctx context.Context, params *ddb.UpdateTimeToLiveInput, optFns ...func(*ddb.Options)) (*ddb.UpdateTimeToLiveOutput, error) {
	m.UpdateTimeToLiveCalls = append(m.UpdateTimeToLiveCalls, DynamoUpdateTimeToLiveCall{Ctx: ctx, Params: params, OptFns: optFns})
	if m.UpdateTimeToLiveFunc != nil {
		return m.UpdateTimeToLiveFunc(ctx, params, optFns...)
	}
	return m.UpdateTimeToLiveResult, m.UpdateTimeToLiveError
}
