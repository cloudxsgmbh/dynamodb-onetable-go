package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestModelCheckAccumulatesSchemaAwareCondition(t *testing.T) {
	tbl, _ := makeTable(t, "CheckTable", DefaultSchema, false)
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}

	tx := map[string]any{}
	if _, err := model.Check(bg(), ot.Item{"id": "u-check"}, &ot.Params{
		Transaction: tx,
		Where:       `${status} = {active}`,
	}); err != nil {
		t.Fatalf("Check: %v", err)
	}

	items, ok := tx["TransactItems"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("TransactItems = %#v, want one check", tx["TransactItems"])
	}
	entry, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("transaction entry = %T, want map[string]any", items[0])
	}
	command, ok := entry["ConditionCheck"].(ot.Item)
	if !ok {
		t.Fatalf("ConditionCheck = %T, want onetable.Item", entry["ConditionCheck"])
	}
	if command["TableName"] != "CheckTable" {
		t.Errorf("TableName = %v, want CheckTable", command["TableName"])
	}
	if expression, _ := command["ConditionExpression"].(string); expression == "" {
		t.Errorf("ConditionExpression is empty")
	}
	names, ok := command["ExpressionAttributeNames"].(map[string]string)
	if !ok || !containsValue(names, "status") {
		t.Errorf("ExpressionAttributeNames = %#v, want schema-resolved status", command["ExpressionAttributeNames"])
	}
	key, ok := command["Key"].(map[string]types.AttributeValue)
	if !ok {
		t.Fatalf("Key = %T, want DynamoDB attribute map", command["Key"])
	}
	if got := avStr(key["pk"]); got != "User#u-check" {
		t.Errorf("Key[pk] = %q, want User#u-check", got)
	}
	if got := avStr(key["sk"]); got != "User#" {
		t.Errorf("Key[sk] = %q, want User#", got)
	}
}

func TestModelCheckRequiresTransactionAndCondition(t *testing.T) {
	tbl, _ := makeTable(t, "CheckValidationTable", DefaultSchema, false)
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}

	if _, err := model.Check(bg(), ot.Item{"id": "u-check"}, &ot.Params{Where: `${status} = {active}`}); err == nil {
		t.Fatal("Check without transaction succeeded, want argument error")
	}

	for _, where := range []string{"", "   "} {
		t.Run(strings.TrimSpace(where), func(t *testing.T) {
			_, err := model.Check(bg(), ot.Item{"id": "u-check"}, &ot.Params{
				Transaction: map[string]any{},
				Where:       where,
			})
			if err == nil {
				t.Fatal("Check without condition succeeded, want argument error")
			}
		})
	}
}

func TestModelCheckAllowsExistsConditions(t *testing.T) {
	tbl, _ := makeTable(t, "CheckExistsTable", DefaultSchema, false)
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprintf("exists_%t", exists), func(t *testing.T) {
			tx := map[string]any{}
			if _, err := model.Check(bg(), ot.Item{"id": "u-check"}, &ot.Params{
				Transaction: tx,
				Exists:      &exists,
			}); err != nil {
				t.Fatalf("Check: %v", err)
			}
			items := tx["TransactItems"].([]any)
			entry := items[0].(map[string]any)
			command := entry["ConditionCheck"].(ot.Item)
			if command["ConditionExpression"] == "" {
				t.Fatal("ConditionExpression is empty")
			}
		})
	}
}

func TestModelCheckCombinesExistsAndWhereConditions(t *testing.T) {
	tbl, _ := makeTable(t, "CheckCombinedConditionTable", DefaultSchema, false)
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	tx := map[string]any{}
	exists := false
	if _, err := model.Check(bg(), ot.Item{"id": "u-check"}, &ot.Params{
		Transaction: tx,
		Exists:      &exists,
		Where:       `${status} = {active}`,
	}); err != nil {
		t.Fatalf("Check: %v", err)
	}
	items := tx["TransactItems"].([]any)
	command := items[0].(map[string]any)["ConditionCheck"].(ot.Item)
	condition, _ := command["ConditionExpression"].(string)
	if !strings.Contains(condition, "attribute_not_exists") {
		t.Errorf("ConditionExpression = %q, want Exists=false condition", condition)
	}
	if !strings.Contains(condition, " = ") {
		t.Errorf("ConditionExpression = %q, want Where comparison too", condition)
	}
	if names, ok := command["ExpressionAttributeNames"].(map[string]string); !ok || !containsValue(names, "status") {
		t.Errorf("ExpressionAttributeNames = %#v, want Where condition on status", command["ExpressionAttributeNames"])
	}
	if values, ok := command["ExpressionAttributeValues"].(map[string]types.AttributeValue); !ok || len(values) == 0 {
		t.Errorf("ExpressionAttributeValues = %#v, want Where condition value", command["ExpressionAttributeValues"])
	}
}

