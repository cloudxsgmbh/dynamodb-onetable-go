package onetable

import (
	"testing"
	"time"

	"github.com/cloudxsgmbh/dynamodb-onetable-go/internal/ulid"
)

func TestTableULIDWithTimestamp(t *testing.T) {
	table := &Table{}
	for _, ms := range []int64{0, 1469922850259} {
		id := table.ULID(time.UnixMilli(ms))
		got, err := ulid.Decode(id)
		if err != nil {
			t.Fatalf("Decode(%q): %v", id, err)
		}
		if got != ms {
			t.Errorf("timestamp = %d, want %d", got, ms)
		}
	}
}

func TestTableULIDDefaultsToCurrentTime(t *testing.T) {
	table := &Table{}
	before := time.Now().UnixMilli()
	id := table.ULID()
	after := time.Now().UnixMilli()
	got, err := ulid.Decode(id)
	if err != nil {
		t.Fatalf("Decode(%q): %v", id, err)
	}
	if got < before || got > after {
		t.Errorf("timestamp = %d, want between %d and %d", got, before, after)
	}
}
