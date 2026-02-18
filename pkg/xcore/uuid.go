package xcore

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"sync"
	"time"
)

// IDGenerator provides unique ID generation
type IDGenerator interface {
	// Generate generates a new unique ID
	Generate() string
	// GenerateWithPrefix generates a new ID with a prefix
	GenerateWithPrefix(prefix string) string
}

// ULIDGenerator generates ULID-compatible IDs
// ULID: Universally Unique Lexicographically Sortable Identifier
// Format: 26-character string like "01ARZ3NDEKTSV4RRFFQ69G5FAV"
// - 48-bit timestamp (seconds since Unix epoch)
// - 80-bit random data
type ULIDGenerator struct {
	mu       sync.Mutex
	lastTime uint64
	random   []byte
}

// NewIDGenerator creates a new ID generator
func NewIDGenerator() *ULIDGenerator {
	return &ULIDGenerator{
		random: make([]byte, 10),
	}
}

// Generate generates a new ULID-compatible ID
func (g *ULIDGenerator) Generate() string {
	return g.GenerateWithPrefix("")
}

// GenerateWithPrefix generates a new ULID with an optional prefix
func (g *ULIDGenerator) GenerateWithPrefix(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Get current time in milliseconds
	now := uint64(time.Now().UnixNano() / 1000000)

	// Ensure monotonicity
	if now <= g.lastTime {
		now = g.lastTime + 1
	}
	g.lastTime = now

	// Generate random data
	if _, err := rand.Read(g.random); err != nil {
		// Fallback to time-based if random fails
		g.random = []byte(fmt.Sprintf("%010d", time.Now().UnixNano()%10000000000))
	}

	// Encode timestamp (48 bits = 10 base32 chars)
	// ULID uses big-endian encoding
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)

	// Convert timestamp to bytes (8 bytes, but we only need 6 for 48 bits)
	timeBytes := []byte{
		byte(now >> 40),
		byte(now >> 32),
		byte(now >> 24),
		byte(now >> 16),
		byte(now >> 8),
		byte(now),
	}

	// Encode timestamp to base32 (10 characters)
	timeEncoded := make([]byte, 10)
	encoding.Encode(timeEncoded, timeBytes)

	// Encode random data (10 bytes = 16 base32 chars)
	randomEncoded := make([]byte, 16)
	encoding.Encode(randomEncoded, g.random)

	// Combine
	id := string(timeEncoded) + string(randomEncoded)

	if prefix != "" {
		return prefix + "_" + id
	}
	return id
}

// GenerateUUID generates a UUID v4 string
// Format: "f47ac10b-58cc-4372-a567-0e02b2c3d479"
func GenerateUUID() string {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		// Fallback to time-based
		return fmt.Sprintf("uuid-%d", time.Now().UnixNano())
	}

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

// GenerateShortID generates a short unique ID (8 chars)
// Format: "aB3xK9mN"
func GenerateShortID() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("id%d", time.Now().UnixNano()%1000000)
	}

	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	return encoding.EncodeToString(bytes)[:8]
}

// GenerateRequestID generates a request ID for tracing
// Combines timestamp and random data for uniqueness and readability
// Format: "req_01ARZ3NDEKTSV4RRFFQ69G5FAV"
func GenerateRequestID() string {
	gen := NewIDGenerator()
	return "req_" + gen.Generate()
}

// GenerateID generates a unique ID with optional prefix
// This is a convenience function using the default generator
func GenerateID(prefix ...string) string {
	gen := NewIDGenerator()
	if len(prefix) > 0 && prefix[0] != "" {
		return gen.GenerateWithPrefix(prefix[0])
	}
	return gen.Generate()
}

// ParseULIDTime extracts the timestamp from a ULID
func ParseULIDTime(id string) (time.Time, error) {
	if len(id) < 10 {
		return time.Time{}, fmt.Errorf("ID too short")
	}

	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	timeEncoded := []byte(id[:10])
	timeBytes := make([]byte, 6)
	_, err := encoding.Decode(timeBytes, timeEncoded)
	if err != nil {
		return time.Time{}, err
	}

	// Reconstruct timestamp
	var ms uint64
	for i := 0; i < 6; i++ {
		ms = (ms << 8) | uint64(timeBytes[i])
	}

	return time.Unix(0, int64(ms)*1000000), nil
}

// IsULID checks if a string is a valid ULID format
func IsULID(id string) bool {
	if len(id) != 26 {
		return false
	}

	// Check if all characters are valid base32
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	_, err := encoding.DecodeString(id)
	return err == nil
}
