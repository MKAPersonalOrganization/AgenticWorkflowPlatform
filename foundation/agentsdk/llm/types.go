package llm

import "fmt"

// Metadata holds arbitrary key-value pairs.
type Metadata map[string]any

// GetString returns the value for a key as a string, or empty string if not found/wrong type.
func (m Metadata) GetString(key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// Role represents the role of a message participant in a conversation.
// This mirrors Microsoft.Extensions.AI.ChatRole from the .NET agent framework.
type Role string

const (
	// RoleUser represents a message from the user/human.
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Validate checks if the role is a valid message role.
func (r Role) Validate() error {
	switch r {
	case RoleUser, RoleAssistant, RoleSystem, RoleTool:
		return nil
	default:
		return fmt.Errorf("invalid role: %s", r)
	}
}

// ErrorKind categorizes an LLM error.
type ErrorKind string

const (
	KindInvalidInput ErrorKind = "invalid_input"
	KindNotFound     ErrorKind = "not_found"
	KindRateLimit    ErrorKind = "rate_limit"
	KindInternal     ErrorKind = "internal"
)

// Error represents a structured error from an LLM operation.
// This aligns with the Universal Agent Interface Go Specification.
type Error struct {
	Op      string         // Operation that failed
	Kind    ErrorKind      // Category of error
	Err     error          // Underlying error
	Context map[string]any // Additional context
}

func (e *Error) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}
