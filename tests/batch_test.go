// Ports: test/batch.ts
package tests

import (
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

var batchData = []ot.Item{
	{"name": "Peter Smith", "email": "peter@example.com", "status": "active"},
	{"name": "Patty O'Furniture", "email": "patty@example.com", "status": "active"},
	{"name": "Cu Later", "email": "cu@example.com", "status": "inactive"},
}

func TestBatch_PutWrite(t *testing.T) {
	tbl, mock := makeTable(t, "BatchTable", DefaultSchema, false)
	batch := map[string]any{}
	for _, d := range batchData {
		if _, err := tbl.Create(bg(), "User", d, &ot.Params{Batch: batch}); err != nil {
			t.Fatalf("batch create: %v", err)
		}
	}
	if _, err := tbl.BatchWrite(bg(), batch, nil); err != nil {
		t.Fatalf("BatchWrite: %v", err)
	}
	if mock.count("BatchTable") != len(batchData) {
		t.Errorf("expected %d items, got %d", len(batchData), mock.count("BatchTable"))
	}
}

func TestBatch_Get(t *testing.T) {
	tbl, _ := makeTable(t, "BatchTable", DefaultSchema, false)
	var users []ot.Item
	for _, d := range batchData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	batch := map[string]any{}
	for _, u := range users {
		tbl.Get(bg(), "User", ot.Item{"id": u["id"]}, &ot.Params{Batch: batch}) //nolint
	}
	result, err := tbl.BatchGet(bg(), batch, &ot.Params{Parse: true, Hidden: boolPtr(false), Consistent: true})
	if err != nil {
		t.Fatalf("BatchGet: %v", err)
	}
	items, _ := result.([]ot.Item)
	assertLen(t, items, len(batchData))
	for _, item := range items {
		found := false
		for _, d := range batchData {
			if item["name"] == d["name"] {
				found = true
			}
		}
		if !found {
			t.Errorf("unexpected item in batch result: %v", item["name"])
		}
	}
}

func TestBatch_PutDeleteCombined(t *testing.T) {
	tbl, _ := makeTable(t, "BatchTable", DefaultSchema, false)
	var users []ot.Item
	for _, d := range batchData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	batch := map[string]any{}
	for _, u := range users {
		tbl.Remove(bg(), "User", ot.Item{"id": u["id"]}, &ot.Params{Batch: batch}) //nolint
	}
	// add one back
	tbl.Create(bg(), "User", batchData[0], &ot.Params{Batch: batch, Exists: nil}) //nolint
	if _, err := tbl.BatchWrite(bg(), batch, nil); err != nil {
		t.Fatalf("BatchWrite combined: %v", err)
	}
}

func TestBatch_GetWithoutParse(t *testing.T) {
	tbl, _ := makeTable(t, "BatchTable", DefaultSchema, false)
	users := make([]ot.Item, 0)
	for _, d := range batchData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	batch := map[string]any{}
	for _, u := range users {
		tbl.Get(bg(), "User", ot.Item{"id": u["id"]}, &ot.Params{Batch: batch}) //nolint
	}
	result, err := tbl.BatchGet(bg(), batch, &ot.Params{Hidden: boolPtr(false)})
	if err != nil {
		t.Fatalf("BatchGet no-parse: %v", err)
	}
	resp, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", result)
	}
	if resp["Responses"] == nil {
		t.Error("expected Responses key")
	}
}

func TestBatch_WithFields(t *testing.T) {
	tbl, _ := makeTable(t, "BatchTable", DefaultSchema, false)
	var users []ot.Item
	for _, d := range batchData {
		u, _ := tbl.Create(bg(), "User", d, nil)
		users = append(users, u)
	}

	batch := map[string]any{}
	for _, u := range users {
		tbl.Get(bg(), "User", ot.Item{"id": u["id"]}, &ot.Params{Batch: batch}) //nolint
	}
	result, err := tbl.BatchGet(bg(), batch, &ot.Params{Parse: true, Fields: []string{"email"}})
	if err != nil {
		t.Fatalf("BatchGet fields: %v", err)
	}
	items, _ := result.([]ot.Item)
	for _, item := range items {
		assertPresent(t, item, "email")
	}
}

func TestBatch_EmptyBatch(t *testing.T) {
	tbl, _ := makeTable(t, "BatchTable", DefaultSchema, false)
	result, err := tbl.BatchGet(bg(), map[string]any{}, nil)
	if err != nil {
		t.Fatalf("empty BatchGet: %v", err)
	}
	items, _ := result.([]ot.Item)
	assertLen(t, items, 0)

	ok, err := tbl.BatchWrite(bg(), map[string]any{}, nil)
	if err != nil {
		t.Fatalf("empty BatchWrite: %v", err)
	}
	if !ok {
		t.Error("expected true for empty BatchWrite")
	}
}
