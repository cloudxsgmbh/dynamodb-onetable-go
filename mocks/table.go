// Package mocks provides hand-rolled mocks for external unit tests.
package mocks

// MockTable is a stubbed Table with call tracking.
type MockTable struct {
	Schema  *MockTableSchema
	Context *MockTableContext
	Items   *MockTableItems
	Batch   *MockTableBatch
	Admin   *MockTableAdmin
}

// NewMockTable creates a new MockTable.
func NewMockTable() *MockTable {
	return &MockTable{
		Schema:  &MockTableSchema{},
		Context: &MockTableContext{},
		Items:   &MockTableItems{},
		Batch:   &MockTableBatch{},
		Admin:   &MockTableAdmin{},
	}
}
