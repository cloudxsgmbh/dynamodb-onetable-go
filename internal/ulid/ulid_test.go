package ulid

import (
	"strings"
	"testing"
	"time"
)

func TestNew_StringLengthAndAlphabet(t *testing.T) {
	u := New().String()
	if len(u) != 26 {
		t.Fatalf("len=%d want=26", len(u))
	}
	for i := range u {
		if !strings.ContainsRune(encoding, rune(u[i])) {
			t.Fatalf("invalid char %q at %d", u[i], i)
		}
	}
}

func TestDecode_RoundTripTimestamp(t *testing.T) {
	at := time.UnixMilli(1710000000123).UTC()
	u := NewAt(at).String()
	ms, err := Decode(u)
	if err != nil {
		t.Fatalf("Decode err: %v", err)
	}
	if ms != at.UnixMilli() {
		t.Fatalf("ms=%d want=%d", ms, at.UnixMilli())
	}
}

func TestDecode_InvalidLength(t *testing.T) {
	if _, err := Decode("short"); err == nil {
		t.Fatal("expected error for invalid length")
	}
}

func TestDecode_InvalidChar(t *testing.T) {
	u := New().String()
	bad := "I" + u[1:] // I not in Crockford set
	if _, err := Decode(bad); err == nil {
		t.Fatal("expected error for invalid char")
	}
}

func TestDecode_Overflow(t *testing.T) {
	u := New().String()
	overflow := "8" + u[1:] // first char must be <= '7'
	if _, err := Decode(overflow); err == nil {
		t.Fatal("expected overflow error")
	}
}