func TestTypedTransactionCheckFailureLeavesAccumulatorUnchanged(t *testing.T) {
	tbl, _ := makeTable(t, "CheckRollbackTable", DefaultSchema, false)
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	tx := ot.NewTransaction()
	params := transactionParams(t, tx)
	if _, err := model.Create(bg(), ot.Item{"id": "u-before"}, params); err != nil {
		t.Fatalf("accumulate create: %v", err)
	}
	before := len(params.Transaction["TransactItems"].([]any))
	_, err = tx.Check(bg(), model, ot.Item{}, &ot.Params{Where: `${status} = {active}`})
	if err == nil {
		t.Fatal("Check without key succeeded, want missing-key error")
	}
	after := len(params.Transaction["TransactItems"].([]any))
	if after != before {
		t.Fatalf("failed Check changed TransactItems from %d to %d", before, after)
	}
}

func TestTypedTransactionComposesModelOperationsAcrossTables(t *testing.T) {
	primary, client := makeTable(t, "PrimaryTransactionTable", DefaultSchema, false)
	client.tables["AuthSessionTransactionTable"] = map[string]map[string]types.AttributeValue{}
	authSessions, err := ot.NewTable(ot.TableParams{
		Name:   "AuthSessionTransactionTable",
		Client: client,
		Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatalf("NewTable auth sessions: %v", err)
	}
	primaryUser, err := primary.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel primary: %v", err)
	}
	authUser, err := authSessions.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel auth sessions: %v", err)
	}

	if _, err := authUser.Create(bg(), ot.Item{"id": "u-update", "status": "idle"}, nil); err != nil {
		t.Fatalf("seed auth session: %v", err)
	}
	if _, err := authUser.Create(bg(), ot.Item{"id": "u-check", "status": "active"}, nil); err != nil {
		t.Fatalf("seed condition-check item: %v", err)
	}
	if _, err := primaryUser.Create(bg(), ot.Item{"id": "u-remove"}, nil); err != nil {
		t.Fatalf("seed primary item: %v", err)
	}

	tx := ot.NewTransaction()
	sharedParams := transactionParams(t, tx)
	if _, err := primaryUser.Create(bg(), ot.Item{"id": "u-create"}, sharedParams); err != nil {
		t.Fatalf("accumulate create: %v", err)
	}
	updateParams, err := tx.Params(sharedParams)
	if err != nil {
		t.Fatalf("update Params after Create: %v", err)
	}
	if _, err := authUser.Update(bg(), ot.Item{"id": "u-update", "status": "active"}, updateParams); err != nil {
		t.Fatalf("accumulate update: %v", err)
	}
	removeParams, err := tx.Params(updateParams)
	if err != nil {
		t.Fatalf("remove Params after Update: %v", err)
	}
	if _, err := primaryUser.Remove(bg(), ot.Item{"id": "u-remove"}, removeParams); err != nil {
		t.Fatalf("accumulate remove: %v", err)
	}
	if _, err := tx.Check(bg(), authUser, ot.Item{"id": "u-check"}, &ot.Params{Where: `${status} = {active}`}); err != nil {
		t.Fatalf("accumulate check: %v", err)
	}

	params := sharedParams
	items, ok := params.Transaction["TransactItems"].([]any)
	if !ok || len(items) != 4 {
		t.Fatalf("TransactItems = %#v, want four operations", params.Transaction["TransactItems"])
	}
	wantOps := []string{"Put", "Update", "Delete", "ConditionCheck"}
	wantTables := []string{
		"PrimaryTransactionTable",
		"AuthSessionTransactionTable",
		"PrimaryTransactionTable",
		"AuthSessionTransactionTable",
	}
	for i, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("transaction item %d = %T, want map[string]any", i, raw)
		}
		command, ok := entry[wantOps[i]].(ot.Item)
		if !ok {
			t.Fatalf("transaction item %d = %#v, want %s", i, entry, wantOps[i])
		}
		if got := command["TableName"]; got != wantTables[i] {
			t.Errorf("transaction item %d TableName = %v, want %s", i, got, wantTables[i])
		}
	}

	if _, err := tx.Write(bg(), primary, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	created, err := primaryUser.Get(bg(), ot.Item{"id": "u-create"}, nil)
	if err != nil || created == nil {
		t.Fatalf("created item = %#v, err %v; want item", created, err)
	}
	updated, err := authUser.Get(bg(), ot.Item{"id": "u-update"}, nil)
	if err != nil || updated == nil || updated["status"] != "active" {
		t.Fatalf("updated auth session = %#v, err %v; want status active", updated, err)
	}
	removed, err := primaryUser.Get(bg(), ot.Item{"id": "u-remove"}, nil)
	if err != nil || removed != nil {
		t.Fatalf("removed item = %#v, err %v; want nil", removed, err)
	}
}

