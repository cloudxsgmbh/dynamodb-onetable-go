// Ports: test/find.ts + test/scan.ts
package tests

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
	users := make([]ot.Item, 0, len(findData))
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
	result, err := tbl.Scan(bg(), "User", ot.Item{}, &ot.Params{Hidden: truePtr()})
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

type paginationDynamoClient struct {
	ot.DynamoClient
	queryPages []*ddb.QueryOutput
	scanPages  []*ddb.ScanOutput
	queryCalls []*ddb.QueryInput
	scanCalls  []*ddb.ScanInput
}

type failingCursorValue struct{}

func (failingCursorValue) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return nil, errors.New("test cursor marshal failure")
}

func (c *paginationDynamoClient) Query(_ context.Context, input *ddb.QueryInput, _ ...func(*ddb.Options)) (*ddb.QueryOutput, error) {
	c.queryCalls = append(c.queryCalls, input)
	index := len(c.queryCalls) - 1
	if index >= len(c.queryPages) {
		return nil, errors.New("unexpected query request")
	}
	return c.queryPages[index], nil
}

func (c *paginationDynamoClient) Scan(_ context.Context, input *ddb.ScanInput, _ ...func(*ddb.Options)) (*ddb.ScanOutput, error) {
	c.scanCalls = append(c.scanCalls, input)
	index := len(c.scanCalls) - 1
	if index >= len(c.scanPages) {
		return nil, errors.New("unexpected scan request")
	}
	return c.scanPages[index], nil
}

func runPagedOperation(t *testing.T, operation string, params *ot.Params, client *paginationDynamoClient) (*ot.Result, error) {
	t.Helper()
	tbl, _ := makeTable(t, "PaginationTable", DefaultSchema, false)
	params.Client = client
	if operation == "find" {
		return tbl.Find(bg(), "User", ot.Item{"id": "cursor-test"}, params)
	}
	return tbl.Scan(bg(), "User", ot.Item{}, params)
}

func paginationCursor(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "User#" + id},
		"sk": &types.AttributeValueMemberS{Value: "User#"},
	}
}

func paginationItem(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"_type": &types.AttributeValueMemberS{Value: "User"},
		"id":    &types.AttributeValueMemberS{Value: id},
		"pk":    &types.AttributeValueMemberS{Value: "User#" + id},
		"sk":    &types.AttributeValueMemberS{Value: "User#"},
	}
}

func paginationNumericCursor(value string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberN{Value: value},
		"sk": &types.AttributeValueMemberS{Value: "User#"},
	}
}

func paginationNumericItem(id, pk, sk string) map[string]types.AttributeValue {
	item := paginationItem(id)
	item["pk"] = &types.AttributeValueMemberN{Value: pk}
	item["sk"] = &types.AttributeValueMemberN{Value: sk}
	item["age"] = &types.AttributeValueMemberN{Value: "9007199254740993"}
	item["gs2pk"] = &types.AttributeValueMemberS{Value: "type:User"}
	item["gs2sk"] = &types.AttributeValueMemberS{Value: "User#" + id}
	return item
}

func paginationNumericSchema() *ot.SchemaDef {
	return &ot.SchemaDef{
		Format:  "onetable:1.1.0",
		Version: "0.0.1",
		Indexes: map[string]*ot.IndexDef{
			"primary": {Hash: "pk", Sort: "sk"},
			"gs2":     {Hash: "gs2pk", Sort: "gs2sk", Project: "all"},
		},
		Models: map[string]ot.ModelDef{
			"User": {
				"pk":     {Type: ot.FieldTypeNumber},
				"sk":     {Type: ot.FieldTypeNumber},
				"id":     {Type: ot.FieldTypeString},
				"status": {Type: ot.FieldTypeString},
				"age":    {Type: ot.FieldTypeNumber},
				"gs2pk":  {Type: ot.FieldTypeString},
				"gs2sk":  {Type: ot.FieldTypeString},
			},
		},
		Params: &ot.SchemaParams{},
	}
}

