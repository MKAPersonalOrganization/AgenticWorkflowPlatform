package domain

import (
	"school-with-ai/modules/preferences"
)

// ──────────────────────────────────────────────────────────────────────────────
// Backward-compatible aliases — canonical types now live in modules/preferences.
// These aliases exist so that existing consumers importing domain.UserPreference
// continue to compile. New code should import modules/preferences directly.
// ──────────────────────────────────────────────────────────────────────────────

// UserPreference is an alias for preferences.UserPreference.
type UserPreference = preferences.UserPreference

// UserPreferenceRepository is an alias for preferences.UserPreferenceRepository.
type UserPreferenceRepository = preferences.UserPreferenceRepository

// PrefSelectedAgent is the well-known preference key for selected agent.
const PrefSelectedAgent = preferences.PrefSelectedAgent
