// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"

type MockTableContext struct {
	GetContextFunc     func() onetable.Item
	GetContextCalls    []GetContextCall
	GetContextResult   onetable.Item
	SetContextFunc     func(onetable.Item, bool) *onetable.Table
	SetContextCalls    []SetContextCall
	SetContextResult   *onetable.Table
	AddContextFunc     func(onetable.Item) *onetable.Table
	AddContextCalls    []AddContextCall
	AddContextResult   *onetable.Table
	ClearContextFunc   func() *onetable.Table
	ClearContextCalls  []ClearContextCall
	ClearContextResult *onetable.Table
}

type GetContextCall struct{}

type SetContextCall struct {
	Context onetable.Item
	Merge   bool
}

type AddContextCall struct {
	Context onetable.Item
}

type ClearContextCall struct{}

func (m *MockTableContext) GetContext() onetable.Item {
	m.GetContextCalls = append(m.GetContextCalls, GetContextCall{})
	if m.GetContextFunc != nil {
		return m.GetContextFunc()
	}
	return m.GetContextResult
}

func (m *MockTableContext) SetContext(ctx onetable.Item, merge bool) *onetable.Table {
	m.SetContextCalls = append(m.SetContextCalls, SetContextCall{Context: ctx, Merge: merge})
	if m.SetContextFunc != nil {
		return m.SetContextFunc(ctx, merge)
	}
	return m.SetContextResult
}

func (m *MockTableContext) AddContext(ctx onetable.Item) *onetable.Table {
	m.AddContextCalls = append(m.AddContextCalls, AddContextCall{Context: ctx})
	if m.AddContextFunc != nil {
		return m.AddContextFunc(ctx)
	}
	return m.AddContextResult
}

func (m *MockTableContext) ClearContext() *onetable.Table {
	m.ClearContextCalls = append(m.ClearContextCalls, ClearContextCall{})
	if m.ClearContextFunc != nil {
		return m.ClearContextFunc()
	}
	return m.ClearContextResult
}
