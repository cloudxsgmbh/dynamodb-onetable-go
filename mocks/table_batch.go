// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

import (
	"context"

	onetable "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

type MockTableBatch struct {
	BatchGetFunc     func(context.Context, map[string]any, *onetable.Params) (any, error)
	BatchGetCalls    []BatchGetCall
	BatchGetResult   any
	BatchGetError    error
	BatchWriteFunc   func(context.Context, map[string]any, *onetable.Params) (bool, error)
	BatchWriteCalls  []BatchWriteCall
	BatchWriteResult bool
	BatchWriteError  error
	TransactFunc     func(context.Context, string, map[string]any, *onetable.Params) (any, error)
	TransactCalls    []TransactCall
	TransactResult   any
	TransactError    error
}

type BatchGetCall struct {
	Ctx    context.Context
	Batch  map[string]any
	Params *onetable.Params
}

type BatchWriteCall struct {
	Ctx    context.Context
	Batch  map[string]any
	Params *onetable.Params
}

type TransactCall struct {
	Ctx    context.Context
	Op     string
	Batch  map[string]any
	Params *onetable.Params
}

func (m *MockTableBatch) BatchGet(ctx context.Context, batch map[string]any, params *onetable.Params) (any, error) {
	m.BatchGetCalls = append(m.BatchGetCalls, BatchGetCall{Ctx: ctx, Batch: batch, Params: params})
	if m.BatchGetFunc != nil {
		return m.BatchGetFunc(ctx, batch, params)
	}
	return m.BatchGetResult, m.BatchGetError
}

func (m *MockTableBatch) BatchWrite(ctx context.Context, batch map[string]any, params *onetable.Params) (bool, error) {
	m.BatchWriteCalls = append(m.BatchWriteCalls, BatchWriteCall{Ctx: ctx, Batch: batch, Params: params})
	if m.BatchWriteFunc != nil {
		return m.BatchWriteFunc(ctx, batch, params)
	}
	return m.BatchWriteResult, m.BatchWriteError
}

func (m *MockTableBatch) Transact(ctx context.Context, op string, transaction map[string]any, params *onetable.Params) (any, error) {
	m.TransactCalls = append(m.TransactCalls, TransactCall{Ctx: ctx, Op: op, Batch: transaction, Params: params})
	if m.TransactFunc != nil {
		return m.TransactFunc(ctx, op, transaction, params)
	}
	return m.TransactResult, m.TransactError
}