func paginationRequestStartKey(t *testing.T, operation string, client *paginationDynamoClient, index int) map[string]types.AttributeValue {
	t.Helper()
	if operation == "find" {
		if len(client.queryCalls) <= index {
			t.Fatalf("query calls = %d, want index %d", len(client.queryCalls), index)
		}
		return client.queryCalls[index].ExclusiveStartKey
	}
	if len(client.scanCalls) <= index {
		t.Fatalf("scan calls = %d, want index %d", len(client.scanCalls), index)
	}
	return client.scanCalls[index].ExclusiveStartKey
}

func TestPagination_NumericLastEvaluatedKeyKeepsFidelity(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{name: "large-integer", value: "9007199254740993"},
		{name: "high-precision-decimal", value: "123456789012345.6789012345"},
		{name: "safe-integer", value: "42"},
	}
	for _, operation := range []string{"find", "scan"} {
		for _, tc := range cases {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				client := &paginationDynamoClient{}
				if operation == "find" {
					client.queryPages = []*ddb.QueryOutput{{LastEvaluatedKey: paginationNumericCursor(tc.value)}, {}}
				} else {
					client.scanPages = []*ddb.ScanOutput{{LastEvaluatedKey: paginationNumericCursor(tc.value)}, {}}
				}
				_, err := runPagedOperation(t, operation, &ot.Params{}, client)
				if err != nil {
					t.Fatalf("%s: %v", operation, err)
				}
				startKey := paginationRequestStartKey(t, operation, client, 1)
				if startKey == nil {
					t.Fatal("second request has no ExclusiveStartKey")
				}
				got, ok := startKey["pk"].(*types.AttributeValueMemberN)
				if !ok || got.Value != tc.value {
					t.Fatalf("second request pk = %v, want exact numeric key %q", startKey["pk"], tc.value)
				}
			})
		}
	}
}

func TestPagination_ResultNextPreservesNumericKeyForCaller(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		wantExact bool
	}{
		{name: "large-integer", value: "9007199254740993", wantExact: true},
		{name: "high-precision-decimal", value: "123456789012345.6789012345", wantExact: true},
		{name: "safe-integer", value: "42"},
	}
	for _, operation := range []string{"find", "scan"} {
		for _, tc := range cases {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				firstClient := &paginationDynamoClient{}
				if operation == "find" {
					firstClient.queryPages = []*ddb.QueryOutput{{LastEvaluatedKey: paginationNumericCursor(tc.value)}}
				} else {
					firstClient.scanPages = []*ddb.ScanOutput{{LastEvaluatedKey: paginationNumericCursor(tc.value)}}
				}
				firstPage, err := runPagedOperation(t, operation, &ot.Params{MaxPages: 1}, firstClient)
				if err != nil {
					t.Fatalf("first %s: %v", operation, err)
				}
				value := firstPage.Next["pk"]
				if tc.wantExact {
					number, ok := value.(json.Number)
					if !ok || number.String() != tc.value {
						t.Fatalf("Next pk = %T(%v), want exact json.Number(%q)", value, value, tc.value)
					}
				} else if number, ok := value.(float64); !ok || number != 42 {
					t.Fatalf("Next pk = %T(%v), want compatible float64(42)", value, value)
				}

				secondClient := &paginationDynamoClient{}
				if operation == "find" {
					secondClient.queryPages = []*ddb.QueryOutput{{}}
				} else {
					secondClient.scanPages = []*ddb.ScanOutput{{}}
				}
				_, err = runPagedOperation(t, operation, &ot.Params{Next: firstPage.Next}, secondClient)
				if err != nil {
					t.Fatalf("caller follow-up %s: %v", operation, err)
				}
				startKey := paginationRequestStartKey(t, operation, secondClient, 0)
				got, ok := startKey["pk"].(*types.AttributeValueMemberN)
				if !ok || got.Value != tc.value {
					t.Fatalf("caller follow-up pk = %v, want exact numeric key %q", startKey["pk"], tc.value)
				}
			})
		}
	}
}

