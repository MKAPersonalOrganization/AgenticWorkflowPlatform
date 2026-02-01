package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateID generates a unique identifier.
func GenerateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// This should never happen, but if it does, panic is appropriate
		// since we can't generate a secure random ID
		panic(fmt.Sprintf("failed to generate random ID: %s", err))
	}

	return hex.EncodeToString(b)
}
