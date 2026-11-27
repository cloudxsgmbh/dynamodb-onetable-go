package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ot "github.com/cloudxsgmbh/dynamodb-onetable-go"
)

type scriptedScanClient struct {
	ot.DynamoClient
	responses  []*ddb.ScanOutput
	queries    []*ddb.QueryOutput
	scanErrors []error
	err        error
	calls      []*ddb.ScanInput
	queryCalls []*ddb.QueryInput
}

func (c *scriptedScanClient) Scan(_ context.Context, input *ddb.ScanInput, _ ...func(*ddb.Options)) (*ddb.ScanOutput, error) {
	c.calls = append(c.calls, input)
	if callIndex := len(c.calls) - 1; callIndex < len(c.scanErrors) && c.scanErrors[callIndex] != nil {
		return nil, c.scanErrors[callIndex]
	}
	if c.err != nil {
		return nil, c.err
	}
	if len(c.responses) == 0 {
		return nil, fmt.Errorf("unexpected Scan call %d", len(c.calls))
	}
	response := c.responses[0]
	c.responses = c.responses[1:]
	if input.Limit != nil && (len(response.Items) > int(*input.Limit) || response.Count > *input.Limit || response.ScannedCount > *input.Limit) {
		return nil, fmt.Errorf("scripted Scan response exceeds evaluation Limit %d", *input.Limit)
	}
	return response, nil
}

func (c *scriptedScanClient) Query(_ context.Context, input *ddb.QueryInput, _ ...func(*ddb.Options)) (*ddb.QueryOutput, error) {
	c.queryCalls = append(c.queryCalls, input)
	if c.err != nil {
		return nil, c.err
	}
	if len(c.queries) == 0 {
		return nil, fmt.Errorf("unexpected Query call %d", len(c.queryCalls))
	}
	response := c.queries[0]
	c.queries = c.queries[1:]
	if input.Limit != nil && (len(response.Items) > int(*input.Limit) || response.Count > *input.Limit || response.ScannedCount > *input.Limit) {
		return nil, fmt.Errorf("scripted Query response exceeds evaluation Limit %d", *input.Limit)
	}
	return response, nil
}

func scanItem(t *testing.T, id string) map[string]types.AttributeValue {
	t.Helper()
	item, err := attributevalue.MarshalMap(map[string]string{"pk": "item#" + id, "sk": "item#" + id, "status": "active"})
	if err != nil {
		t.Fatalf("marshal scan item: %v", err)
	}
	return item
}

func scanKey(t *testing.T, id string) map[string]types.AttributeValue {
	t.Helper()
	key, err := attributevalue.MarshalMap(map[string]string{"pk": "item#" + id, "sk": "item#" + id})
	if err != nil {
		t.Fatalf("marshal scan key: %v", err)
	}
	return key
}

func newScanTable(t *testing.T, client ot.DynamoClient) *ot.Table {
	t.Helper()
	tbl, err := ot.NewTable(ot.TableParams{Name: "ScanSemantics", Client: client, Schema: DefaultSchema})
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

func assertItemIDs(t *testing.T, items []ot.Item, want ...string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("got %d items (%v), want IDs %v", len(items), items, want)
	}
	for i, id := range want {
		if got := items[i]["pk"]; got != "item#"+id {
			t.Errorf("item %d pk = %v, want %q", i, got, "item#"+id)
		}
	}
}

// Continuation-key checks intentionally exercise the separate cursor fix;
// this branch's suite fails these assertions until both diffs are integrated.
func assertScanCall(t *testing.T, call *ddb.ScanInput, wantLimit int32, wantStartKey string) {
	t.Helper()
	if call.Limit == nil || *call.Limit != wantLimit {
		t.Errorf("Scan Limit = %v, want %d evaluated items", call.Limit, wantLimit)
	}
	gotStartKey := ""
	if call.ExclusiveStartKey != nil {
		gotStartKey = avStr(call.ExclusiveStartKey["pk"])
	}
	if gotStartKey != wantStartKey {
		t.Errorf("Scan ExclusiveStartKey.pk = %q, want %q", gotStartKey, wantStartKey)
	}
}

