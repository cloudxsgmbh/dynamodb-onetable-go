package uid

import (
	"strings"
	"testing"
)

func TestUID_LengthAndAlphabet(t *testing.T) {
	s := UID(64)
	if len(s) != 64 {
		t.Fatalf("len=%d want=64", len(s))
	}
	for i := range s {
		if !strings.ContainsRune(letters, rune(s[i])) {
			t.Fatalf("invalid char %q at %d", s[i], i)
		}
	}
}

func TestUUID_FormatVersionVariant(t *testing.T) {
	u := UUID()
	if len(u) != 36 {
		t.Fatalf("len=%d want=36", len(u))
	}
	if u[8] != '-' || u[13] != '-' || u[18] != '-' || u[23] != '-' {
		t.Fatalf("bad hyphen positions: %q", u)
	}
	if u[14] != '4' {
		t.Fatalf("bad version nibble %q want '4'", u[14])
	}
	if !strings.ContainsRune("89abAB", rune(u[19])) {
		t.Fatalf("bad variant nibble %q", u[19])
	}
}
