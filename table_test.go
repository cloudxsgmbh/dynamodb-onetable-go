package onetable

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestBuildGetInput(t *testing.T) {
	cmd := Item{
		"TableName": "t",
		"Key":       Item{"pk": "P#1", "sk": "S#1"},
	}
	in, err := buildGetInput(cmd)
	if err != nil {
		t.Fatalf("buildGetInput err: %v", err)
	}
	if _, ok := in.Key["pk"].(*types.AttributeValueMemberS); !ok {
		t.Fatalf("pk not string av: %T", in.Key["pk"])
	}
}

func TestBuildPutInput(t *testing.T) {
	cmd := Item{"TableName": "t", "Item": Item{"pk": "P#1", "n": 7}}
	in, err := buildPutInput(cmd)
	if err != nil {
		t.Fatalf("buildPutInput err: %v", err)
	}
	if _, ok := in.Item["pk"].(*types.AttributeValueMemberS); !ok {
		t.Fatalf("pk not string av: %T", in.Item["pk"])
	}
	if _, ok := in.Item["n"].(*types.AttributeValueMemberN); !ok {
		t.Fatalf("n not number av: %T", in.Item["n"])
	}
}

func TestBuildDeleteInput(t *testing.T) {
	cmd := Item{"TableName": "t", "Key": Item{"pk": "P#1", "sk": "S#1"}}
	in, err := buildDeleteInput(cmd)
	if err != nil {
		t.Fatalf("buildDeleteInput err: %v", err)
	}
	if in.Key == nil || in.Key["pk"] == nil {
		t.Fatal("missing key")
	}
}

func TestBuildUpdateInput(t *testing.T) {
	cmd := Item{
		"TableName":                 "t",
		"Key":                       Item{"pk": "P#1", "sk": "S#1"},
		"UpdateExpression":          "set #n = :n",
		"ExpressionAttributeNames":  map[string]string{"#n": "name"},
		"ExpressionAttributeValues": map[string]types.AttributeValue{":n": &types.AttributeValueMemberS{Value: "x"}},
	}
	in, err := buildUpdateInput(cmd)
	if err != nil {
		t.Fatalf("buildUpdateInput err: %v", err)
	}
	if in.UpdateExpression == nil || *in.UpdateExpression == "" {
		t.Fatal("missing update expression")
	}
}

func TestBuildQueryInput(t *testing.T) {
	cmd := Item{
		"TableName":              "t",
		"KeyConditionExpression": "pk = :pk",
		"Limit":                  5,
		"ScanIndexForward":       true,
	}
	in, err := buildQueryInput(cmd)
	if err != nil {
		t.Fatalf("buildQueryInput err: %v", err)
	}
	if in.Limit == nil || *in.Limit != 5 {
		t.Fatalf("bad limit: %v", in.Limit)
	}
}

func TestBuildScanInput(t *testing.T) {
	cmd := Item{"TableName": "t", "Limit": 7, "Segment": 1, "TotalSegments": 4}
	in, err := buildScanInput(cmd)
	if err != nil {
		t.Fatalf("buildScanInput err: %v", err)
	}
	if in.Segment == nil || *in.Segment != 1 {
		t.Fatalf("bad segment: %v", in.Segment)
	}
}

func TestBuildTransactWriteInput(t *testing.T) {
	cmd := Item{
		"TransactItems": []any{
			map[string]any{"Put": Item{"TableName": "t", "Item": Item{"pk": "P#1", "sk": "S#1"}}},
			map[string]any{"Update": Item{"TableName": "t", "Key": Item{"pk": "P#1", "sk": "S#1"}, "UpdateExpression": "set #a=:a"}},
			map[string]any{"Delete": Item{"TableName": "t", "Key": Item{"pk": "P#1", "sk": "S#1"}}},
			map[string]any{"ConditionCheck": Item{
				"TableName":           "t",
				"Key":                 map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "P#1"}},
				"ConditionExpression": "attribute_exists(pk)",
			}},
		},
	}
	in, err := buildTransactWriteInput(cmd)
	if err != nil {
		t.Fatalf("buildTransactWriteInput err: %v", err)
	}
	if len(in.TransactItems) != 4 {
		t.Fatalf("expected 4 transact items, got %d", len(in.TransactItems))
	}
}

func TestBuildTransactGetInput(t *testing.T) {
	cmd := Item{
		"TransactItems": []any{
			map[string]any{"Get": Item{"TableName": "t", "Key": Item{"pk": "P#1", "sk": "S#1"}}},
		},
	}
	in, err := buildTransactGetInput(cmd)
	if err != nil {
		t.Fatalf("buildTransactGetInput err: %v", err)
	}
	if len(in.TransactItems) != 1 || in.TransactItems[0].Get == nil {
		t.Fatal("missing transact get")
	}
}

func TestBuildBatchGetInput(t *testing.T) {
	cmd := Item{
		"RequestItems": map[string]any{
			"t": map[string]any{
				"Keys": []any{map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "P#1"}}},
			},
		},
	}
	in, err := buildBatchGetInput(cmd)
	if err != nil {
		t.Fatalf("buildBatchGetInput err: %v", err)
	}
	if len(in.RequestItems["t"].Keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(in.RequestItems["t"].Keys))
	}
}

func TestBuildBatchWriteInput(t *testing.T) {
	cmd := Item{
		"RequestItems": map[string]any{
			"t": []any{
				map[string]any{"PutRequest": Item{"TableName": "t", "Item": Item{"pk": "P#1", "sk": "S#1"}}},
				map[string]any{"DeleteRequest": Item{"TableName": "t", "Key": Item{"pk": "P#1", "sk": "S#1"}}},
			},
		},
	}
	in, err := buildBatchWriteInput(cmd)
	if err != nil {
		t.Fatalf("buildBatchWriteInput err: %v", err)
	}
	if len(in.RequestItems["t"]) != 2 {
		t.Fatalf("expected 2 write requests, got %d", len(in.RequestItems["t"]))
	}
}