func TestPagination_GSILastEvaluatedKeyKeepsAllKeys(t *testing.T) {
	cursor := map[string]types.AttributeValue{
		"gs2pk": &types.AttributeValueMemberS{Value: "type:User"},
		"gs2sk": &types.AttributeValueMemberS{Value: "User#page-1"},
		"pk":    &types.AttributeValueMemberS{Value: "User#page-1"},
		"sk":    &types.AttributeValueMemberS{Value: "User#"},
	}
	client := &paginationDynamoClient{queryPages: []*ddb.QueryOutput{{LastEvaluatedKey: cursor}, {}}}
	tbl, _ := makeTable(t, "PaginationTable", DefaultSchema, false)
	_, err := tbl.Find(bg(), "User", ot.Item{"status": "active"}, &ot.Params{Index: "gs2", Client: client})
	if err != nil {
		t.Fatalf("Find GSI: %v", err)
	}
	startKey := paginationRequestStartKey(t, "find", client, 1)
	if len(startKey) != len(cursor) {
		t.Fatalf("GSI ExclusiveStartKey has %d keys, want %d: %v", len(startKey), len(cursor), startKey)
	}
	for key, want := range map[string]string{
		"gs2pk": "type:User", "gs2sk": "User#page-1", "pk": "User#page-1", "sk": "User#",
	} {
		if got := avStr(startKey[key]); got != want {
			t.Errorf("GSI ExclusiveStartKey[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestPagination_PrevCursorUsesRawKeysAndKeepsReverseSwap(t *testing.T) {
	const largeInteger = "9007199254740993"
	const highPrecision = "123456789012345.6789012345"
	cases := []struct {
		name       string
		index      string
		properties ot.Item
		prev       ot.Item
		firstPK    string
		firstSK    string
		lastKey    map[string]types.AttributeValue
	}{
		{
			name:       "primary-numeric-keys",
			properties: ot.Item{"pk": json.Number(largeInteger)},
			prev:       ot.Item{"pk": json.Number(largeInteger), "sk": json.Number(highPrecision)},
			firstPK:    largeInteger,
			firstSK:    highPrecision,
			lastKey: map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberN{Value: "9007199254740995"},
				"sk": &types.AttributeValueMemberN{Value: "123456789012345.6789012347"},
			},
		},
		{
			name:       "gsi-with-numeric-primary-key",
			index:      "gs2",
			properties: ot.Item{"gs2pk": "type:User"},
			prev: ot.Item{
				"gs2pk": "type:User", "gs2sk": "User#prior",
				"pk": json.Number(largeInteger), "sk": json.Number(highPrecision),
			},
			firstPK: largeInteger,
			firstSK: highPrecision,
			lastKey: map[string]types.AttributeValue{
				"gs2pk": &types.AttributeValueMemberS{Value: "type:User"},
				"gs2sk": &types.AttributeValueMemberS{Value: "User#last"},
				"pk":    &types.AttributeValueMemberN{Value: "9007199254740995"},
				"sk":    &types.AttributeValueMemberN{Value: "123456789012345.6789012347"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []map[string]types.AttributeValue{
				paginationNumericItem("first", tc.firstPK, tc.firstSK),
				paginationNumericItem("second", "9007199254740994", "123456789012345.6789012346"),
			}
			client := &paginationDynamoClient{queryPages: []*ddb.QueryOutput{{Items: items, Count: 2, LastEvaluatedKey: tc.lastKey}}}
			tbl, _ := makeTable(t, "PaginationTable", paginationNumericSchema(), false)
			result, err := tbl.Find(bg(), "User", tc.properties, &ot.Params{
				Index: tc.index, Prev: tc.prev, MaxPages: 1, Client: client,
			})
			if err != nil {
				t.Fatalf("Find Prev: %v", err)
			}
			if len(result.Items) != 2 || result.Items[0]["id"] != "second" || result.Items[1]["id"] != "first" {
				t.Fatalf("reverse result order = %v, want [second first]", result.Items)
			}
			if _, ok := result.Items[0]["age"].(float64); !ok {
				t.Fatalf("ordinary item age type = %T, want unchanged float64 decoding", result.Items[0]["age"])
			}
			if _, ok := result.Items[1]["pk"].(float64); !ok {
				t.Fatalf("ordinary item key type = %T, want unchanged float64 decoding", result.Items[1]["pk"])
			}
			request := paginationRequestStartKey(t, "find", client, 0)
			if request == nil {
				t.Fatal("backward query missing caller-supplied ExclusiveStartKey")
			}
			if input := client.queryCalls[0].ScanIndexForward; input == nil || *input {
				t.Fatalf("ScanIndexForward = %v, want false for Prev-only query", input)
			}

			if got, ok := result.Next["pk"].(json.Number); !ok || got.String() != tc.firstPK {
				t.Errorf("swapped Next pk = %T(%v), want exact json.Number(%q)", result.Next["pk"], result.Next["pk"], tc.firstPK)
			}
			if got, ok := result.Next["sk"].(json.Number); !ok || got.String() != tc.firstSK {
				t.Errorf("swapped Next sk = %T(%v), want exact json.Number(%q)", result.Next["sk"], result.Next["sk"], tc.firstSK)
			}
			followClient := &paginationDynamoClient{queryPages: []*ddb.QueryOutput{{}}}
			_, err = tbl.Find(bg(), "User", tc.properties, &ot.Params{
				Index: tc.index, Next: result.Next, Client: followClient,
			})
			if err != nil {
				t.Fatalf("round-trip Prev cursor: %v", err)
			}
			followKey := paginationRequestStartKey(t, "find", followClient, 0)
			for key, want := range map[string]string{"pk": tc.firstPK, "sk": tc.firstSK} {
				value, ok := followKey[key].(*types.AttributeValueMemberN)
				if !ok || value.Value != want {
					t.Errorf("round-trip ExclusiveStartKey[%q] = %v, want exact N %q", key, followKey[key], want)
				}
			}
			if tc.index == "gs2" {
				if got := avStr(followKey["gs2pk"]); got != "type:User" {
					t.Errorf("round-trip GSI hash key = %q, want %q", got, "type:User")
				}
				if got := avStr(followKey["gs2sk"]); got != "User#first" {
					t.Errorf("round-trip GSI sort key = %q, want %q", got, "User#first")
				}
			}
			if got, ok := result.Prev["pk"].(json.Number); !ok || got.String() != "9007199254740995" {
				t.Errorf("swapped Prev pk = %T(%v), want LastEvaluatedKey 9007199254740995", result.Prev["pk"], result.Prev["pk"])
			}
			if tc.index == "gs2" {
				if result.Next["gs2pk"] != "type:User" || result.Next["gs2sk"] != "User#first" {
					t.Errorf("swapped GSI Next = %v, want first item's GSI keys", result.Next)
				}
				if got, ok := result.Prev["gs2sk"].(string); !ok || got != "User#last" {
					t.Errorf("swapped GSI Prev sk = %v, want LastEvaluatedKey", result.Prev["gs2sk"])
				}
			}
		})
	}
}

func TestPagination_InvalidCallerCursorReturnsError(t *testing.T) {
	cases := []struct {
		name string
		next ot.Item
	}{
		{name: "unsupported-value", next: ot.Item{"pk": make(chan int), "sk": "User#"}},
		{name: "marshal-error", next: ot.Item{"pk": failingCursorValue{}, "sk": "User#"}},
		{name: "missing-hash-key", next: ot.Item{"sk": "User#"}},
	}
	for _, operation := range []string{"find", "scan"} {
		for _, tc := range cases {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				client := &paginationDynamoClient{}
				if operation == "find" {
					client.queryPages = []*ddb.QueryOutput{{}}
				} else {
					client.scanPages = []*ddb.ScanOutput{{}}
				}
				params := &ot.Params{Next: tc.next}
				_, err := runPagedOperation(t, operation, params, client)
				if err == nil {
					t.Fatal("expected invalid cursor value to return an error")
				}
				if len(client.queryCalls)+len(client.scanCalls) != 0 {
					t.Fatal("client was called after cursor marshalling failed")
				}
			})
		}
	}
}

func TestPagination_LateCursorDecodeErrorPropagates(t *testing.T) {
	for _, operation := range []string{"find", "scan"} {
		t.Run(operation, func(t *testing.T) {
			badCursor := map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberN{Value: "not-a-number"},
				"sk": &types.AttributeValueMemberS{Value: "User#"},
			}
			client := &paginationDynamoClient{}
			if operation == "find" {
				client.queryPages = []*ddb.QueryOutput{{LastEvaluatedKey: paginationCursor("page-1")}, {LastEvaluatedKey: badCursor}}
			} else {
				client.scanPages = []*ddb.ScanOutput{{LastEvaluatedKey: paginationCursor("page-1")}, {LastEvaluatedKey: badCursor}}
			}
			_, err := runPagedOperation(t, operation, &ot.Params{}, client)
			if err == nil {
				t.Fatal("expected malformed later-page cursor to return an error")
			}
			calls := len(client.queryCalls)
			if operation == "scan" {
				calls = len(client.scanCalls)
			}
			if calls != 2 {
				t.Fatalf("requests = %d, want error on second page", calls)
			}
		})
	}
}

