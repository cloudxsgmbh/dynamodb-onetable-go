// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import (
	"context"

	onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

type MockTableItems struct {
	CreateFunc        func(context.Context, string, onetable.Item, *onetable.Params) (onetable.Item, error)
	CreateCalls       []CreateCall
	CreateResult      onetable.Item
	CreateError       error
	FindFunc          func(context.Context, string, onetable.Item, *onetable.Params) (*onetable.Result, error)
	FindCalls         []FindCall
	FindResult        *onetable.Result
	FindError         error
	GetFunc           func(context.Context, string, onetable.Item, *onetable.Params) (onetable.Item, error)
	GetCalls          []GetCall
	GetResult         onetable.Item
	GetError          error
	RemoveFunc        func(context.Context, string, onetable.Item, *onetable.Params) (onetable.Item, error)
	RemoveCalls       []RemoveCall
	RemoveResult      onetable.Item
	RemoveError       error
	ScanFunc          func(context.Context, string, onetable.Item, *onetable.Params) (*onetable.Result, error)
	ScanCalls         []ScanCall
	ScanResult        *onetable.Result
	ScanError         error
	UpdateFunc        func(context.Context, string, onetable.Item, *onetable.Params) (onetable.Item, error)
	UpdateCalls       []UpdateCall
	UpdateResult      onetable.Item
	UpdateError       error
	UpsertFunc        func(context.Context, string, onetable.Item, *onetable.Params) (onetable.Item, error)
	UpsertCalls       []UpsertCall
	UpsertResult      onetable.Item
	UpsertError       error
	GetItemFunc       func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	GetItemCalls      []GetItemCall
	GetItemResult     onetable.Item
	GetItemError      error
	PutItemFunc       func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	PutItemCalls      []PutItemCall
	PutItemResult     onetable.Item
	PutItemError      error
	DeleteItemFunc    func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	DeleteItemCalls   []DeleteItemCall
	DeleteItemResult  onetable.Item
	DeleteItemError   error
	QueryItemsFunc    func(context.Context, onetable.Item, *onetable.Params) (*onetable.Result, error)
	QueryItemsCalls   []QueryItemsCall
	QueryItemsResult  *onetable.Result
	QueryItemsError   error
	ScanItemsFunc     func(context.Context, onetable.Item, *onetable.Params) (*onetable.Result, error)
	ScanItemsCalls    []ScanItemsCall
	ScanItemsResult   *onetable.Result
	ScanItemsError    error
	UpdateItemFunc    func(context.Context, onetable.Item, *onetable.Params) (onetable.Item, error)
	UpdateItemCalls   []UpdateItemCall
	UpdateItemResult  onetable.Item
	UpdateItemError   error
	GroupByTypeFunc   func([]onetable.Item, *onetable.Params) map[string][]onetable.Item
	GroupByTypeCalls  []GroupByTypeCall
	GroupByTypeResult map[string][]onetable.Item
	FetchFunc         func(context.Context, []string, onetable.Item, *onetable.Params) (map[string][]onetable.Item, error)
	FetchCalls        []FetchCall
	FetchResult       map[string][]onetable.Item
	FetchError        error
	UUIDFunc          func() string
	UUIDCalls         []UUIDCall
	UUIDResult        string
	ULIDFunc          func() string
	ULIDCalls         []ULIDCall
	ULIDResult        string
	UIDFunc           func(int) string
	UIDCalls          []UIDCall
	UIDResult         string
}

type CreateCall struct {
	Ctx        context.Context
	ModelName  string
	Properties onetable.Item
	Params     *onetable.Params
}

type FindCall struct {
	Ctx        context.Context
	ModelName  string
	Properties onetable.Item
	Params     *onetable.Params
}

type GetCall struct {
	Ctx        context.Context
	ModelName  string
	Properties onetable.Item
	Params     *onetable.Params
}

type RemoveCall struct {
	Ctx        context.Context
	ModelName  string
	Properties onetable.Item
	Params     *onetable.Params
}

type ScanCall struct {
	Ctx        context.Context
	ModelName  string
	Properties onetable.Item
	Params     *onetable.Params
}

type UpdateCall struct {
	Ctx        context.Context
	ModelName  string
	Properties onetable.Item
	Params     *onetable.Params
}

type UpsertCall struct {
	Ctx        context.Context
	ModelName  string
	Properties onetable.Item
	Params     *onetable.Params
}

type GetItemCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type PutItemCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type DeleteItemCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type QueryItemsCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type ScanItemsCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type UpdateItemCall struct {
	Ctx        context.Context
	Properties onetable.Item
	Params     *onetable.Params
}

type GroupByTypeCall struct {
	Items  []onetable.Item
	Params *onetable.Params
}

type FetchCall struct {
	Ctx        context.Context
	Models     []string
	Properties onetable.Item
	Params     *onetable.Params
}

type UUIDCall struct{}

type ULIDCall struct{}

type UIDCall struct {
	Size int
}

func (m *MockTableItems) Create(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.CreateCalls = append(m.CreateCalls, CreateCall{Ctx: ctx, ModelName: modelName, Properties: properties, Params: params})
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, modelName, properties, params)
	}
	return m.CreateResult, m.CreateError
}

func (m *MockTableItems) Find(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	m.FindCalls = append(m.FindCalls, FindCall{Ctx: ctx, ModelName: modelName, Properties: properties, Params: params})
	if m.FindFunc != nil {
		return m.FindFunc(ctx, modelName, properties, params)
	}
	return m.FindResult, m.FindError
}

