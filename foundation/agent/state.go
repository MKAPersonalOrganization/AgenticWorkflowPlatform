package agent

import "iter"

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

// ReadonlyState provides read access to the session state visible during an invocation.
//
// Mirrors ADK's session.ReadonlyState — exposed on Invocation so that
// InstructionFunc and Requirements can read accumulated state (e.g. output
// from a prior agent stored under an OutputKey) without being able to mutate it.
//
// Key scoping follows the StateScopeApp / StateScopeUser / StateScopeTemp
// prefix conventions above. Session-scoped keys have no prefix.
type ReadonlyState interface {
	// Get returns the value for the given key.
	// The second return value is false if the key is not present.
	Get(key string) (any, bool)

	// All iterates over all key-value pairs in the state.
	All() iter.Seq2[string, any]
}
