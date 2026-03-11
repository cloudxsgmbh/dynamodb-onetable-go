package onetable

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestBuildGetInputNormalizesStringMapKey(t *testing.T) {
	cmd := Item{
		"TableName": "t",
		"Key": Item{
			"pk": "S#1",
			"sk": "S#session",
		},
	}
	in, err := buildGetInput(cmd)
	if err != nil {
		t.Fatalf("buildGetInput err: %v", err)
	}
	if in.Key == nil {
		t.Fatalf("expected key")
	}
	if _, ok := in.Key["pk"].(*types.AttributeValueMemberS); !ok {
		t.Fatalf("pk not S av: %T", in.Key["pk"])
	}
	if _, ok := in.Key["sk"].(*types.AttributeValueMemberS); !ok {
		t.Fatalf("sk not S av: %T", in.Key["sk"])
	}
}
