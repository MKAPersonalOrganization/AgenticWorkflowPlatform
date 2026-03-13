package agent

import "context"

// UserContext holds contextual information about the current user that
// agents may use to personalize their behavior. The fields are intentionally
// generic — domain-specific profiles are mapped into this structure by
// adapter-layer code.
type UserContext struct {
	UserID     string
	Attributes map[string]string   // Flat key-value attributes (e.g., "role": "admin")
	Tags       map[string][]string // Multi-value tags (e.g., "interests": ["math", "science"])
	Goals      string              // Free-text goal description
}

// UserContextProvider abstracts access to user context so that
// foundation-layer middleware does not import domain-specific modules.
// The concrete implementation lives in the adapter/composition layer.
type UserContextProvider interface {
	GetUserContext(ctx context.Context, userID string) (*UserContext, error)
}
