package onetable

import (
	"context"
	"fmt"
	"maps"
	"slices"
)

// Transaction accumulates schema-prepared write operations for one atomic
// DynamoDB transaction. Use Params to attach it to model operations and Write
// to execute all accumulated operations through a table's DynamoDB client.
// It is not safe for concurrent use.
type Transaction struct {
	commands map[string]any
}

// NewTransaction creates an empty write transaction.
func NewTransaction() *Transaction {
	return &Transaction{commands: map[string]any{}}
}

// Params returns a copy of params attached to this transaction's shared
// accumulator. Existing modifiers are preserved; Set and Remove are cloned
// because model preparation may mutate them. A nil params pointer is replaced
// with an empty Params value. Use fresh params when modifiers differ between
// operations.
func (tx *Transaction) Params(params *Params) (*Params, error) {
	if tx == nil {
		return nil, NewArgError("Cannot attach params to a nil transaction")
	}
	if tx.commands == nil {
		tx.commands = map[string]any{}
	}
	if params == nil {
		params = &Params{}
	} else {
		copy := *params
		copy.Set = maps.Clone(params.Set)
		copy.Remove = slices.Clone(params.Remove)
		params = &copy
	}
	params.Transaction = tx.commands
	return params, nil
}

// Check adds a schema-aware standalone condition check to this transaction.
func (tx *Transaction) Check(ctx context.Context, model *Model, properties Item, params *Params) (Item, error) {
	if tx == nil {
		return nil, NewArgError("Cannot add a check to a nil transaction")
	}
	if model == nil {
		return nil, NewArgError("Cannot add a check without a model")
	}
	params, err := tx.Params(params)
	if err != nil {
		return nil, err
	}
	return model.Check(ctx, properties, params)
}

// Write executes this transaction atomically using table's DynamoDB client.
// Model operations retain their own table names, so they may target other
// tables accessible to the same client. Write does not consume or clear the
// accumulator; calling it again resubmits the same operations.
func (tx *Transaction) Write(ctx context.Context, table *Table, params *Params) (any, error) {
	if tx == nil {
		return nil, NewArgError("Cannot write a nil transaction")
	}
	if table == nil {
		return nil, NewArgError("Cannot write a transaction without a table")
	}
	items, ok := tx.commands["TransactItems"].([]any)
	if !ok || len(items) == 0 {
		return nil, NewArgError("Cannot write an empty transaction")
	}
	for i, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok || len(entry) != 1 {
			return nil, NewArgError(fmt.Sprintf("Invalid transaction operation at index %d", i))
		}
		var operation string
		for key := range entry {
			operation = key
		}
		if operation != "Put" && operation != "Update" && operation != "Delete" && operation != "ConditionCheck" {
			return nil, NewArgError(fmt.Sprintf("Unsupported transaction operation %q at index %d", operation, i))
		}
		command, ok := entry[operation].(map[string]any)
		if !ok {
			return nil, NewArgError(fmt.Sprintf("Invalid transaction command at index %d", i))
		}
		requiredFields := map[string][]string{
			"Put":            {"TableName", "Item"},
			"Update":         {"TableName", "Key", "UpdateExpression"},
			"Delete":         {"TableName", "Key"},
			"ConditionCheck": {"TableName", "Key", "ConditionExpression"},
		}[operation]
		for _, field := range requiredFields {
			value, ok := command[field]
			if !ok || value == nil {
				return nil, NewArgError(fmt.Sprintf("Transaction operation %q at index %d is missing %q", operation, i, field))
			}
			if field == "TableName" {
				name, ok := value.(string)
				if !ok || name == "" {
					return nil, NewArgError(fmt.Sprintf("Transaction operation %q at index %d has an invalid table name", operation, i))
				}
			}
			if field == "UpdateExpression" || field == "ConditionExpression" {
				expression, ok := value.(string)
				if !ok || expression == "" {
					return nil, NewArgError(fmt.Sprintf("Transaction operation %q at index %d has an empty %s", operation, i, field))
				}
			}
		}
	}
	return table.Transact(ctx, "write", tx.commands, params)
}