func (m *MockTableItems) Get(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.GetCalls = append(m.GetCalls, GetCall{Ctx: ctx, ModelName: modelName, Properties: properties, Params: params})
	if m.GetFunc != nil {
		return m.GetFunc(ctx, modelName, properties, params)
	}
	return m.GetResult, m.GetError
}

func (m *MockTableItems) Remove(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.RemoveCalls = append(m.RemoveCalls, RemoveCall{Ctx: ctx, ModelName: modelName, Properties: properties, Params: params})
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, modelName, properties, params)
	}
	return m.RemoveResult, m.RemoveError
}

func (m *MockTableItems) Scan(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	m.ScanCalls = append(m.ScanCalls, ScanCall{Ctx: ctx, ModelName: modelName, Properties: properties, Params: params})
	if m.ScanFunc != nil {
		return m.ScanFunc(ctx, modelName, properties, params)
	}
	return m.ScanResult, m.ScanError
}

func (m *MockTableItems) Update(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.UpdateCalls = append(m.UpdateCalls, UpdateCall{Ctx: ctx, ModelName: modelName, Properties: properties, Params: params})
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, modelName, properties, params)
	}
	return m.UpdateResult, m.UpdateError
}

func (m *MockTableItems) Upsert(ctx context.Context, modelName string, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.UpsertCalls = append(m.UpsertCalls, UpsertCall{Ctx: ctx, ModelName: modelName, Properties: properties, Params: params})
	if m.UpsertFunc != nil {
		return m.UpsertFunc(ctx, modelName, properties, params)
	}
	return m.UpsertResult, m.UpsertError
}

func (m *MockTableItems) GetItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.GetItemCalls = append(m.GetItemCalls, GetItemCall{Ctx: ctx, Properties: properties, Params: params})
	if m.GetItemFunc != nil {
		return m.GetItemFunc(ctx, properties, params)
	}
	return m.GetItemResult, m.GetItemError
}

func (m *MockTableItems) PutItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.PutItemCalls = append(m.PutItemCalls, PutItemCall{Ctx: ctx, Properties: properties, Params: params})
	if m.PutItemFunc != nil {
		return m.PutItemFunc(ctx, properties, params)
	}
	return m.PutItemResult, m.PutItemError
}

func (m *MockTableItems) DeleteItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.DeleteItemCalls = append(m.DeleteItemCalls, DeleteItemCall{Ctx: ctx, Properties: properties, Params: params})
	if m.DeleteItemFunc != nil {
		return m.DeleteItemFunc(ctx, properties, params)
	}
	return m.DeleteItemResult, m.DeleteItemError
}

func (m *MockTableItems) QueryItems(ctx context.Context, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	m.QueryItemsCalls = append(m.QueryItemsCalls, QueryItemsCall{Ctx: ctx, Properties: properties, Params: params})
	if m.QueryItemsFunc != nil {
		return m.QueryItemsFunc(ctx, properties, params)
	}
	return m.QueryItemsResult, m.QueryItemsError
}

func (m *MockTableItems) ScanItems(ctx context.Context, properties onetable.Item, params *onetable.Params) (*onetable.Result, error) {
	m.ScanItemsCalls = append(m.ScanItemsCalls, ScanItemsCall{Ctx: ctx, Properties: properties, Params: params})
	if m.ScanItemsFunc != nil {
		return m.ScanItemsFunc(ctx, properties, params)
	}
	return m.ScanItemsResult, m.ScanItemsError
}

func (m *MockTableItems) UpdateItem(ctx context.Context, properties onetable.Item, params *onetable.Params) (onetable.Item, error) {
	m.UpdateItemCalls = append(m.UpdateItemCalls, UpdateItemCall{Ctx: ctx, Properties: properties, Params: params})
	if m.UpdateItemFunc != nil {
		return m.UpdateItemFunc(ctx, properties, params)
	}
	return m.UpdateItemResult, m.UpdateItemError
}

func (m *MockTableItems) GroupByType(items []onetable.Item, params *onetable.Params) map[string][]onetable.Item {
	m.GroupByTypeCalls = append(m.GroupByTypeCalls, GroupByTypeCall{Items: items, Params: params})
	if m.GroupByTypeFunc != nil {
		return m.GroupByTypeFunc(items, params)
	}
	return m.GroupByTypeResult
}

func (m *MockTableItems) Fetch(ctx context.Context, models []string, properties onetable.Item, params *onetable.Params) (map[string][]onetable.Item, error) {
	m.FetchCalls = append(m.FetchCalls, FetchCall{Ctx: ctx, Models: models, Properties: properties, Params: params})
	if m.FetchFunc != nil {
		return m.FetchFunc(ctx, models, properties, params)
	}
	return m.FetchResult, m.FetchError
}

func (m *MockTableItems) UUID() string {
	m.UUIDCalls = append(m.UUIDCalls, UUIDCall{})
	if m.UUIDFunc != nil {
		return m.UUIDFunc()
	}
	return m.UUIDResult
}

func (m *MockTableItems) ULID() string {
	m.ULIDCalls = append(m.ULIDCalls, ULIDCall{})
	if m.ULIDFunc != nil {
		return m.ULIDFunc()
	}
	return m.ULIDResult
}

func (m *MockTableItems) UID(size int) string {
	m.UIDCalls = append(m.UIDCalls, UIDCall{Size: size})
	if m.UIDFunc != nil {
		return m.UIDFunc(size)
	}
	return m.UIDResult
}
