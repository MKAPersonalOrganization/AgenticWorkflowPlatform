package core

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateID generates a random hexadecimal ID.
func GenerateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return hex.EncodeToString([]byte(string(rune(len(bytes)))))
	}

	return hex.EncodeToString(bytes)
}
