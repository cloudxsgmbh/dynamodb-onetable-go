// Ports: test/nested.ts
package tests

import (
	"testing"
	"time"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestNested_Create(t *testing.T) {
	tbl, _ := makeTable(t, "NestedTable", NestedSchema, false)
	now := time.Now()
	user, err := tbl.Create(bg(), "User", ot.Item{
		"name":    "Peter Smith",
		"email":   "peter@example.com",
		"status":  "active",
		"balance": float64(0),
		"tokens":  []any{"red", "white", "blue"},
		"started": now,
		"location": map[string]any{
			"address": "444 Cherry Tree Lane",
			"city":    "Seattle",
			"zip":     "98011",
			"started": now,
			"unknown": 99, // must be dropped from nested schema
		},
		"unknown": 42, // must be dropped at top level
	}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	assertULID(t, user["id"])
	assertNum(t, user, "balance", 0)
	assertAbsent(t, user, "unknown")
	assertAbsent(t, user, "pk")
	assertAbsent(t, user, "sk")
	assertDate(t, user["created"])
	assertDate(t, user["updated"])

	loc, ok := user["location"].(map[string]any)
	if !ok {
		t.Fatalf("location not a map: %T", user["location"])
	}
	assertStr(t, loc, "city", "Seattle")
	if _, hasUnknown := loc["unknown"]; hasUnknown {
		t.Error("nested unknown field should be dropped")
	}
	if _, ok := loc["started"].(time.Time); !ok {
		t.Errorf("nested date not parsed: %T %v", loc["started"], loc["started"])
	}
}

func TestNested_Get(t *testing.T) {
	tbl, _ := makeTable(t, "NestedTable", NestedSchema, false)
	now := time.Now()
	created, _ := tbl.Create(bg(), "User", ot.Item{
		"name": "Peter Smith",
		"location": map[string]any{
			"city": "Seattle", "zip": "98011", "started": now,
		},
	}, nil)

	got, err := tbl.Get(bg(), "User", ot.Item{"id": created["id"]}, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected item")
	}
	loc, ok := got["location"].(map[string]any)
	if !ok {
		t.Fatalf("location: %T", got["location"])
	}
	assertStr(t, loc, "city", "Seattle")
}

func TestNested_UpdateTopLevel(t *testing.T) {
	tbl, _ := makeTable(t, "NestedTable", NestedSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "status": "active"}, nil)

	updated, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"], "status": "inactive"}, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertStr(t, updated, "status", "inactive")
}

func TestNested_UpdateNestedViaSet(t *testing.T) {
	tbl, _ := makeTable(t, "NestedTable", NestedSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"name":     "Peter Smith",
		"location": map[string]any{"address": "Old St", "city": "Seattle", "zip": "98011"},
	}, nil)

	_, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"]},
		&ot.Params{Set: map[string]string{"location.zip": `{"98012"}`}})
	if err != nil {
		t.Fatalf("Update nested set: %v", err)
	}
}

func TestNested_RemoveNestedViaParams(t *testing.T) {
	tbl, _ := makeTable(t, "NestedTable", NestedSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"name":     "Peter Smith",
		"location": map[string]any{"city": "Seattle", "zip": "98011"},
	}, nil)

	_, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"]},
		&ot.Params{Remove: []string{"location.zip"}})
	if err != nil {
		t.Fatalf("Update remove nested: %v", err)
	}
}
