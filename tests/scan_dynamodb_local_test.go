package tests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

func TestScan_DynamoDBLocalSemantics(t *testing.T) {
	endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		t.Skip("set DYNAMODB_LOCAL_ENDPOINT to run DynamoDB Local integration coverage")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		t.Fatalf("invalid DYNAMODB_LOCAL_ENDPOINT %q", endpoint)
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatalf("refusing non-loopback DynamoDB endpoint %q; this test creates and deletes a table", endpoint)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := ddb.New(ddb.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "local", SecretAccessKey: "local", Source: "test"}, nil
		}),
	})
	tableName := fmt.Sprintf("scan-local-%d", time.Now().UnixNano())
	_, err = client.CreateTable(ctx, &ddb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("create local table: %v", err)
	}
	defer func() {
		_, err := client.DeleteTable(context.Background(), &ddb.DeleteTableInput{TableName: aws.String(tableName)})
		if err != nil {
			t.Errorf("delete local table: %v", err)
		}
	}()
	if err := ddb.NewTableExistsWaiter(client).Wait(ctx, &ddb.DescribeTableInput{TableName: aws.String(tableName)}, 30*time.Second); err != nil {
		t.Fatalf("wait for local table: %v", err)
	}

	for i := 0; i < 100; i++ {
		rare := "no"
		if i == 0 {
			rare = "yes"
		}
		_, err := client.PutItem(ctx, &ddb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]types.AttributeValue{
				"pk":   &types.AttributeValueMemberS{Value: fmt.Sprintf("item#%03d", i)},
				"sk":   &types.AttributeValueMemberS{Value: "row"},
				"rare": &types.AttributeValueMemberS{Value: rare},
			},
		})
		if err != nil {
			t.Fatalf("put local item %d: %v", i, err)
		}
	}

	schema := &ot.SchemaDef{
		Format:  "onetable:1.1.0",
		Version: "0.0.1",
		Indexes: map[string]*ot.IndexDef{"primary": {Hash: "pk", Sort: "sk"}},
	}
	tbl, err := ot.NewTable(ot.TableParams{Name: tableName, Client: client, Schema: schema})
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}

	// DynamoDB Scan does not guarantee order. Discover its local scan order,
	// then choose match values that make page one return 2/5 and page two
	// return 4/5. A fixed per-request Limit of 5 would therefore overrun 5.
	var orderedItems []map[string]types.AttributeValue
	var startKey map[string]types.AttributeValue
	for page := 0; page < 110; page++ {
		pageOut, err := client.Scan(ctx, &ddb.ScanInput{
			TableName:         aws.String(tableName),
			Limit:             aws.Int32(1),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			t.Fatalf("discover local Scan order page %d: %v", page+1, err)
		}
		orderedItems = append(orderedItems, pageOut.Items...)
		if pageOut.LastEvaluatedKey == nil {
			break
		}
		startKey = pageOut.LastEvaluatedKey
	}
	if len(orderedItems) < 14 {
		t.Fatalf("discovered %d local scan items, need at least 14", len(orderedItems))
	}
	matchPositions := map[int]bool{0: true, 1: true, 5: true, 6: true, 7: true, 8: true, 10: true, 11: true, 12: true, 13: true}
	for i, item := range orderedItems {
		match := "no"
		if matchPositions[i] {
			match = "yes"
		}
		item["match"] = &types.AttributeValueMemberS{Value: match}
		_, err := client.PutItem(ctx, &ddb.PutItemInput{TableName: aws.String(tableName), Item: item})
		if err != nil {
			t.Fatalf("set local match value %d: %v", i, err)
		}
	}

	firstPage, err := client.Scan(ctx, &ddb.ScanInput{
		TableName:                 aws.String(tableName),
		Limit:                     aws.Int32(5),
		FilterExpression:          aws.String("#match = :yes"),
		ExpressionAttributeNames:  map[string]string{"#match": "match"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":yes": &types.AttributeValueMemberS{Value: "yes"}},
	})
	if err != nil {
		t.Fatalf("preflight local filtered Scan page 1: %v", err)
	}
	secondPage, err := client.Scan(ctx, &ddb.ScanInput{
		TableName:                 aws.String(tableName),
		Limit:                     aws.Int32(5),
		ExclusiveStartKey:         firstPage.LastEvaluatedKey,
		FilterExpression:          aws.String("#match = :yes"),
		ExpressionAttributeNames:  map[string]string{"#match": "match"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":yes": &types.AttributeValueMemberS{Value: "yes"}},
	})
	if err != nil {
		t.Fatalf("preflight local filtered Scan page 2: %v", err)
	}
	if firstPage.Count != 2 || firstPage.ScannedCount != 5 || secondPage.Count != 4 || secondPage.ScannedCount != 5 {
		t.Fatalf("preflight page counts = (%d/%d, %d/%d), want (2/5, 4/5)", firstPage.Count, firstPage.ScannedCount, secondPage.Count, secondPage.ScannedCount)
	}

	result, err := tbl.ScanItems(ctx, ot.Item{}, &ot.Params{Limit: 5, Where: "${match} = {yes}"})
	if err != nil {
		t.Fatalf("filtered ScanItems: %v", err)
	}
	if got := len(result.Items); got != 5 {
		t.Fatalf("filtered ScanItems returned %d items, want exactly 5", got)
	}
	if result.Next == nil {
		t.Fatal("filtered ScanItems Next = nil, want continuation cursor")
	}

	resumed, err := tbl.ScanItems(ctx, ot.Item{}, &ot.Params{Limit: 3, Next: result.Next, Where: "${match} = {yes}"})
	if err != nil {
		t.Fatalf("resume filtered ScanItems: %v", err)
	}
	if got := len(resumed.Items); got != 3 {
		t.Fatalf("resumed ScanItems returned %d items, want 3", got)
	}
	seen := make(map[string]bool, len(result.Items)+len(resumed.Items))
	for _, item := range append(result.Items, resumed.Items...) {
		pk, _ := item["pk"].(string)
		if seen[pk] {
			t.Fatalf("item %q returned more than once across pages", pk)
		}
		seen[pk] = true
	}
	if len(seen) != 8 {
		t.Fatalf("combined filtered results contain %d unique items, want 8", len(seen))
	}

	// DynamoDB may return a page with no matches while still providing a
	// continuation key; the caller must keep scanning in that case.
	var rareStartKey map[string]types.AttributeValue
	emptyPageWithCursor := false
	rareCount := 0
	for page := 0; page < 110; page++ {
		pageOut, err := client.Scan(ctx, &ddb.ScanInput{
			TableName:                 aws.String(tableName),
			Limit:                     aws.Int32(1),
			FilterExpression:          aws.String("#rare = :yes"),
			ExpressionAttributeNames:  map[string]string{"#rare": "rare"},
			ExpressionAttributeValues: map[string]types.AttributeValue{":yes": &types.AttributeValueMemberS{Value: "yes"}},
			ExclusiveStartKey:         rareStartKey,
		})
		if err != nil {
			t.Fatalf("direct local Scan page %d: %v", page+1, err)
		}
		rareCount += len(pageOut.Items)
		if len(pageOut.Items) == 0 && pageOut.LastEvaluatedKey != nil {
			emptyPageWithCursor = true
		}
		if pageOut.LastEvaluatedKey == nil {
			break
		}
		rareStartKey = pageOut.LastEvaluatedKey
	}
	if rareCount != 1 {
		t.Fatalf("direct filtered Scan found %d rare items, want 1", rareCount)
	}
	if !emptyPageWithCursor {
		t.Fatal("DynamoDB Local did not produce the expected empty filtered page with a continuation key")
	}

	missingTable, err := ot.NewTable(ot.TableParams{Name: tableName + "-missing", Client: client, Schema: schema})
	if err != nil {
		t.Fatalf("NewTable for error case: %v", err)
	}
	_, err = missingTable.ScanItems(ctx, ot.Item{}, nil)
	var tableErr *ot.OneTableError
	if err == nil || !errors.As(err, &tableErr) || tableErr.Code != ot.ErrRuntime || !strings.Contains(err.Error(), "ResourceNotFoundException") {
		t.Fatalf("missing-table ScanItems error = %v, want wrapped ResourceNotFoundException", err)
	}
	var serviceErr *types.ResourceNotFoundException
	if !errors.As(err, &serviceErr) {
		t.Fatalf("missing-table error %T does not preserve the DynamoDB service cause", err)
	}
}
