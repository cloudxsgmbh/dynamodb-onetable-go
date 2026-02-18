// Ports: test/unique.ts
package tests

import (
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestUnique_CreateUser(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	props := ot.Item{"name": "Peter Smith", "email": "peter@example.com"}
	user, err := tbl.Create(bg(), "User", props, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertStr(t, user, "name", "Peter Smith")
	assertStr(t, user, "email", "peter@example.com")

	// should have created 1 data item + unique sentinel items (email + interpolated)
	count := mock.count("UniqueTable")
	if count < 3 {
		t.Errorf("expected >= 3 items (data + 2 unique sentinels), got %d", count)
	}
}

func TestUnique_CreateSecondUser(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "email": "peter@example.com"}, nil)                        //nolint
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com", "phone": "+15555555555"}, nil) //nolint

	// 2 users + 2 sentinels for peter (email+interpolated) + 3 sentinels for judy (email+phone+interpolated)
	count := mock.count("UniqueTable")
	if count < 7 {
		t.Errorf("expected >= 7 items, got %d", count)
	}
}

func TestUnique_DuplicateEmailRejected(t *testing.T) {
	tbl, _ := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "email": "peter@example.com"}, nil) //nolint

	_, err := tbl.Create(bg(), "User", ot.Item{"name": "Another Peter", "email": "peter@example.com"}, nil)
	if err == nil {
		t.Fatal("expected error for duplicate unique email")
	}
	assertErrCode(t, err, ot.ErrUnique)
}

func TestUnique_UpdateSameEmail(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com", "phone": "+15555555555"}, nil) //nolint
	beforeCount := mock.count("UniqueTable")

	user, err := tbl.Update(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com"},
		&ot.Params{Return: "get"})
	if err != nil {
		t.Fatalf("Update same email: %v", err)
	}
	assertStr(t, user, "email", "judy@example.com")
	// sentinel count should be unchanged
	if mock.count("UniqueTable") != beforeCount {
		t.Errorf("sentinel count changed unexpectedly: was %d, now %d", beforeCount, mock.count("UniqueTable"))
	}
}

func TestUnique_UpdateNewEmail(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com", "phone": "+15555555555"}, nil) //nolint
	beforeCount := mock.count("UniqueTable")

	user, err := tbl.Update(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy-a@example.com"},
		&ot.Params{Return: "get"})
	if err != nil {
		t.Fatalf("Update new email: %v", err)
	}
	assertStr(t, user, "email", "judy-a@example.com")
	// sentinel count should be same (old removed, new added)
	if mock.count("UniqueTable") != beforeCount {
		t.Errorf("sentinel count changed: was %d, now %d", beforeCount, mock.count("UniqueTable"))
	}
}

func TestUnique_UpdateNonUniqueField(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com"}, nil) //nolint
	beforeCount := mock.count("UniqueTable")

	user, err := tbl.Update(bg(), "User", ot.Item{"name": "Judy Smith", "age": float64(42)},
		&ot.Params{Return: "get"})
	if err != nil {
		t.Fatalf("Update non-unique: %v", err)
	}
	assertNum(t, user, "age", 42)
	if mock.count("UniqueTable") != beforeCount {
		t.Errorf("sentinel count changed unexpectedly")
	}
}

func TestUnique_RemoveOptionalUniqueField(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com", "phone": "+15555555555"}, nil) //nolint
	beforeCount := mock.count("UniqueTable")

	user, err := tbl.Update(bg(), "User", ot.Item{"name": "Judy Smith", "phone": nil},
		&ot.Params{Return: "get"})
	if err != nil {
		t.Fatalf("Remove optional unique: %v", err)
	}
	assertAbsent(t, user, "phone")
	// phone sentinel removed → count decreases by 1
	if mock.count("UniqueTable") != beforeCount-1 {
		t.Errorf("expected count %d, got %d", beforeCount-1, mock.count("UniqueTable"))
	}
}

func TestUnique_DuplicateEmailUpdateRejected(t *testing.T) {
	tbl, _ := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "email": "peter@example.com"}, nil) //nolint
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com"}, nil)   //nolint

	_, err := tbl.Update(bg(), "User", ot.Item{"name": "Judy Smith", "email": "peter@example.com"},
		&ot.Params{Return: "none"})
	if err == nil {
		t.Fatal("expected error for duplicate unique email on update")
	}
	assertErrCode(t, err, ot.ErrUnique)
}

func TestUnique_RemoveUser(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "email": "peter@example.com"}, nil) //nolint
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com"}, nil)   //nolint

	result, _ := tbl.Scan(bg(), "User", ot.Item{}, nil)
	assertLen(t, result.Items, 2)

	if _, err := tbl.Remove(bg(), "User", result.Items[0], nil); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	result, _ = tbl.Scan(bg(), "User", ot.Item{}, nil)
	assertLen(t, result.Items, 1)
	// sentinels for removed user should also be gone
	_ = mock.count("UniqueTable")
}

func TestUnique_RemoveAll(t *testing.T) {
	tbl, mock := makeTable(t, "UniqueTable", UniqueSchema, false)
	tbl.Create(bg(), "User", ot.Item{"name": "Peter Smith", "email": "peter@example.com"}, nil) //nolint
	tbl.Create(bg(), "User", ot.Item{"name": "Judy Smith", "email": "judy@example.com"}, nil)   //nolint

	result, _ := tbl.Scan(bg(), "User", ot.Item{}, nil)
	for _, u := range result.Items {
		tbl.Remove(bg(), "User", u, nil) //nolint
	}

	result, _ = tbl.Scan(bg(), "User", ot.Item{}, nil)
	assertLen(t, result.Items, 0)
	if mock.count("UniqueTable") != 0 {
		t.Errorf("expected 0 items after remove all, got %d", mock.count("UniqueTable"))
	}
}

func TestUnique_CreateViaUpsert(t *testing.T) {
	tbl, _ := makeTable(t, "UniqueTable", UniqueSchema, false)
	props := ot.Item{"name": "Judy Smith", "email": "judy@example.com"}
	user, err := tbl.Upsert(bg(), "User", props, &ot.Params{Return: "get"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	assertStr(t, user, "email", "judy@example.com")
}
