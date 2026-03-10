// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import (
	"context"

	onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

// MockModel is a stubbed Model with call tracking.
type MockModel struct {
	CreateFunc   func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	CreateCalls  []ModelCreateCall
	CreateResult onetable.Item
	CreateError  error
	GetFunc      func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	GetCalls     []ModelGetCall
	GetResult    onetable.Item
	GetError     error
	FindFunc     func(context.Context, onetable.Item, *onetable.Params) (*onetable.Result, error)
	FindCalls    []ModelFindCall
	FindResult   *onetable.Result
	FindError    error
	ScanFunc     func(context.Context, onetable.Item, *onetable.Params) (*onetable.Result, error)
	ScanCalls    []ModelScanCall
	ScanResult   *onetable.Result
	ScanError    error
	UpdateFunc   func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	UpdateCalls  []ModelUpdateCall
	UpdateResult onetable.Item
	UpdateError  error
	UpsertFunc   func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	UpsertCalls  []ModelUpsertCall
	UpsertResult onetable.Item
	UpsertError  error
	RemoveFunc   func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	RemoveCalls  []ModelRemoveCall
	RemoveResult onetable.Item
	RemoveError  error
	InitFunc     func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	InitCalls    []ModelInitCall
	InitResult   onetable.Item
	InitError    error
}

// NewMockModel creates a new MockModel.
func NewMockModel() *MockModel {
	return &MockModel{}
}

type ModelCreateCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ModelGetCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ModelFindCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ModelScanCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ModelUpdateCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ModelUpsertCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ModelRemoveCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ModelInitCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

func (m *MockModel) Create(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.CreateCalls = append(m.CreateCalls, ModelCreateCall{Ctx: ctx, Properties: properties, Params: params})
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, properties, params)
	}
	return m.CreateResult, m.CreateError
}

func (m *MockModel) Get(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.GetCalls = append(m.GetCalls, ModelGetCall{Ctx: ctx, Properties: properties, Params: params})
	if m.GetFunc != nil {
		return m.GetFunc(ctx, properties, params)
	}
	return m.GetResult, m.GetError
}

func (m *MockModel) Find(ctx context.Context, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	m.FindCalls = append(m.FindCalls, ModelFindCall{Ctx: ctx, Properties: properties, Params: params})
	if m.FindFunc != nil {
		return m.FindFunc(ctx, properties, params)
	}
	return m.FindResult, m.FindError
}

func (m *MockModel) Scan(ctx context.Context, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	m.ScanCalls = append(m.ScanCalls, ModelScanCall{Ctx: ctx, Properties: properties, Params: params})
	if m.ScanFunc != nil {
		return m.ScanFunc(ctx, properties, params)
	}
	return m.ScanResult, m.ScanError
}

func (m *MockModel) Update(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.UpdateCalls = append(m.UpdateCalls, ModelUpdateCall{Ctx: ctx, Properties: properties, Params: params})
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, properties, params)
	}
	return m.UpdateResult, m.UpdateError
}

func (m *MockModel) Upsert(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.UpsertCalls = append(m.UpsertCalls, ModelUpsertCall{Ctx: ctx, Properties: properties, Params: params})
	if m.UpsertFunc != nil {
		return m.UpsertFunc(ctx, properties, params)
	}
	return m.UpsertResult, m.UpsertError
}

func (m *MockModel) Remove(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.RemoveCalls = append(m.RemoveCalls, ModelRemoveCall{Ctx: ctx, Properties: properties, Params: params})
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, properties, params)
	}
	return m.RemoveResult, m.RemoveError
}

func (m *MockModel) Init(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.InitCalls = append(m.InitCalls, ModelInitCall{Ctx: ctx, Properties: properties, Params: params})
	if m.InitFunc != nil {
		return m.InitFunc(ctx, properties, params)
	}
	return m.InitResult, m.InitError
}
