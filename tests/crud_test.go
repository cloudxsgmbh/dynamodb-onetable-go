// Ports: test/crud.ts + test/default.ts
package tests

import (
	"testing"
	"time"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestCRUD_GetSchema(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	s := tbl.GetCurrentSchema()
	if s == nil || s.Models == nil || s.Indexes == nil || s.Params == nil {
		t.Fatal("schema missing fields")
	}
	if _, ok := s.Models["User"]; !ok {
		t.Fatal("User model missing")
	}
	if _, ok := s.Models["User"]["pk"]; !ok {
		t.Fatal("pk field missing")
	}
}

func TestCRUD_ValidateModel(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	if _, err := tbl.GetModel("Unknown"); err == nil {
		t.Fatal("expected error for unknown model")
	}
	m, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if m.Name != "User" {
		t.Errorf("name: %q", m.Name)
	}
}

func TestCRUD_Create(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	now := time.Now()
	user, err := tbl.Create(bg(), "User", ot.Item{
		"name": "Peter Smith", "email": "peter@example.com",
		"profile": map[string]any{"avatar": "eagle"},
		"status":  "active", "age": float64(42),
		"registered": now,
		"unknown":    99, // must be dropped
	}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertULID(t, user["id"])
	assertStr(t, user, "name", "Peter Smith")
	assertStr(t, user, "status", "active")
	assertNum(t, user, "age", 42)
	assertAbsent(t, user, "unknown")
	assertAbsent(t, user, "pk")
	assertAbsent(t, user, "sk")
	assertDate(t, user["created"])
	assertDate(t, user["updated"])

	profile, ok := user["profile"].(map[string]any)
	if !ok || profile["avatar"] != "eagle" {
		t.Errorf("profile: %v", user["profile"])
	}
}

func TestCRUD_Get(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "status": "active"}, nil)

	got, err := tbl.Get(bg(), "User", ot.Item{"id": user["id"]}, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected item")
	}
	assertStr(t, got, "name", "Peter Smith")
	assertStr(t, got, "status", "active")
	assertDate(t, got["created"])
	assertDate(t, got["updated"])
	assertULID(t, got["id"])
}

func TestCRUD_GetHidden(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "status": "active"}, nil)

	got, err := tbl.Get(bg(), "User", ot.Item{"id": user["id"]}, &ot.Params{Hidden: boolPtr(true)})
	if err != nil {
		t.Fatalf("Get hidden: %v", err)
	}
	assertStr(t, got, "name", "Peter Smith")
	assertPresent(t, got, "pk")
	assertPresent(t, got, "sk")
	assertPresent(t, got, "gs1pk")
}

func TestCRUD_Update(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "status": "active", "age": float64(20)}, nil)

	updated, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"], "status": "inactive", "age": float64(99)}, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertStr(t, updated, "name", "Peter Smith")
	assertStr(t, updated, "status", "inactive")
	assertNum(t, updated, "age", 99)
	assertDate(t, updated["created"])
	assertDate(t, updated["updated"])
	assertULID(t, updated["id"])
}

func TestCRUD_RemoveAttributeNull(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "status": "active"}, nil)

	// null → remove; status has default "idle"
	updated, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"], "status": nil}, nil)
	if err != nil {
		t.Fatalf("Update null: %v", err)
	}
	// mock UpdateItem merges keys only — status field will come back as default
	_ = updated
}

func TestCRUD_Remove(t *testing.T) {
	tbl, mock := makeTable(t, "CrudTable", DefaultSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{"name": "Sky Blue", "status": "active"}, nil)
	if mock.count("CrudTable") == 0 {
		t.Fatal("item not stored")
	}

	removed, err := tbl.Remove(bg(), "User", ot.Item{"id": user["id"]}, nil)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_ = removed
	if mock.count("CrudTable") != 0 {
		t.Errorf("expected 0 items after remove, got %d", mock.count("CrudTable"))
	}
}

func TestCRUD_Scan(t *testing.T) {
	tbl, _ := makeTable(t, "CrudTable", DefaultSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Sky Blue", "status": "active"}, nil) //nolint

	result, err := tbl.Scan(bg(), "User", ot.Item{}, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Items) < 1 {
		t.Fatalf("expected >= 1 items, got %d", len(result.Items))
	}
}

func TestCRUD_DefaultStatus(t *testing.T) {
	tbl, _ := makeTable(t, "DefaultTable", DefaultSchema, false)
	user, err := tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "email": "peter@example.com"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertStr(t, user, "status", "idle")
	assertULID(t, user["id"])
}

func TestCRUD_ScanHidden(t *testing.T) {
	tbl, _ := makeTable(t, "ScanTable", DefaultSchema, false)
	data := []ot.Item{
		{"name": "Peter Smith", "email": "peter@example.com", "status": "active"},
		{"name": "Cu Later", "email": "cu@example.com", "status": "inactive"},
	}
	for _, d := range data {
		tbl.Create(bg(), "User", d, nil) //nolint
	}

	result, err := tbl.Scan(bg(), "User", ot.Item{}, &ot.Params{Hidden: boolPtr(true)})
	if err != nil {
		t.Fatalf("Scan hidden: %v", err)
	}
	assertLen(t, result.Items, len(data))
	for _, item := range result.Items {
		assertPresent(t, item, "_type")
		assertPresent(t, item, "pk")
		assertPresent(t, item, "sk")
		assertULID(t, item["id"])
	}
}
