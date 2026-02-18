// Ports: test/validate.ts
package tests

import (
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestValidate_Valid(t *testing.T) {
	tbl, _ := makeTable(t, "ValidateTable", ValidationSchema, false)
	params := ot.Item{
		"name":    "Peter O'Flanagan",
		"email":   "peter@example.com",
		"address": "444 Cherry Tree Lane",
		"city":    "San Francisco",
		"zip":     "98103",
		"phone":   "(408) 4847700",
		"status":  "active",
	}
	user, err := tbl.Create(bg(), "User", params, nil)
	if err != nil {
		t.Fatalf("Create valid: %v", err)
	}
	assertStr(t, user, "name", "Peter O'Flanagan")
	assertStr(t, user, "email", "peter@example.com")
}

func TestValidate_UpdateWithoutRequired(t *testing.T) {
	tbl, _ := makeTable(t, "ValidateTable", ValidationSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"name": "Peter O'Flanagan", "email": "peter@example.com",
		"status": "active",
	}, nil)
	updated, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"], "age": float64(42)}, nil)
	if err != nil {
		t.Fatalf("Update without required: %v", err)
	}
	assertNum(t, updated, "age", 42)
}

func TestValidate_Invalid(t *testing.T) {
	tbl, _ := makeTable(t, "ValidateTable", ValidationSchema, false)
	params := ot.Item{
		"name":    "Peter@O'Flanagan",       // invalid: contains @
		"email":   "peter example.com",      // invalid: no @
		"address": "444 Cherry Tree Lane[]", // invalid: []
		"city":    "New York",               // invalid: not San Francisco
		"zip":     "98103@@1234",            // invalid: @@
		"phone":   "not-connected",          // invalid
		"age":     float64(99),
		// missing status
	}
	_, err := tbl.Create(bg(), "User", params, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
	ote, ok := err.(*ot.OneTableError)
	if !ok {
		t.Fatalf("expected OneTableError, got %T: %v", err, err)
	}
	assertContains(t, ote.Message, "Validation Error in \"User\"")
	validation, _ := ote.Context["validation"].(map[string]string)
	if validation == nil {
		t.Fatal("expected validation map in context")
	}
	for _, field := range []string{"name", "email", "address", "city", "zip", "phone"} {
		if validation[field] == "" {
			t.Errorf("expected validation error for %q", field)
		}
	}
	if validation["age"] != "" {
		t.Errorf("age should not have validation error")
	}
}

func TestValidate_MissingRequired(t *testing.T) {
	tbl, _ := makeTable(t, "ValidateTable", ValidationSchema, false)
	_, err := tbl.Create(bg(), "User", ot.Item{
		"name":    "Jenny Smith",
		"address": "444 Cherry Tree Lane",
		"status":  "active",
		"age":     float64(42),
		// missing email
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing required email")
	}
	ote, ok := err.(*ot.OneTableError)
	if !ok {
		t.Fatalf("expected OneTableError, got %T", err)
	}
	validation, _ := ote.Context["validation"].(map[string]string)
	if validation["email"] == "" {
		t.Error("expected validation error for email")
	}
	if validation["status"] != "" {
		t.Error("status should not have error (it was provided)")
	}
}

func TestValidate_RemoveRequired(t *testing.T) {
	tbl, _ := makeTable(t, "ValidateTable", ValidationSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{
		"name": "Jenny Smith", "email": "jenny@example.com", "status": "active",
	}, nil)
	_, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"], "email": nil}, nil)
	if err == nil {
		t.Fatal("expected error when nulling required email")
	}
}

func TestValidate_Enum(t *testing.T) {
	tbl, _ := makeTable(t, "ValidateTable", DefaultSchema, false)
	// valid enum
	_, err := tbl.Create(bg(), "Pet", ot.Item{"name": "Rex", "race": "dog", "breed": "Lab"}, nil)
	if err != nil {
		t.Fatalf("valid enum: %v", err)
	}
	// invalid enum
	_, err = tbl.Create(bg(), "Pet", ot.Item{"name": "Rex", "race": "dragon", "breed": "Lab"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid enum")
	}
}
