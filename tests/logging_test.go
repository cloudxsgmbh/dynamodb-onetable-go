package tests

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestInfoLogOmitsReadRequestValues(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
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
	for _, metadata := range []string{"OneTable", "get", "User"} {
		if !strings.Contains(logged, metadata) {
			t.Errorf("INFO log is missing safe metadata %q", metadata)
		}
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

	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
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
	if !strings.Contains(logged, "OneTable") || !strings.Contains(logged, "transactWrite") {
		t.Errorf("INFO log is missing safe transaction metadata")
	}
}
