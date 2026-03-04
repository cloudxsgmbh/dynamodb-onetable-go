// Package ulid provides ULID generation/decoding helpers.
package ulid

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

const (
	timeLen   = 10
	randomLen = 16
)

const encoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID is a Universal Unique Lexicographically Sortable Identifier.
// https://github.com/ulid/spec
// Follows oklog/ulid encoding for time + entropy.
type ULID struct {
	when    time.Time
	entropy [10]byte
}

// New creates a ULID for the current time.
func New() *ULID {
	ulid := &ULID{when: time.Now()}
	ulid.fillEntropy()
	return ulid
}

// NewAt creates a ULID for the given time.
func NewAt(t time.Time) *ULID {
	ulid := &ULID{when: t}
	ulid.fillEntropy()
	return ulid
}

// String encodes the ULID as a 26-character string.
func (u *ULID) String() string {
	var id [16]byte
	u.setTime(&id)
	copy(id[6:], u.entropy[:])
	var out [26]byte
	_ = encode(out[:], id)
	return string(out[:])
}

// Decode extracts the millisecond timestamp from a ULID string.
func Decode(s string) (int64, error) {
	if len(s) != timeLen+randomLen {
		return 0, fmt.Errorf("ulid: invalid ULID length %d", len(s))
	}
	if s[0] > '7' {
		return 0, fmt.Errorf("ulid: invalid ULID overflow")
	}
	id, err := decode([]byte(s))
	if err != nil {
		return 0, err
	}
	ms := uint64(id[5]) | uint64(id[4])<<8 |
		uint64(id[3])<<16 | uint64(id[2])<<24 |
		uint64(id[1])<<32 | uint64(id[0])<<40
	return int64(ms), nil
}

func (u *ULID) fillEntropy() {
	if _, err := io.ReadFull(rand.Reader, u.entropy[:]); err != nil {
		panic("ulid: crypto/rand read failed: " + err.Error())
	}
}

func (u *ULID) setTime(id *[16]byte) {
	ms := uint64(u.when.UnixNano() / int64(time.Millisecond))
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)
}

func encode(dst []byte, id [16]byte) error {
	if len(dst) != 26 {
		return fmt.Errorf("ulid: invalid ULID buffer size %d", len(dst))
	}
	dst[0] = encoding[(id[0]&224)>>5]
	dst[1] = encoding[id[0]&31]
	dst[2] = encoding[(id[1]&248)>>3]
	dst[3] = encoding[((id[1]&7)<<2)|((id[2]&192)>>6)]
	dst[4] = encoding[(id[2]&62)>>1]
	dst[5] = encoding[((id[2]&1)<<4)|((id[3]&240)>>4)]
	dst[6] = encoding[((id[3]&15)<<1)|((id[4]&128)>>7)]
	dst[7] = encoding[(id[4]&124)>>2]
	dst[8] = encoding[((id[4]&3)<<3)|((id[5]&224)>>5)]
	dst[9] = encoding[id[5]&31]

	dst[10] = encoding[(id[6]&248)>>3]
	dst[11] = encoding[((id[6]&7)<<2)|((id[7]&192)>>6)]
	dst[12] = encoding[(id[7]&62)>>1]
	dst[13] = encoding[((id[7]&1)<<4)|((id[8]&240)>>4)]
	dst[14] = encoding[((id[8]&15)<<1)|((id[9]&128)>>7)]
	dst[15] = encoding[(id[9]&124)>>2]
	dst[16] = encoding[((id[9]&3)<<3)|((id[10]&224)>>5)]
	dst[17] = encoding[id[10]&31]
	dst[18] = encoding[(id[11]&248)>>3]
	dst[19] = encoding[((id[11]&7)<<2)|((id[12]&192)>>6)]
	dst[20] = encoding[(id[12]&62)>>1]
	dst[21] = encoding[((id[12]&1)<<4)|((id[13]&240)>>4)]
	dst[22] = encoding[((id[13]&15)<<1)|((id[14]&128)>>7)]
	dst[23] = encoding[(id[14]&124)>>2]
	dst[24] = encoding[((id[14]&3)<<3)|((id[15]&224)>>5)]
	dst[25] = encoding[id[15]&31]
	return nil
}

var dec = [256]byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x01,
	0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E,
	0x0F, 0x10, 0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14, 0x15, 0xFF,
	0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C, 0x1D, 0x1E,
	0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x0A, 0x0B, 0x0C,
	0x0D, 0x0E, 0x0F, 0x10, 0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14,
	0x15, 0xFF, 0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C,
	0x1D, 0x1E, 0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

func decode(src []byte) ([16]byte, error) {
	if len(src) != 26 {
		return [16]byte{}, fmt.Errorf("ulid: invalid ULID length %d", len(src))
	}
	if src[0] > '7' {
		return [16]byte{}, fmt.Errorf("ulid: invalid ULID overflow")
	}
	for i := 0; i < len(src); i++ {
		if dec[src[i]] == 0xFF {
			return [16]byte{}, fmt.Errorf("ulid: invalid ULID char %q", src[i])
		}
	}
	var id [16]byte
	id[0] = (dec[src[0]] << 5) | dec[src[1]]
	id[1] = (dec[src[2]] << 3) | (dec[src[3]] >> 2)
	id[2] = (dec[src[3]] << 6) | (dec[src[4]] << 1) | (dec[src[5]] >> 4)
	id[3] = (dec[src[5]] << 4) | (dec[src[6]] >> 1)
	id[4] = (dec[src[6]] << 7) | (dec[src[7]] << 2) | (dec[src[8]] >> 3)
	id[5] = (dec[src[8]] << 5) | dec[src[9]]

	id[6] = (dec[src[10]] << 3) | (dec[src[11]] >> 2)
	id[7] = (dec[src[11]] << 6) | (dec[src[12]] << 1) | (dec[src[13]] >> 4)
	id[8] = (dec[src[13]] << 4) | (dec[src[14]] >> 1)
	id[9] = (dec[src[14]] << 7) | (dec[src[15]] << 2) | (dec[src[16]] >> 3)
	id[10] = (dec[src[16]] << 5) | dec[src[17]]
	id[11] = (dec[src[18]] << 3) | dec[src[19]]>>2
	id[12] = (dec[src[19]] << 6) | (dec[src[20]] << 1) | (dec[src[21]] >> 4)
	id[13] = (dec[src[21]] << 4) | (dec[src[22]] >> 1)
	id[14] = (dec[src[22]] << 7) | (dec[src[23]] << 2) | (dec[src[24]] >> 3)
	id[15] = (dec[src[24]] << 5) | dec[src[25]]
	return id, nil
}
