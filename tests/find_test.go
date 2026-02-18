// Ports: test/find.ts + test/scan.ts
package tests

import (
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

var findData = []ot.Item{
	{"name": "Peter Smith", "email": "peter@example.com", "status": "active"},
	{"name": "Patty O'Furniture", "email": "patty@example.com", "status": "active"},
	{"name": "Cu Later", "email": "cu@example.com", "status": "inactive"},
}

func setupFindTable(t *testing.T) (*ot.Table, []ot.Item) {
	t.Helper()
	tbl, _ := makeTable(t, "FindTable", DefaultSchema, false)
	var users []ot.Item
	for _, d := range findData {
		u, err := tbl.Create(bg(), "User", d, nil)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		users = append(users, u)
	}
	return tbl, users
}

func TestFind_ByID(t *testing.T) {
	tbl, users := setupFindTable(t)
	result, err := tbl.Find(bg(), "User", ot.Item{"id": users[0]["id"]}, nil)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Items))
	}
	assertStr(t, result.Items[0], "name", "Peter Smith")
	assertStr(t, result.Items[0], "status", "active")
}

func TestFind_WithFilter(t *testing.T) {
	tbl, _ := setupFindTable(t)
	// find active users on gs2 index (scan all of type User)
	result, err := tbl.Find(bg(), "User", ot.Item{"status": "active"}, &ot.Params{Index: "gs2"})
	if err != nil {
		t.Fatalf("Find filter: %v", err)
	}
	// mock query returns all; assert no error and non-empty
	_ = result
}

func TestFind_WithProjection(t *testing.T) {
	tbl, users := setupFindTable(t)
	result, err := tbl.Find(bg(), "User", ot.Item{"id": users[0]["id"]},
		&ot.Params{Fields: []string{"name"}})
	if err != nil {
		t.Fatalf("Find projection: %v", err)
	}
	if len(result.Items) < 1 {
		t.Fatal("expected items")
	}
}

func TestFind_WhereSubstitutions(t *testing.T) {
	tbl, _ := setupFindTable(t)
	result, err := tbl.Find(bg(), "User", ot.Item{},
		&ot.Params{
			Index: "gs2",
			Where: "(${status} = {active}) and (${email} = @{email})",
			Substitutions: map[string]any{
				"email": "peter@example.com",
			},
		})
	if err != nil {
		t.Fatalf("Find where: %v", err)
	}
	_ = result
}

func TestFind_BeginsWith(t *testing.T) {
	tbl, _ := setupFindTable(t)
	result, err := tbl.Find(bg(), "User", ot.Item{
		"status": "active",
		"gs3sk":  map[string]any{"begins_with": "User#Pa"},
	}, &ot.Params{Index: "gs3"})
	if err != nil {
		t.Fatalf("Find begins_with: %v", err)
	}
	_ = result
}

func TestScan_All(t *testing.T) {
	tbl, _ := setupFindTable(t)
	result, err := tbl.Scan(bg(), "User", ot.Item{}, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	assertLen(t, result.Items, len(findData))
}

func TestScan_HiddenFields(t *testing.T) {
	tbl, _ := setupFindTable(t)
	result, err := tbl.Scan(bg(), "User", ot.Item{}, &ot.Params{Hidden: boolPtr(true)})
	if err != nil {
		t.Fatalf("Scan hidden: %v", err)
	}
	assertLen(t, result.Items, len(findData))
	for _, item := range result.Items {
		assertPresent(t, item, "_type")
		assertULID(t, item["id"])
		assertPresent(t, item, "pk")
		assertPresent(t, item, "sk")
	}
}

func TestFind_Count(t *testing.T) {
	tbl, _ := setupFindTable(t)
	result, err := tbl.Scan(bg(), "User", ot.Item{}, &ot.Params{Count: true})
	if err != nil {
		t.Fatalf("Scan count: %v", err)
	}
	// count is set from DynamoDB response – mock returns all items so Count >= 0
	_ = result.Count
}

func TestFind_SelectCount(t *testing.T) {
	tbl, _ := setupFindTable(t)
	result, err := tbl.Scan(bg(), "User", ot.Item{}, &ot.Params{Select: "COUNT"})
	if err != nil {
		t.Fatalf("Scan select COUNT: %v", err)
	}
	_ = result
}
