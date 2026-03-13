package agent

// State scope prefixes for different visibility levels.
// These mirror ADK's session state scoping conventions.
const (
	// StateScopeApp is the prefix for app-level state (shared across all users/sessions).
	StateScopeApp = "app:"

	// StateScopeUser is the prefix for user-level state (shared across sessions for a user).
	StateScopeUser = "user:"

	// StateScopeTemp is the prefix for temporary state (discarded after invocation).
	StateScopeTemp = "temp:"

	// No prefix means session-level state (specific to this session).
)