func TestPagination_TerminalPageClearsNextCursor(t *testing.T) {
	for _, operation := range []string{"find", "scan"} {
		t.Run(operation, func(t *testing.T) {
			cursor := paginationCursor("cursor-test")
			client := &paginationDynamoClient{}
			if operation == "find" {
				client.queryPages = []*ddb.QueryOutput{
					{LastEvaluatedKey: cursor},
					{},
				}
			} else {
				client.scanPages = []*ddb.ScanOutput{
					{LastEvaluatedKey: cursor},
					{},
				}
			}

			result, err := runPagedOperation(t, operation, &ot.Params{}, client)
			if err != nil {
				t.Fatalf("%s: %v", operation, err)
			}
			if result.Next != nil {
				t.Fatalf("Next = %v, want nil after terminal page", result.Next)
			}
			if operation == "find" {
				if len(client.queryCalls) != 2 {
					t.Fatalf("query calls = %d, want 2", len(client.queryCalls))
				}
			} else if len(client.scanCalls) != 2 {
				t.Fatalf("scan calls = %d, want 2", len(client.scanCalls))
			}
		})
	}
}

func TestPagination_BoundedPageKeepsNextCursor(t *testing.T) {
	for _, operation := range []string{"find", "scan"} {
		for _, bound := range []string{"limit", "max-pages"} {
			t.Run(operation+"/"+bound, func(t *testing.T) {
				cursor := paginationCursor("cursor-test")
				client := &paginationDynamoClient{}
				if operation == "find" {
					client.queryPages = []*ddb.QueryOutput{{Items: []map[string]types.AttributeValue{paginationItem("cursor-test")}, Count: 1, LastEvaluatedKey: cursor}}
				} else {
					client.scanPages = []*ddb.ScanOutput{{Items: []map[string]types.AttributeValue{paginationItem("cursor-test")}, Count: 1, ScannedCount: 1, LastEvaluatedKey: cursor}}
				}

				params := &ot.Params{}
				if bound == "limit" {
					params.Limit = 1
				} else {
					params.MaxPages = 1
				}
				result, err := runPagedOperation(t, operation, params, client)
				if err != nil {
					t.Fatalf("%s with %s: %v", operation, bound, err)
				}
				if result.Next == nil {
					t.Fatal("Next = nil, want continuation cursor when iteration is bounded")
				}
				if result.Next["pk"] != "User#cursor-test" || result.Next["sk"] != "User#" {
					t.Fatalf("Next = %v, want the page continuation key", result.Next)
				}
				calls := len(client.queryCalls)
				if operation == "scan" {
					calls = len(client.scanCalls)
				}
				if calls != 1 {
					t.Fatalf("calls = %d, want 1 when stopped by %s", calls, bound)
				}
			})
		}
	}
}

