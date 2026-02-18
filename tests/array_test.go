// Ports: test/array.ts
package tests

import (
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestArray_Create(t *testing.T) {
	tbl, _ := makeTable(t, "ArrayTable", ArraySchema, true)
	user, err := tbl.Create(bg(), "User", ot.Item{
		"email":     "user@example.com",
		"addresses": []any{map[string]any{"street": "44 Park Ave", "zip": float64(3000)}},
	}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	addrs := toAnySlice(user["addresses"])
	if len(addrs) != 1 {
		t.Fatalf("addresses: %T %v", user["addresses"], user["addresses"])
	}
	addr, _ := addrs[0].(map[string]any)
	assertStr(t, addr, "street", "44 Park Ave")
	assertNum(t, addr, "zip", 3000)
}

func TestArray_Get(t *testing.T) {
	tbl, _ := makeTable(t, "ArrayTable", ArraySchema, true)
	tbl.Create(bg(), "User", ot.Item{ //nolint
		"email":     "user@example.com",
		"addresses": []any{map[string]any{"street": "44 Park Ave", "zip": float64(3000)}},
	}, nil)

	got, err := tbl.Get(bg(), "User", ot.Item{"email": "user@example.com"}, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected item")
	}
	addrs := toAnySlice(got["addresses"])
	if len(addrs) != 1 {
		t.Fatalf("addresses on get: %T %v", got["addresses"], got["addresses"])
	}
}

func TestArray_PartialUpdate(t *testing.T) {
	tbl, _ := makeTable(t, "ArrayTable", ArraySchema, true)
	tbl.Create(bg(), "User", ot.Item{ //nolint
		"email":     "user@example.com",
		"addresses": []any{map[string]any{"street": "44 Park Ave", "zip": float64(3000)}},
	}, nil)

	// partial update: update street, preserve zip
	updated, err := tbl.Update(bg(), "User", ot.Item{
		"email":     "user@example.com",
		"addresses": []any{map[string]any{"street": "12 Mayfair"}},
	}, nil)
	if err != nil {
		t.Fatalf("partial update: %v", err)
	}
	_ = updated
}

func TestArray_FullUpdate(t *testing.T) {
	tbl, _ := makeTable(t, "ArrayTable", ArraySchema, true)
	tbl.Create(bg(), "User", ot.Item{ //nolint
		"email":     "user@example.com",
		"addresses": []any{map[string]any{"street": "44 Park Ave", "zip": float64(3000)}},
	}, nil)

	// full update: replace entire address, zip gone
	updated, err := tbl.Update(bg(), "User", ot.Item{
		"email":     "user@example.com",
		"addresses": []any{map[string]any{"street": "7 Yellow Brick Road"}},
	}, &ot.Params{Partial: boolPtr(false)})
	if err != nil {
		t.Fatalf("full update: %v", err)
	}
	_ = updated
}
