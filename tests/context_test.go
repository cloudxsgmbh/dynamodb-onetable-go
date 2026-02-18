// Ports: test/context.ts
package tests

import (
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestContext_SetGetClear(t *testing.T) {
	tbl, _ := makeTable(t, "ContextTable", TenantSchema, false)
	account, _ := tbl.Create(bg(), "Account", ot.Item{"name": "Acme"}, nil)
	accountID := account["id"]

	tbl.SetContext(ot.Item{"accountId": accountID}, false)
	ctx := tbl.GetContext()
	if ctx["accountId"] != accountID {
		t.Errorf("context accountId: got %v, want %v", ctx["accountId"], accountID)
	}

	// merge
	tbl.SetContext(ot.Item{"color": "blue"}, true)
	ctx = tbl.GetContext()
	if ctx["accountId"] != accountID {
		t.Errorf("merge lost accountId")
	}
	if ctx["color"] != "blue" {
		t.Errorf("merge missing color")
	}

	// revert
	tbl.SetContext(ot.Item{"accountId": accountID}, false)
	ctx = tbl.GetContext()
	if _, hasColor := ctx["color"]; hasColor {
		t.Error("color should be gone after SetContext")
	}

	// addContext
	tbl.AddContext(ot.Item{"color": "blue"})
	ctx = tbl.GetContext()
	if ctx["color"] != "blue" {
		t.Error("AddContext missing color")
	}

	// clear
	tbl.ClearContext()
	ctx = tbl.GetContext()
	if len(ctx) != 0 {
		t.Errorf("expected empty context after clear, got %v", ctx)
	}
}

func TestContext_CreateUsersWithContext(t *testing.T) {
	tbl, _ := makeTable(t, "ContextTable", TenantSchema, false)
	account, _ := tbl.Create(bg(), "Account", ot.Item{"name": "Acme"}, nil)
	accountID := account["id"]

	tbl.SetContext(ot.Item{"accountId": accountID}, false)

	data := []ot.Item{
		{"name": "Peter Smith", "email": "peter@example.com"},
		{"name": "Patty O'Furniture", "email": "patty@example.com"},
		{"name": "Cu Later", "email": "cu@example.com"},
	}
	for _, d := range data {
		user, err := tbl.Create(bg(), "User", d, nil)
		if err != nil {
			t.Fatalf("Create user: %v", err)
		}
		assertULID(t, user["id"])
		if user["accountId"] != accountID {
			t.Errorf("accountId: got %v, want %v", user["accountId"], accountID)
		}
	}

	result, err := tbl.Scan(bg(), "User", ot.Item{}, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	assertLen(t, result.Items, 3)
}

func TestContext_RemoveMany(t *testing.T) {
	tbl, _ := makeTable(t, "ContextTable", TenantSchema, false)
	account, _ := tbl.Create(bg(), "Account", ot.Item{"name": "Acme"}, nil)
	tbl.SetContext(ot.Item{"accountId": account["id"]}, false)

	for _, d := range []ot.Item{
		{"name": "Peter Smith", "email": "peter@example.com"},
		{"name": "Patty O'Furniture", "email": "patty@example.com"},
	} {
		tbl.Create(bg(), "User", d, nil) //nolint
	}

	_, err := tbl.Remove(bg(), "User", ot.Item{}, &ot.Params{Many: true})
	if err != nil {
		t.Fatalf("Remove many: %v", err)
	}

	result, _ := tbl.Scan(bg(), "User", ot.Item{}, nil)
	assertLen(t, result.Items, 0)
}
