/*
Package uid – UUID and UID generators.

Mirrors JS: UID.js – crypto-grade IDs using Crockford base-32.
*/
package uid

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"math"
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

// UUID returns an RFC-4122 v4 UUID string.
func UUID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		panic("uid: crypto/rand read failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}
