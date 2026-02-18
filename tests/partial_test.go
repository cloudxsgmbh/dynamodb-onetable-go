// Ports: test/partial.ts
package tests

import (
	"testing"
	"time"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestPartial_Create(t *testing.T) {
	tbl, _ := makeTable(t, "PartialTable", PartialSchema, true)
	user, err := tbl.Create(bg(), "User", ot.Item{
		"email":  "user@example.com",
		"id":     "42",
		"status": "active",
		"address": map[string]any{
			"street": "42 Park Ave",
			"zip":    float64(12345),
			"box":    map[string]any{"start": time.Now()},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertStr(t, user, "email", "user@example.com")
	addr, ok := user["address"].(map[string]any)
	if !ok {
		t.Fatalf("address not map: %T", user["address"])
	}
	assertStr(t, addr, "street", "42 Park Ave")
	assertNum(t, addr, "zip", 12345)
}

func TestPartial_Get(t *testing.T) {
	tbl, _ := makeTable(t, "PartialTable", PartialSchema, true)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"email": "user@example.com", "id": "42", "status": "active",
		"address": map[string]any{"street": "42 Park Ave", "zip": float64(12345)},
	}, nil)

	got, err := tbl.Get(bg(), "User", ot.Item{"id": user["id"],
		"address": map[string]any{"zip": float64(12345)}}, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected item")
	}
	assertStr(t, got, "email", "user@example.com")
}

func TestPartial_UpdateEmail(t *testing.T) {
	tbl, _ := makeTable(t, "PartialTable", PartialSchema, true)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"email": "user@example.com", "id": "42", "status": "active",
		"address": map[string]any{"street": "42 Park Ave", "zip": float64(12345)},
	}, nil)

	updated, err := tbl.Update(bg(), "User", ot.Item{
		"id": user["id"], "email": "ralph@example.com",
		"address": map[string]any{"box": map[string]any{"start": time.Now(), "end": time.Now()}},
	}, nil)
	if err != nil {
		t.Fatalf("Update email: %v", err)
	}
	assertStr(t, updated, "email", "ralph@example.com")
}

func TestPartial_UpdateZipPartial(t *testing.T) {
	tbl, _ := makeTable(t, "PartialTable", PartialSchema, true)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"email": "user@example.com", "id": "42", "status": "active",
		"address": map[string]any{"street": "42 Park Ave", "zip": float64(12345)},
	}, nil)

	_, err := tbl.Update(bg(), "User", ot.Item{
		"id":      user["id"],
		"address": map[string]any{"zip": float64(99999)},
	}, nil)
	if err != nil {
		t.Fatalf("Update zip partial: %v", err)
	}
}

func TestPartial_UpdateFullReplace(t *testing.T) {
	tbl, _ := makeTable(t, "PartialTable", PartialSchema, true)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"email": "user@example.com", "id": "42", "status": "active",
		"address": map[string]any{"street": "42 Park Ave", "zip": float64(12345)},
	}, nil)

	_, err := tbl.Update(bg(), "User", ot.Item{
		"id":      user["id"],
		"address": map[string]any{"zip": float64(22222)},
	}, &ot.Params{Partial: boolPtr(false)})
	if err != nil {
		t.Fatalf("Update full replace: %v", err)
	}
}
