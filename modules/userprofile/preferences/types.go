package preferences

import (
	"context"
	"time"
)

// UserPreference represents a single user preference (key-value pair).
// Used for cross-device persistence of UI state such as selected agent.
type UserPreference struct {
	UserID    string    `json:"user_id" db:"user_id"`
	Key       string    `json:"key" db:"key"`
	Value     string    `json:"value" db:"value"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Well-known preference keys.
const (
	PrefSelectedAgent = "selected_agent"
)

// UserPreferenceRepository defines the storage contract for user preferences.
type UserPreferenceRepository interface {
	// Get retrieves a single preference by user ID and key.
	// Returns nil, nil if not found.
	Get(ctx context.Context, userID, key string) (*UserPreference, error)

	// GetAll retrieves all preferences for a user.
	GetAll(ctx context.Context, userID string) ([]UserPreference, error)

	// Set creates or updates a single preference.
	Set(ctx context.Context, pref *UserPreference) error
}