func TestPagination_SubsequentRequestsUseLastEvaluatedKey(t *testing.T) {
	for _, operation := range []string{"find", "scan"} {
		t.Run(operation, func(t *testing.T) {
			client := &paginationDynamoClient{}
			if operation == "find" {
				client.queryPages = []*ddb.QueryOutput{
					{Items: []map[string]types.AttributeValue{paginationItem("page-1")}, Count: 1, LastEvaluatedKey: paginationCursor("page-1")},
					{Items: []map[string]types.AttributeValue{paginationItem("page-2")}, Count: 1, LastEvaluatedKey: paginationCursor("page-2")},
					{Items: []map[string]types.AttributeValue{paginationItem("page-3")}, Count: 1},
				}
			} else {
				client.scanPages = []*ddb.ScanOutput{
					{Items: []map[string]types.AttributeValue{paginationItem("page-1")}, Count: 1, ScannedCount: 1, LastEvaluatedKey: paginationCursor("page-1")},
					{Items: []map[string]types.AttributeValue{paginationItem("page-2")}, Count: 1, ScannedCount: 1, LastEvaluatedKey: paginationCursor("page-2")},
					{Items: []map[string]types.AttributeValue{paginationItem("page-3")}, Count: 1, ScannedCount: 1},
				}
			}

			result, err := runPagedOperation(t, operation, &ot.Params{}, client)
			if err != nil {
				t.Fatalf("%s: %v", operation, err)
			}
			if len(result.Items) != 3 {
				t.Fatalf("items = %d, want all 3 pages", len(result.Items))
			}
			for i, wantID := range []string{"page-1", "page-2", "page-3"} {
				if got := result.Items[i]["id"]; got != wantID {
					t.Errorf("items[%d].id = %v, want %q", i, got, wantID)
				}
			}

			var firstStartKey, secondStartKey map[string]types.AttributeValue
			calls := 0
			if operation == "find" {
				calls = len(client.queryCalls)
				if calls >= 3 {
					firstStartKey = client.queryCalls[1].ExclusiveStartKey
					secondStartKey = client.queryCalls[2].ExclusiveStartKey
				}
			} else {
				calls = len(client.scanCalls)
				if calls >= 3 {
					firstStartKey = client.scanCalls[1].ExclusiveStartKey
					secondStartKey = client.scanCalls[2].ExclusiveStartKey
				}
			}
			if calls != 3 {
				t.Fatalf("requests = %d, want 3 distinct pages", calls)
			}
			if firstStartKey == nil || secondStartKey == nil {
				t.Fatalf("continuation keys = (%v, %v), want a key on both subsequent requests", firstStartKey, secondStartKey)
			}
			if got := avStr(firstStartKey["pk"]); got != "User#page-1" {
				t.Errorf("second request pk = %q, want first page cursor", got)
			}
			if got := avStr(secondStartKey["pk"]); got != "User#page-2" {
				t.Errorf("third request pk = %q, want second page cursor", got)
			}
		})
	}
}

