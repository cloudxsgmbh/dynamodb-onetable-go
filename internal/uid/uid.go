/*
Package uid – UUID, ULID and UID generators.

Mirrors JS: UID.js – crypto-grade IDs using Crockford base-32.
*/
package uid

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

// Crockford base-32 alphabet (excludes I, L, O, U).
// The last character 'Z' is repeated so that rand==0xFF maps cleanly.
const letters = "0123456789ABCDEFGHJKMNPQRSTVWXYZZ"

const lettersLen = len(letters) - 1 // 32

// UID generates a crypto-random string of the given length using base-32 encoding.
// Size >= 10 is suitably unique for most use-cases.
func UID(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic("uid: crypto/rand read failed: " + err.Error())
	}
	out := make([]byte, size)
	for i := 0; i < size; i++ {
		idx := int(math.Floor(float64(buf[i]) / 0xff * float64(lettersLen)))
		out[i] = letters[idx]
	}
	return string(out)
}

// UUID returns a simple (non-crypto) RFC-4122 v4 UUID string.
func UUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("uid: crypto/rand read failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ULID is a Universal Unique Lexicographically Sortable Identifier.
// https://github.com/ulid/spec
type ULID struct {
	when time.Time
}

const (
	timeLen   = 10
	randomLen = 16
)

// New creates a ULID for the current time.
func New() *ULID { return &ULID{when: time.Now()} }

// NewAt creates a ULID for the given time.
func NewAt(t time.Time) *ULID { return &ULID{when: t} }

// String encodes the ULID as a 26-character string.
func (u *ULID) String() string {
	return u.encodeTime() + u.encodeRandom()
}

func (u *ULID) encodeTime() string {
	ms := u.when.UnixMilli()
	b := make([]byte, timeLen)
	for i := timeLen - 1; i >= 0; i-- {
		b[i] = letters[ms%int64(lettersLen)]
		ms /= int64(lettersLen)
	}
	return string(b)
}

func (u *ULID) encodeRandom() string {
	buf := make([]byte, randomLen)
	if _, err := rand.Read(buf); err != nil {
		panic("uid: crypto/rand read failed: " + err.Error())
	}
	out := make([]byte, randomLen)
	for i := 0; i < randomLen; i++ {
		idx := int(math.Floor(float64(buf[i]) / 0xff * float64(lettersLen)))
		out[i] = letters[idx]
	}
	return string(out)
}

// Decode extracts the millisecond timestamp from a ULID string.
func Decode(s string) (int64, error) {
	if len(s) != timeLen+randomLen {
		return 0, fmt.Errorf("uid: invalid ULID length %d", len(s))
	}
	var ms int64
	for _, c := range []byte(s[:timeLen]) {
		idx := strings.IndexByte(letters, c)
		if idx < 0 {
			return 0, fmt.Errorf("uid: invalid ULID char %q", c)
		}
		ms = ms*int64(lettersLen) + int64(idx)
	}
	return ms, nil
}

// randUint64 is a helper for UUID (not exported).
func randUint64() uint64 {
	var b [8]byte
	rand.Read(b[:]) //nolint:errcheck
	return binary.LittleEndian.Uint64(b[:])
}
