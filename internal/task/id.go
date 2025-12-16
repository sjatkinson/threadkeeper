package task

import (
	"crypto/rand"
	"encoding/base32"
	"time"
)

// GenerateID generates a durable, time-sortable ID (ULID-like using base32).
// It combines a timestamp (6 bytes) with random bytes (10 bytes) and encodes in base32.
func GenerateID() (string, error) {
	// Get timestamp in milliseconds
	timestampMs := time.Now().UTC().UnixMilli()
	tsBytes := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		tsBytes[i] = byte(timestampMs & 0xff)
		timestampMs >>= 8
	}

	// Generate random bytes
	rndBytes := make([]byte, 10)
	if _, err := rand.Read(rndBytes); err != nil {
		return "", err
	}

	// Concatenate and encode
	raw := append(tsBytes, rndBytes...)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	return encoded, nil
}