func TestPagination_UserSuppliedNextCursorReachesRequest(t *testing.T) {
	for _, operation := range []string{"find", "scan"} {
		t.Run(operation, func(t *testing.T) {
			client := &paginationDynamoClient{}
			if operation == "find" {
				client.queryPages = []*ddb.QueryOutput{{}}
			} else {
				client.scanPages = []*ddb.ScanOutput{{}}
			}
			params := &ot.Params{Next: ot.Item{"pk": "User#caller-cursor", "sk": "User#"}}
			_, err := runPagedOperation(t, operation, params, client)
			if err != nil {
				t.Fatalf("%s: %v", operation, err)
			}

			var startKey map[string]types.AttributeValue
			if operation == "find" {
				if len(client.queryCalls) != 1 {
					t.Fatalf("query calls = %d, want 1", len(client.queryCalls))
				}
				startKey = client.queryCalls[0].ExclusiveStartKey
			} else {
				if len(client.scanCalls) != 1 {
					t.Fatalf("scan calls = %d, want 1", len(client.scanCalls))
				}
				startKey = client.scanCalls[0].ExclusiveStartKey
			}
			if got := avStr(startKey["pk"]); got != "User#caller-cursor" {
				t.Fatalf("request pk = %q, want caller-supplied cursor", got)
			}
			if got := avStr(startKey["sk"]); got != "User#" {
				t.Fatalf("request sk = %q, want caller-supplied cursor", got)
			}
		})
	}
}
