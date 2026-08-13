// Package ids provides UUIDv7 generation for sortable, portable identifiers.
package ids

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

// NewV7 generates a UUIDv7 (time-ordered UUID).
// Format: 48-bit unix ms | 4-bit version(7) | 12-bit rand | 2-bit variant | 62-bit rand
func NewV7() string {
	var uuid [16]byte

	// 48-bit Unix timestamp in milliseconds
	ms := uint64(time.Now().UnixMilli())
	uuid[0] = byte(ms >> 40)
	uuid[1] = byte(ms >> 32)
	uuid[2] = byte(ms >> 24)
	uuid[3] = byte(ms >> 16)
	uuid[4] = byte(ms >> 8)
	uuid[5] = byte(ms)

	// Random bytes for the rest
	rand.Read(uuid[6:])

	// Version 7
	uuid[6] = (uuid[6] & 0x0f) | 0x70
	// Variant bits (RFC 4122)
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(uuid[0:4]),
		hex.EncodeToString(uuid[4:6]),
		hex.EncodeToString(uuid[6:8]),
		hex.EncodeToString(uuid[8:10]),
		hex.EncodeToString(uuid[10:16]),
	)
}

// NewV7Bytes returns the raw 16-byte UUIDv7.
func NewV7Bytes() [16]byte {
	var uuid [16]byte
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint64(uuid[:8], ms<<16)
	rand.Read(uuid[6:])
	uuid[6] = (uuid[6] & 0x0f) | 0x70
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return uuid
}
