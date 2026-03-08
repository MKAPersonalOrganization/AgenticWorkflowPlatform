package pluginsettings

import (
	"time"
)

// UserPluginSettings stores per-user, per-plugin configuration.
// The settings map is serialised as JSONB — each key corresponds to a
// [toolregistry.SettingsField.Key] declared by the plugin's SettingsSchema.
type UserPluginSettings struct {
	UserID    string            `json:"user_id" db:"user_id"`
	PluginID  string            `json:"plugin_id" db:"plugin_id"`
	Settings  map[string]string `json:"settings" db:"settings"`
	CreatedAt time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt time.Time         `json:"updated_at" db:"updated_at"`
}

// MaskedSettings returns a copy of Settings with sensitive values (passwords)
// replaced by a mask that reveals only the last 4 characters.
func (s *UserPluginSettings) MaskedSettings(sensitiveKeys map[string]bool) map[string]string {
	out := make(map[string]string, len(s.Settings))
	for k, v := range s.Settings {
		if sensitiveKeys[k] && len(v) > 4 {
			out[k] = "****" + v[len(v)-4:]
		} else if sensitiveKeys[k] {
			out[k] = "****"
		} else {
			out[k] = v
		}
	}
	return out
}