func TestScan_LimitReturnsExactNumberOfMatchingItems(t *testing.T) {
	client := &scriptedScanClient{responses: []*ddb.ScanOutput{
		{Items: []map[string]types.AttributeValue{scanItem(t, "1"), scanItem(t, "2")}, Count: 2, ScannedCount: 3, LastEvaluatedKey: scanKey(t, "3")},
		{Items: []map[string]types.AttributeValue{scanItem(t, "4")}, Count: 1, ScannedCount: 1, LastEvaluatedKey: scanKey(t, "4")},
		{Items: []map[string]types.AttributeValue{scanItem(t, "5"), scanItem(t, "6")}, Count: 2, ScannedCount: 2},
	}}
	tbl := newScanTable(t, client)
	stats := &ot.Stats{}

	result, err := tbl.ScanItems(context.Background(), ot.Item{}, &ot.Params{Limit: 3, Where: "${status} = {active}", Stats: stats})
	if err != nil {
		t.Fatalf("ScanItems: %v", err)
	}
	assertItemIDs(t, result.Items, "1", "2", "4")
	if result.Next == nil || result.Next["pk"] != "item#4" {
		t.Fatalf("ScanItems Next = %v, want continuation key for item 4", result.Next)
	}
	if got := len(client.calls); got != 2 {
		t.Fatalf("initial Scan called %d times, want 2 pages", got)
	}
	assertScanCall(t, client.calls[0], 3, "")
	assertScanCall(t, client.calls[1], 1, "item#3")
	if client.calls[0].FilterExpression == nil || client.calls[1].FilterExpression == nil {
		t.Fatal("expected FilterExpression on both partial Scan pages")
	}
	if stats.Count != 3 || stats.Scanned != 4 {
		t.Errorf("Stats after first result = %+v, want Count=3 Scanned=4", stats)
	}

	resumed, err := tbl.ScanItems(context.Background(), ot.Item{}, &ot.Params{Limit: 3, Next: result.Next, Stats: stats})
	if err != nil {
		t.Fatalf("resume ScanItems: %v", err)
	}
	assertItemIDs(t, resumed.Items, "5", "6")
	assertScanCall(t, client.calls[2], 3, "item#4")
	combined := append(append([]ot.Item{}, result.Items...), resumed.Items...)
	assertItemIDs(t, combined, "1", "2", "4", "5", "6")
	if stats.Count != 5 || stats.Scanned != 6 {
		t.Errorf("Stats after resume = %+v, want Count=5 Scanned=6", stats)
	}
}

func TestScan_CountLimitUsesRemainingResultBudget(t *testing.T) {
	client := &scriptedScanClient{responses: []*ddb.ScanOutput{
		{Count: 2, ScannedCount: 3, LastEvaluatedKey: scanKey(t, "3")},
		{Count: 1, ScannedCount: 1},
	}}
	tbl := newScanTable(t, client)
	stats := &ot.Stats{}

	result, err := tbl.ScanItems(context.Background(), ot.Item{}, &ot.Params{Limit: 3, Count: true, Where: "${status} = {active}", Stats: stats})
	if err != nil {
		t.Fatalf("count ScanItems: %v", err)
	}
	if result.Count != 3 || len(result.Items) != 0 {
		t.Fatalf("count result = %+v, want Count=3 and no returned items", result)
	}
	if len(client.calls) != 2 {
		t.Fatalf("count Scan called %d times, want 2 pages", len(client.calls))
	}
	assertScanCall(t, client.calls[0], 3, "")
	assertScanCall(t, client.calls[1], 1, "item#3")
	if stats.Count != 3 || stats.Scanned != 4 {
		t.Errorf("count Stats = %+v, want Count=3 Scanned=4", stats)
	}
}

func TestScan_MaxPagesAlsoBoundsLimitBudget(t *testing.T) {
	for _, maxPages := range []int{1, 2} {
		t.Run(fmt.Sprintf("max-pages-%d", maxPages), func(t *testing.T) {
			client := &scriptedScanClient{responses: []*ddb.ScanOutput{
				{Items: []map[string]types.AttributeValue{scanItem(t, "1"), scanItem(t, "2")}, Count: 2, ScannedCount: 3, LastEvaluatedKey: scanKey(t, "3")},
				{Items: []map[string]types.AttributeValue{scanItem(t, "4")}, Count: 1, ScannedCount: 1, LastEvaluatedKey: scanKey(t, "4")},
			}}
			tbl := newScanTable(t, client)

			result, err := tbl.ScanItems(context.Background(), ot.Item{}, &ot.Params{Limit: 3, MaxPages: maxPages, Where: "${status} = {active}"})
			if err != nil {
				t.Fatalf("ScanItems: %v", err)
			}
			wantItems := 2
			if maxPages == 2 {
				wantItems = 3
			}
			if got := len(result.Items); got != wantItems {
				t.Fatalf("ScanItems returned %d items with MaxPages=%d, want %d", got, maxPages, wantItems)
			}
			if got := len(client.calls); got != maxPages {
				t.Fatalf("Scan called %d times with MaxPages=%d, want %d", got, maxPages, maxPages)
			}
			wantCursor := "item#3"
			if maxPages == 2 {
				wantCursor = "item#4"
			}
			if result.Next == nil || result.Next["pk"] != wantCursor {
				t.Errorf("Next = %v with MaxPages=%d, want cursor %q", result.Next, maxPages, wantCursor)
			}
			if client.calls[0].Limit == nil || *client.calls[0].Limit != 3 {
				t.Errorf("first Scan Limit = %v, want 3", client.calls[0].Limit)
			}
			if maxPages == 2 && (client.calls[1].Limit == nil || *client.calls[1].Limit != 1) {
				t.Errorf("second Scan Limit = %v, want remaining result budget 1", client.calls[1].Limit)
			}
		})
	}
}

