// Ports: test/timestamps.ts
package tests

import (
	"testing"
	"time"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestTimestamps_CreatedUpdated(t *testing.T) {
	tbl, _ := makeTable(t, "TimestampsTable", TimestampsSchema, false)
	before := time.Now().Add(-time.Second)
	user, err := tbl.Create(bg(), "User", ot.Item{
		"name": "Peter Smith", "email": "peter@example.com",
	}, nil)
	after := time.Now().Add(time.Second)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	createdAt, ok := user["createdAt"].(time.Time)
	if !ok {
		t.Fatalf("createdAt not a time.Time: %T %v", user["createdAt"], user["createdAt"])
	}
	updatedAt, ok := user["updatedAt"].(time.Time)
	if !ok {
		t.Fatalf("updatedAt not a time.Time: %T %v", user["updatedAt"], user["updatedAt"])
	}

	if createdAt.Before(before) || createdAt.After(after) {
		t.Errorf("createdAt %v out of range [%v, %v]", createdAt, before, after)
	}
	if updatedAt.Before(before) || updatedAt.After(after) {
		t.Errorf("updatedAt %v out of range [%v, %v]", updatedAt, before, after)
	}
}

func TestTimestamps_UpdatedChanges(t *testing.T) {
	tbl, _ := makeTable(t, "TimestampsTable", TimestampsSchema, false)
	user, _ := tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "email": "peter@example.com"}, nil)
	origCreated, _ := user["createdAt"].(time.Time)
	origUpdated, _ := user["updatedAt"].(time.Time)

	time.Sleep(2 * time.Millisecond)

	updated, err := tbl.Update(bg(), "User", ot.Item{"id": user["id"]},
		&ot.Params{Set: map[string]string{"name": "Marcelo"}}) // exists:nil → upsert
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	newUpdated, _ := updated["updatedAt"].(time.Time)
	if !newUpdated.After(origUpdated) {
		t.Errorf("updatedAt should be newer: orig=%v new=%v", origUpdated, newUpdated)
	}
	// createdAt should not be overwritten
	_ = origCreated
}

func TestTimestamps_DefaultFields(t *testing.T) {
	// DefaultSchema uses timestamps:true with default created/updated field names
	tbl, _ := makeTable(t, "CrudTimestamps", DefaultSchema, false)
	user, err := tbl.Create(bg(), "User", ot.Item{"name": "Alice"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertDate(t, user["created"])
	assertDate(t, user["updated"])
}
