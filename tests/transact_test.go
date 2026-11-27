// Ports: test/transact.ts
package tests

import (
	"context"
	"testing"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

var txData = []ot.Item{
	{"name": "Peter Smith", "email": "peter@example.com", "status": "active"},
	{"name": "Patty O'Furniture", "email": "patty@example.com", "status": "active"},
	{"name": "Cu Later", "email": "cu@example.com", "status": "inactive"},
}

func TestTransact_Create(t *testing.T) {
	tbl, _ := makeTable(t, "TransactTable", DefaultSchema, false)
	transaction := map[string]any{}
	var last ot.Item
	for _, d := range txData {
		u, err := tbl.Create(bg(), "User", d, &ot.Params{Transaction: transaction})
		if err != nil {
			t.Fatalf("transact create: %v", err)
		}
		last = u
	}
	if _, err := tbl.Transact(bg(), "write", transaction, &ot.Params{Parse: true, Hidden: falsePtr()}); err != nil {
		t.Fatalf("Transact write: %v", err)
	}
	// returned item from transact is a stub (no pk/sk)
	assertAbsent(t, last, "pk")
	assertPresent(t, last, "id")
}

func TestTransact_Get(t *testing.T) {
	tbl, _ := makeTable(t, "TransactTable", DefaultSchema, false)
	users := make([]ot.Item, 0, len(txData))
	for _, d := range txData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	transaction := map[string]any{}
	for _, u := range users {
		tbl.Get(bg(), "User", ot.Item{"id": u["id"]}, &ot.Params{Transaction: transaction}) //nolint
	}
	result, err := tbl.Transact(bg(), "get", transaction, &ot.Params{Parse: true, Hidden: falsePtr()})
	if err != nil {
		t.Fatalf("Transact get: %v", err)
	}
	items, ok := result.([]ot.Item)
	if !ok {
		t.Fatalf("expected []Item, got %T", result)
	}
	assertLen(t, items, len(txData))
}

func TestTransact_Update(t *testing.T) {
	tbl, _ := makeTable(t, "TransactTable", DefaultSchema, false)
	users := make([]ot.Item, 0, len(txData))
	for _, d := range txData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	transaction := map[string]any{}
	for _, u := range users {
		tbl.Update(bg(), "User", ot.Item{"id": u["id"], "status": "offline"}, //nolint
			&ot.Params{Transaction: transaction})
	}
	if _, err := tbl.Transact(bg(), "write", transaction, nil); err != nil {
		t.Fatalf("Transact update: %v", err)
	}
}

func TestTransact_GroupByType(t *testing.T) {
	tbl, _ := makeTable(t, "TransactTable", DefaultSchema, false)
	users := make([]ot.Item, 0, len(txData))
	for _, d := range txData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	// mark all offline via transaction
	transaction := map[string]any{}
	for _, u := range users {
		tbl.Update(bg(), "User", ot.Item{"id": u["id"], "status": "offline"}, //nolint
			&ot.Params{Transaction: transaction})
	}
	tbl.Transact(bg(), "write", transaction, nil) //nolint

	all, _ := tbl.Scan(bg(), "User", ot.Item{}, &ot.Params{Hidden: truePtr()})
	grouped := tbl.GroupByType(all.Items, nil)
	if len(grouped["User"]) != len(txData) {
		t.Errorf("expected %d Users in group, got %d", len(txData), len(grouped["User"]))
	}
}

func TestTransact_GetWithoutParse(t *testing.T) {
	tbl, _ := makeTable(t, "TransactTable", DefaultSchema, false)
	users := make([]ot.Item, 0, len(txData))
	for _, d := range txData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	transaction := map[string]any{}
	for _, u := range users {
		tbl.Get(bg(), "User", ot.Item{"id": u["id"]}, &ot.Params{Transaction: transaction}) //nolint
	}
	result, err := tbl.Transact(bg(), "get", transaction, &ot.Params{Parse: false, Hidden: truePtr()})
	if err != nil {
		t.Fatalf("Transact get no-parse: %v", err)
	}
	raw, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if raw["Responses"] == nil {
		t.Error("expected Responses")
	}
}

func TestFullMockTransactWriteItemsConditionCheck(t *testing.T) {
	mock := newFullMock()
	tableName := "ConditionCheckTable"
	condition := "status = :expected"
	mock.tables[tableName] = map[string]map[string]types.AttributeValue{
		"user||1": {
			"pk":     &types.AttributeValueMemberS{Value: "user"},
			"sk":     &types.AttributeValueMemberS{Value: "1"},
			"status": &types.AttributeValueMemberS{Value: "active"},
		},
	}

	_, err := mock.TransactWriteItems(context.Background(), &ddb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{{
			ConditionCheck: &types.ConditionCheck{
				TableName:           &tableName,
				Key:                 map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "user"}, "sk": &types.AttributeValueMemberS{Value: "1"}},
				ConditionExpression: &condition,
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":expected": &types.AttributeValueMemberS{Value: "active"},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("TransactWriteItems with passing ConditionCheck: %v", err)
	}
	if got := mock.tables[tableName]["user||1"]["status"].(*types.AttributeValueMemberS).Value; got != "active" {
		t.Fatalf("ConditionCheck modified existing item status: got %q", got)
	}
}

func TestFullMockTransactWriteItemsConditionCheckFailureIsAtomic(t *testing.T) {
	mock := newFullMock()
	tableName := "ConditionCheckTable"
	condition := "status = :expected"
	mock.tables[tableName] = map[string]map[string]types.AttributeValue{
		"user||1": {
			"pk":     &types.AttributeValueMemberS{Value: "user"},
			"sk":     &types.AttributeValueMemberS{Value: "1"},
			"status": &types.AttributeValueMemberS{Value: "inactive"},
		},
	}

	_, err := mock.TransactWriteItems(context.Background(), &ddb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{
				TableName: &tableName,
				Item: map[string]types.AttributeValue{
					"pk":     &types.AttributeValueMemberS{Value: "user"},
					"sk":     &types.AttributeValueMemberS{Value: "2"},
					"status": &types.AttributeValueMemberS{Value: "active"},
				},
			}},
			{ConditionCheck: &types.ConditionCheck{
				TableName:           &tableName,
				Key:                 map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "user"}, "sk": &types.AttributeValueMemberS{Value: "1"}},
				ConditionExpression: &condition,
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":expected": &types.AttributeValueMemberS{Value: "active"},
				},
			}},
		},
	})
	if err == nil {
		t.Fatal("TransactWriteItems with failing ConditionCheck succeeded, want condition failure")
	}
	if got := mock.tables[tableName]["user||1"]["status"].(*types.AttributeValueMemberS).Value; got != "inactive" {
		t.Fatalf("failed transaction modified checked item status: got %q", got)
	}
	if _, exists := mock.tables[tableName]["user||2"]; exists {
		t.Fatal("failed ConditionCheck left the earlier Put applied")
	}
}