func TestScan_ContinuesWhenFilteredPageHasContinuationKey(t *testing.T) {
	client := &scriptedScanClient{responses: []*ddb.ScanOutput{
		{Items: []map[string]types.AttributeValue{}, Count: 0, ScannedCount: 3, LastEvaluatedKey: scanKey(t, "3")},
		{Items: []map[string]types.AttributeValue{scanItem(t, "4")}, Count: 1, ScannedCount: 2},
	}}
	tbl := newScanTable(t, client)

	result, err := tbl.ScanItems(context.Background(), ot.Item{}, &ot.Params{Limit: 3, Where: "${status} = {active}"})
	if err != nil {
		t.Fatalf("ScanItems: %v", err)
	}
	if got := len(result.Items); got != 1 {
		t.Fatalf("ScanItems returned %d items, want 1 after the empty filtered page", got)
	}
	if got := len(client.calls); got != 2 {
		t.Fatalf("Scan called %d times, want another request after an empty page with LastEvaluatedKey", got)
	}
	assertScanCall(t, client.calls[0], 3, "")
	assertScanCall(t, client.calls[1], 3, "item#3")
}

func TestScan_PropagatesDynamoDBError(t *testing.T) {
	serviceErr := errors.New("AccessDeniedException: scan denied")
	client := &scriptedScanClient{err: serviceErr}
	tbl := newScanTable(t, client)

	_, err := tbl.ScanItems(context.Background(), ot.Item{}, nil)
	if !errors.Is(err, serviceErr) {
		t.Fatalf("ScanItems error = %v, want wrapped service error %v", err, serviceErr)
	}
	var tableErr *ot.OneTableError
	if !errors.As(err, &tableErr) || tableErr.Code != ot.ErrRuntime {
		t.Fatalf("ScanItems error = %T (%v), want RuntimeError wrapping service error", err, err)
	}
}

func TestScan_PropagatesErrorFromLaterPage(t *testing.T) {
	serviceErr := errors.New("ProvisionedThroughputExceededException: later page denied")
	client := &scriptedScanClient{
		responses: []*ddb.ScanOutput{{
			Items:            []map[string]types.AttributeValue{scanItem(t, "1"), scanItem(t, "2")},
			Count:            2,
			ScannedCount:     3,
			LastEvaluatedKey: scanKey(t, "3"),
		}},
		scanErrors: []error{nil, serviceErr},
	}
	tbl := newScanTable(t, client)

	result, err := tbl.ScanItems(context.Background(), ot.Item{}, &ot.Params{Limit: 3, Where: "${status} = {active}"})
	if result != nil {
		t.Errorf("ScanItems result = %v, want nil when later page fails", result)
	}
	if !errors.Is(err, serviceErr) {
		t.Fatalf("ScanItems error = %v, want wrapped later-page error %v", err, serviceErr)
	}
	if len(client.calls) != 2 {
		t.Fatalf("Scan calls = %d, want 2", len(client.calls))
	}
	if client.calls[1].Limit == nil || *client.calls[1].Limit != 1 {
		t.Errorf("failing Scan Limit = %v, want remaining result budget 1", client.calls[1].Limit)
	}
}

func TestQuery_LimitUsesRemainingResultBudget(t *testing.T) {
	client := &scriptedScanClient{queries: []*ddb.QueryOutput{
		{Items: []map[string]types.AttributeValue{scanItem(t, "1"), scanItem(t, "2")}, Count: 2, ScannedCount: 3, LastEvaluatedKey: scanKey(t, "3")},
		{Items: []map[string]types.AttributeValue{scanItem(t, "4")}, Count: 1, ScannedCount: 1, LastEvaluatedKey: scanKey(t, "4")},
	}}
	tbl := newScanTable(t, client)
	stats := &ot.Stats{}
	result, err := tbl.QueryItems(context.Background(), ot.Item{"pk": "item#"}, &ot.Params{Limit: 3, Where: "${status} = {active}", Stats: stats})
	if err != nil {
		t.Fatalf("QueryItems: %v", err)
	}
	assertItemIDs(t, result.Items, "1", "2", "4")
	if len(client.queryCalls) != 2 {
		t.Fatalf("Query called %d times, want 2 pages", len(client.queryCalls))
	}
	if client.queryCalls[0].Limit == nil || *client.queryCalls[0].Limit != 3 {
		t.Errorf("first Query Limit = %v, want 3", client.queryCalls[0].Limit)
	}
	if client.queryCalls[1].Limit == nil || *client.queryCalls[1].Limit != 1 {
		t.Errorf("second Query Limit = %v, want remaining budget 1", client.queryCalls[1].Limit)
	}
	if client.queryCalls[0].FilterExpression == nil || client.queryCalls[1].FilterExpression == nil {
		t.Fatal("expected FilterExpression on both partial Query pages")
	}
	if got := avStr(client.queryCalls[1].ExclusiveStartKey["pk"]); got != "item#3" {
		t.Errorf("second Query ExclusiveStartKey.pk = %q, want item#3", got)
	}
	if stats.Count != 3 || stats.Scanned != 4 {
		t.Errorf("Query Stats = %+v, want Count=3 Scanned=4", stats)
	}
}