func TestTypedTransactionBuildsOneCrossTableDynamoRequest(t *testing.T) {
	client := &transactInputCapture{}
	primary, err := ot.NewTable(ot.TableParams{
		Name:   "PrimaryRequestTable",
		Client: client,
		Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatalf("NewTable primary: %v", err)
	}
	authSessions, err := ot.NewTable(ot.TableParams{
		Name:   "AuthSessionRequestTable",
		Client: client,
		Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatalf("NewTable auth sessions: %v", err)
	}
	primaryUser, err := primary.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel primary: %v", err)
	}
	authUser, err := authSessions.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel auth sessions: %v", err)
	}

	tx := ot.NewTransaction()
	sharedParams := transactionParams(t, tx)
	if _, err := primaryUser.Create(bg(), ot.Item{"id": "u-create"}, sharedParams); err != nil {
		t.Fatalf("accumulate create: %v", err)
	}
	if _, err := authUser.Update(bg(), ot.Item{"id": "u-update", "status": "active"}, sharedParams); err != nil {
		t.Fatalf("accumulate update: %v", err)
	}
	if _, err := primaryUser.Remove(bg(), ot.Item{"id": "u-remove"}, sharedParams); err != nil {
		t.Fatalf("accumulate remove: %v", err)
	}
	if _, err := tx.Check(bg(), authUser, ot.Item{"id": "u-check"}, &ot.Params{Where: `${status} = {active}`}); err != nil {
		t.Fatalf("accumulate check: %v", err)
	}
	if _, err := tx.Write(bg(), primary, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if client.input == nil {
		t.Fatal("TransactWriteItems was not called")
	}
	if got := len(client.input.TransactItems); got != 4 {
		t.Fatalf("TransactItems length = %d, want 4", got)
	}
	items := client.input.TransactItems
	if items[0].Put == nil || items[1].Update == nil || items[2].Delete == nil || items[3].ConditionCheck == nil {
		t.Fatalf("TransactItems have unexpected operation shapes: %#v", items)
	}
	if got := *items[0].Put.TableName; got != "PrimaryRequestTable" {
		t.Errorf("Put.TableName = %q, want PrimaryRequestTable", got)
	}
	if got := *items[1].Update.TableName; got != "AuthSessionRequestTable" {
		t.Errorf("Update.TableName = %q, want AuthSessionRequestTable", got)
	}
	if got := *items[2].Delete.TableName; got != "PrimaryRequestTable" {
		t.Errorf("Delete.TableName = %q, want PrimaryRequestTable", got)
	}
	if got := *items[3].ConditionCheck.TableName; got != "AuthSessionRequestTable" {
		t.Errorf("ConditionCheck.TableName = %q, want AuthSessionRequestTable", got)
	}
	if items[3].ConditionCheck.ConditionExpression == nil || *items[3].ConditionCheck.ConditionExpression == "" {
		t.Error("ConditionCheck.ConditionExpression is empty")
	}
}

func TestTypedTransactionUniqueOperationsUseTheirOwningTable(t *testing.T) {
	client := &transactInputCapture{}
	primary, err := ot.NewTable(ot.TableParams{
		Name:   "PrimaryUniqueTable",
		Client: client,
		Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatalf("NewTable primary: %v", err)
	}
	authSessions, err := ot.NewTable(ot.TableParams{
		Name:   "AuthSessionUniqueTable",
		Client: client,
		Schema: UniqueSchema,
	})
	if err != nil {
		t.Fatalf("NewTable auth sessions: %v", err)
	}
	primaryUser, err := primary.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel primary: %v", err)
	}
	authUser, err := authSessions.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel auth sessions: %v", err)
	}
	tx := ot.NewTransaction()
	params := transactionParams(t, tx)
	if _, err := primaryUser.Create(bg(), ot.Item{"id": "u-primary"}, params); err != nil {
		t.Fatalf("accumulate primary create: %v", err)
	}
	if _, err := authUser.Create(bg(), ot.Item{"name": "Session User", "email": "session@example.com"}, params); err != nil {
		t.Fatalf("accumulate unique auth create: %v", err)
	}
	if _, err := tx.Write(bg(), primary, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if client.input == nil {
		t.Fatal("TransactWriteItems was not called")
	}
	primaryCount, authCount := 0, 0
	for i, item := range client.input.TransactItems {
		if item.Put == nil {
			t.Fatalf("TransactItems[%d] is not a Put: %#v", i, item)
		}
		if got := *item.Put.TableName; got == "PrimaryUniqueTable" {
			primaryCount++
		} else if got == "AuthSessionUniqueTable" {
			authCount++
		} else {
			t.Errorf("TransactItems[%d] TableName = %q, want one of the source tables", i, got)
		}
	}
	if primaryCount != 1 || authCount < 2 {
		t.Errorf("table operation counts: primary=%d auth=%d, want primary=1 and auth unique + model writes", primaryCount, authCount)
	}
}

func TestTypedTransactionUniqueUpdateUsesItsOwningTable(t *testing.T) {
	baseClient := newFullMock()
	baseClient.tables["PrimaryUniqueUpdateTable"] = map[string]map[string]types.AttributeValue{}
	baseClient.tables["AuthSessionUniqueUpdateTable"] = map[string]map[string]types.AttributeValue{}
	client := &transactInputCapture{DynamoClient: baseClient}
	primary, err := ot.NewTable(ot.TableParams{
		Name:   "PrimaryUniqueUpdateTable",
		Client: client,
		Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatalf("NewTable primary: %v", err)
	}
	authSessions, err := ot.NewTable(ot.TableParams{
		Name:   "AuthSessionUniqueUpdateTable",
		Client: client,
		Schema: UniqueSchema,
	})
	if err != nil {
		t.Fatalf("NewTable auth sessions: %v", err)
	}
	primaryUser, err := primary.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel primary: %v", err)
	}
	authUser, err := authSessions.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel auth sessions: %v", err)
	}
	if _, err := authUser.Create(bg(), ot.Item{"name": "Session User", "email": "old@example.com"}, nil); err != nil {
		t.Fatalf("seed auth user: %v", err)
	}
	tx := ot.NewTransaction()
	params := transactionParams(t, tx)
	if _, err := primaryUser.Create(bg(), ot.Item{"id": "u-primary"}, params); err != nil {
		t.Fatalf("accumulate primary create: %v", err)
	}
	if _, err := authUser.Update(bg(), ot.Item{"name": "Session User", "email": "new@example.com"}, params); err != nil {
		t.Fatalf("accumulate unique update: %v", err)
	}
	if _, err := tx.Write(bg(), primary, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	primaryCount, authCount := 0, 0
	for i, item := range client.input.TransactItems {
		var tableName *string
		switch {
		case item.Put != nil:
			tableName = item.Put.TableName
		case item.Update != nil:
			tableName = item.Update.TableName
		case item.Delete != nil:
			tableName = item.Delete.TableName
		default:
			t.Fatalf("TransactItems[%d] has unsupported operation: %#v", i, item)
		}
		if got := aws.ToString(tableName); got == "PrimaryUniqueUpdateTable" {
			primaryCount++
		} else if got == "AuthSessionUniqueUpdateTable" {
			authCount++
		} else {
			t.Errorf("TransactItems[%d] TableName = %q, want one of the source tables", i, got)
		}
	}
	if primaryCount != 1 || authCount < 4 {
		t.Errorf("table operation counts: primary=%d auth=%d, want primary=1 and auth unique + model operations", primaryCount, authCount)
	}
}

func TestTypedTransactionWriteDoesNotConsumeOperations(t *testing.T) {
	client := &transactInputCapture{}
	tbl, err := ot.NewTable(ot.TableParams{
		Name:   "ReplayTransactionTable",
		Client: client,
		Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	tx := ot.NewTransaction()
	if _, err := model.Create(bg(), ot.Item{"id": "u-replay"}, transactionParams(t, tx)); err != nil {
		t.Fatalf("accumulate create: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := tx.Write(bg(), tbl, nil); err != nil {
			t.Fatalf("Write %d: %v", i+1, err)
		}
	}
	if client.calls != 2 || len(client.requests) != 2 {
		t.Fatalf("TransactWriteItems calls = %d, want 2", client.calls)
	}
	for i, request := range client.requests {
		if len(request.TransactItems) != 1 || request.TransactItems[0].Put == nil {
			t.Errorf("request %d lost accumulated operation: %#v", i+1, request.TransactItems)
		}
	}
}

func TestTypedTransactionFailedCheckDoesNotApplyWrites(t *testing.T) {
	baseClient := newFullMock()
	baseClient.tables["CheckFailureTable"] = map[string]map[string]types.AttributeValue{}
	client := &transactInputCapture{
		DynamoClient: baseClient,
		err:          &types.TransactionCanceledException{Message: aws.String("condition failed")},
	}
	tbl, err := ot.NewTable(ot.TableParams{
		Name:   "CheckFailureTable",
		Client: client,
		Schema: DefaultSchema,
	})
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if _, err := model.Create(bg(), ot.Item{"id": "u-existing", "status": "idle"}, nil); err != nil {
		t.Fatalf("seed existing item: %v", err)
	}
	client.err = nil
	tx := ot.NewTransaction()
	params := transactionParams(t, tx)
	if _, err := model.Create(bg(), ot.Item{"id": "u-new"}, params); err != nil {
		t.Fatalf("accumulate create: %v", err)
	}
	if _, err := model.Update(bg(), ot.Item{"id": "u-existing", "status": "active"}, params); err != nil {
		t.Fatalf("accumulate update: %v", err)
	}
	if _, err := tx.Check(bg(), model, ot.Item{"id": "u-check"}, &ot.Params{Where: `${status} = {active}`}); err != nil {
		t.Fatalf("accumulate check: %v", err)
	}
	client.err = &types.TransactionCanceledException{Message: aws.String("condition failed")}
	if _, err := tx.Write(bg(), tbl, nil); err == nil {
		t.Fatal("Write with cancelled ConditionCheck succeeded")
	}
	if client.calls != 1 || client.input == nil || len(client.input.TransactItems) != 3 {
		t.Fatalf("TransactWriteItems calls=%d request=%#v, want one mixed transaction", client.calls, client.input)
	}
	if client.input.TransactItems[2].ConditionCheck == nil {
		t.Fatal("failed transaction request does not contain ConditionCheck")
	}
	newItem, err := model.Get(bg(), ot.Item{"id": "u-new"}, nil)
	if err != nil || newItem != nil {
		t.Fatalf("new item after cancelled transaction = %#v, err %v; want nil", newItem, err)
	}
	existing, err := model.Get(bg(), ot.Item{"id": "u-existing"}, nil)
	if err != nil || existing == nil || existing["status"] != "idle" {
		t.Fatalf("existing item after cancelled transaction = %#v, err %v; want status idle", existing, err)
	}
}

func TestUniqueCreatePreservesTransactionCancellationCause(t *testing.T) {
	cause := &types.TransactionCanceledException{Message: aws.String("condition failed")}
	client := &transactInputCapture{err: cause}
	tbl, err := ot.NewTable(ot.TableParams{
		Name:   "UniqueCancellationTable",
		Client: client,
		Schema: UniqueSchema,
	})
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	_, err = model.Create(bg(), ot.Item{"name": "Session User", "email": "session@example.com"}, nil)
	if err == nil {
		t.Fatal("Create succeeded, want transaction cancellation")
	}
	var got *types.TransactionCanceledException
	if !errors.As(err, &got) || got != cause {
		t.Fatalf("errors.As cause = %p, want original cancellation %p; err: %v", got, cause, err)
	}
	var tableErr *ot.OneTableError
	if !errors.As(err, &tableErr) || tableErr.Code != ot.ErrUnique {
		t.Fatalf("error = %#v, want unique OneTableError retaining cancellation cause", err)
	}
}

func TestUniqueUpdatePreservesTransactionCancellationCause(t *testing.T) {
	cause := &types.TransactionCanceledException{Message: aws.String("condition failed")}
	baseClient := newFullMock()
	baseClient.tables["UniqueUpdateCancellationTable"] = map[string]map[string]types.AttributeValue{}
	client := &transactInputCapture{DynamoClient: baseClient}
	tbl, err := ot.NewTable(ot.TableParams{
		Name:   "UniqueUpdateCancellationTable",
		Client: client,
		Schema: UniqueSchema,
	})
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if _, err := model.Create(bg(), ot.Item{"name": "Session User", "email": "old@example.com"}, nil); err != nil {
		t.Fatalf("seed unique user: %v", err)
	}
	client.err = cause
	_, err = model.Update(bg(), ot.Item{"name": "Session User", "email": "new@example.com"}, nil)
	if err == nil {
		t.Fatal("Update succeeded, want transaction cancellation")
	}
	var got *types.TransactionCanceledException
	if !errors.As(err, &got) || got != cause {
		t.Fatalf("errors.As cause = %p, want original cancellation %p; err: %v", got, cause, err)
	}
}

func TestTypedTransactionUniqueConflictReturnsWriteCancellation(t *testing.T) {
	cause := &types.TransactionCanceledException{Message: aws.String("unique condition failed")}
	client := &transactInputCapture{err: cause}
	tbl, err := ot.NewTable(ot.TableParams{
		Name:   "TypedUniqueConflictTable",
		Client: client,
		Schema: UniqueSchema,
	})
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	tx := ot.NewTransaction()
	if _, err := model.Create(bg(), ot.Item{"name": "Session User", "email": "same@example.com"}, transactionParams(t, tx)); err != nil {
		t.Fatalf("accumulate unique create: %v", err)
	}
	_, err = tx.Write(bg(), tbl, nil)
	if err == nil {
		t.Fatal("Write succeeded, want unique constraint cancellation")
	}
	var tableErr *ot.OneTableError
	if !errors.As(err, &tableErr) || tableErr.Code != ot.ErrRuntime {
		t.Fatalf("Write error = %#v, want transaction runtime error", err)
	}
	var got *types.TransactionCanceledException
	if !errors.As(err, &got) || got != cause {
		t.Fatalf("errors.As cause = %p, want original cancellation %p; err: %v", got, cause, err)
	}
}

func TestTypedTransactionValidatesReceiverAndExecutor(t *testing.T) {
	var nilTx *ot.Transaction
	if _, err := nilTx.Check(bg(), nil, nil, nil); err == nil {
		t.Fatal("Check on nil transaction succeeded, want argument error")
	}
	if _, err := nilTx.Write(bg(), nil, nil); err == nil {
		t.Fatal("Write on nil transaction succeeded, want argument error")
	}

	tx := ot.NewTransaction()
	if _, err := tx.Write(bg(), nil, nil); err == nil {
		t.Fatal("Write with nil table succeeded, want argument error")
	}
	emptyTable, _ := makeTable(t, "EmptyTransactionTable", DefaultSchema, false)
	if _, err := tx.Write(bg(), emptyTable, nil); err == nil {
		t.Fatal("Write with no operations succeeded, want argument error")
	}
	invalidTx := ot.NewTransaction()
	transactionParams(t, invalidTx).Transaction["TransactItems"] = []any{"not an operation"}
	if _, err := invalidTx.Write(bg(), emptyTable, nil); err == nil {
		t.Fatal("Write with an invalid operation succeeded, want argument error")
	}
	invalidCommandTx := ot.NewTransaction()
	transactionParams(t, invalidCommandTx).Transaction["TransactItems"] = []any{
		map[string]any{"Update": map[string]any{}},
	}
	if _, err := invalidCommandTx.Write(bg(), emptyTable, &ot.Params{Execute: falsePtr()}); err == nil {
		t.Fatal("Write with an incomplete command succeeded, want argument error")
	}
}

func TestTypedTransactionParamsPreservesModifiers(t *testing.T) {
	tx := ot.NewTransaction()
	params, err := tx.Params(&ot.Params{Where: `${status} = {active}`, Parse: true})
	if err != nil {
		t.Fatalf("Params: %v", err)
	}
	if params.Transaction == nil {
		t.Fatal("Params.Transaction is nil")
	}
	if params.Where != `${status} = {active}` || !params.Parse {
		t.Fatalf("Params modifiers were not preserved: %#v", params)
	}
	var nilTx *ot.Transaction
	if _, err := nilTx.Params(nil); err == nil {
		t.Fatal("Params on nil transaction succeeded, want argument error")
	}
	var zeroTx ot.Transaction
	zeroParams, err := zeroTx.Params(nil)
	if err != nil || zeroParams.Transaction == nil {
		t.Fatalf("zero-value transaction Params = %#v, err %v", zeroParams, err)
	}
}

func TestTypedTransactionParamsCopiesCallerModifiers(t *testing.T) {
	tx := ot.NewTransaction()
	input := &ot.Params{
		Where:  `${status} = {active}`,
		Set:    map[string]string{"name": "original"},
		Remove: []string{"age"},
	}
	first, err := tx.Params(input)
	if err != nil {
		t.Fatalf("Params: %v", err)
	}
	if input.Transaction != nil {
		t.Fatal("Params mutated the caller's Transaction field")
	}
	first.Where = `${status} = {inactive}`
	first.Set["name"] = "changed"
	first.Remove[0] = "changed"
	second, err := tx.Params(input)
	if err != nil {
		t.Fatalf("second Params: %v", err)
	}
	if first == second {
		t.Fatal("Params returned the same pointer, want independent operation params")
	}
	if second.Where != `${status} = {active}` {
		t.Fatalf("second Where = %q, want original modifier", second.Where)
	}
	if second.Set["name"] != "original" || second.Remove[0] != "age" {
		t.Fatalf("mutable modifiers leaked across operation params: Set=%v Remove=%v", second.Set, second.Remove)
	}
}

func TestLegacyMapTransactionCompatibility(t *testing.T) {
	tbl, _ := makeTable(t, "LegacyTransactionTable", DefaultSchema, false)
	model, err := tbl.GetModel("User")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}

	transaction := map[string]any{}
	if _, err := model.Create(bg(), ot.Item{"id": "u-legacy"}, &ot.Params{Transaction: transaction}); err != nil {
		t.Fatalf("accumulate legacy create: %v", err)
	}
	items, ok := transaction["TransactItems"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("legacy TransactItems = %#v, want one operation", transaction["TransactItems"])
	}
	if _, ok := items[0].(map[string]any)["Put"]; !ok {
		t.Fatalf("legacy transaction item = %#v, want Put", items[0])
	}
	if _, err := tbl.Transact(bg(), "write", transaction, nil); err != nil {
		t.Fatalf("legacy Transact write: %v", err)
	}
	created, err := model.Get(bg(), ot.Item{"id": "u-legacy"}, nil)
	if err != nil || created == nil {
		t.Fatalf("legacy created item = %#v, err %v; want item", created, err)
	}
}

func containsValue(values map[string]string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func transactionParams(t *testing.T, tx *ot.Transaction) *ot.Params {
	t.Helper()
	params, err := tx.Params(nil)
	if err != nil {
		t.Fatalf("Transaction.Params: %v", err)
	}
	return params
}

type transactInputCapture struct {
	ot.DynamoClient
	input    *ddb.TransactWriteItemsInput
	requests []*ddb.TransactWriteItemsInput
	err      error
	calls    int
}

func (c *transactInputCapture) TransactWriteItems(ctx context.Context, input *ddb.TransactWriteItemsInput, options ...func(*ddb.Options)) (*ddb.TransactWriteItemsOutput, error) {
	c.input = input
	c.requests = append(c.requests, input)
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	if c.DynamoClient != nil {
		return c.DynamoClient.TransactWriteItems(ctx, input, options...)
	}
	return &ddb.TransactWriteItemsOutput{}, nil
}
