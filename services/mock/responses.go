package mock

import (
	"fmt"
)

// Simple mock response generator for Phase 1
// This will be enhanced in later phases

// GenerateEchoResponse generates a simple echo response
func GenerateEchoResponse(userMessage string) string {
	return fmt.Sprintf("Echo: %s", userMessage)
}
