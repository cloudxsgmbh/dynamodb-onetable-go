package tests

import (
	"bytes"
	"context"
	"log"
	"strings"
	"sync"
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

type lockedLogBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedLogBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(value)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func TestInfoLogOmitsReadRequestValues(t *testing.T) {
	output := &lockedLogBuffer{}
	previous := log.Writer()
	log.SetOutput(output)
	t.Cleanup(func() { log.SetOutput(previous) })

	table, _ := makeTable(t, "ReadRequestSecretTable", DefaultSchema, false)
	_, err := table.Get(context.Background(), "User", ot.Item{"id": "read-key-secret-7d4a"}, &ot.Params{Consistent: true})
	if err != nil {
		t.Fatal(err)
	}
	logged := output.String()
	for _, secret := range []string{"read-key-secret-7d4a", "ReadRequestSecretTable"} {
		if strings.Contains(logged, secret) {
			t.Errorf("INFO log contains request value %q", secret)
		}
	}
	for _, metadata := range []string{"OneTable", "get", "User", `"op":"get"`} {
		if !strings.Contains(logged, metadata) {
			t.Errorf("INFO log is missing safe metadata %q", metadata)
		}
	}
}

func TestInfoLogOmitsUnexecutedCommandValues(t *testing.T) {
	output := &lockedLogBuffer{}
	previous := log.Writer()
	log.SetOutput(output)
	t.Cleanup(func() { log.SetOutput(previous) })

	table, _ := makeTable(t, "DryRunSecretTable", DefaultSchema, false)
	execute := false
	command, err := table.Get(context.Background(), "User", ot.Item{
		"id": "dry-run-key-secret-7ac1",
	}, &ot.Params{Execute: &execute})
	if err != nil {
		t.Fatal(err)
	}
	if command["TableName"] != "DryRunSecretTable" || command["Key"] == nil {
		t.Fatalf("Execute=false stopped returning the prepared command: %#v", command)
	}
	logged := output.String()
	for _, secret := range []string{"DryRunSecretTable", "dry-run-key-secret-7ac1"} {
		if strings.Contains(logged, secret) {
			t.Errorf("INFO log contains unexecuted request value %q", secret)
		}
	}
	if !strings.Contains(logged, "OneTable") || !strings.Contains(logged, `"op":"get"`) {
		t.Errorf("INFO log is missing safe operation metadata")
	}
}

func TestInfoLogOmitsTransactionRequestValues(t *testing.T) {
	primary, client := makeTable(t, "PrimaryTransactionTable", DefaultSchema, false)
	secondary, err := ot.NewTable(ot.TableParams{
		Name: "AuthSessionSecretTable", Client: client, Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatal(err)
	}
	primaryUser, err := primary.GetModel("User")
	if err != nil {
		t.Fatal(err)
	}
	secondaryUser, err := secondary.GetModel("User")
	if err != nil {
		t.Fatal(err)
	}
	_, err = primaryUser.Create(context.Background(), ot.Item{
		"id": "condition-check-key", "status": "condition-secret-29c1",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	output := &lockedLogBuffer{}
	previous := log.Writer()
	log.SetOutput(output)
	t.Cleanup(func() { log.SetOutput(previous) })

	tx := ot.NewTransaction()
	params, err := tx.Params(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = primaryUser.Create(context.Background(), ot.Item{
		"id": "transaction-key-secret-83b0", "name": "transaction-value-secret-c05d",
	}, params)
	if err != nil {
		t.Fatal(err)
	}
	params, err = tx.Params(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = secondaryUser.Create(context.Background(), ot.Item{
		"id": "session-key-secret-41aa", "name": "session-value-secret-91ef",
	}, params)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Check(context.Background(), primaryUser, ot.Item{"id": "condition-check-key"}, &ot.Params{
		Where: "${status} = {condition-secret-29c1}",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Write(context.Background(), primary, nil)
	if err != nil {
		t.Fatal(err)
	}
	logged := output.String()
	for _, secret := range []string{
		"AuthSessionSecretTable", "transaction-key-secret-83b0", "transaction-value-secret-c05d",
		"condition-secret-29c1", "session-key-secret-41aa", "session-value-secret-91ef",
	} {
		if strings.Contains(logged, secret) {
			t.Errorf("INFO log contains transaction value %q", secret)
		}
	}
	if !strings.Contains(logged, "OneTable") || !strings.Contains(logged, `"op":"transactWrite"`) {
		t.Errorf("INFO log is missing safe transaction metadata")
	}
}
