package pluginsettings

import (
	"aicompass/platform/toolregistry"
	"context"
	"fmt"
)

// Service provides business logic for per-user plugin settings.
type Service struct {
	repo Repository
}

// NewService creates a new plugin settings service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetForUserPlugin returns the settings for a specific user + plugin pair.
// Returns nil (no error) if no settings have been saved yet.
func (s *Service) GetForUserPlugin(ctx context.Context, userID, pluginID string) (*UserPluginSettings, error) {
	return s.repo.GetByUserPlugin(ctx, userID, pluginID)
}

// GetAllForUser returns all plugin settings for a user.
func (s *Service) GetAllForUser(ctx context.Context, userID string) ([]UserPluginSettings, error) {
	return s.repo.GetAllByUser(ctx, userID)
}

// GetAllForUserAsMap returns all plugin settings for a user as a map
// keyed by plugin ID → setting key → value.
// This is the format expected by [toolregistry.FactoryDeps.PluginSettings].
func (s *Service) GetAllForUserAsMap(ctx context.Context, userID string) (map[string]map[string]string, error) {
	all, err := s.repo.GetAllByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]map[string]string, len(all))
	for _, ps := range all {
		out[ps.PluginID] = ps.Settings
	}
	return out, nil
}

// Save validates and persists plugin settings for a user.
// Validation: required fields must be present and non-empty;
// unknown keys (not in the plugin's schema) are silently dropped.
func (s *Service) Save(ctx context.Context, userID, pluginID string, values map[string]string) error {
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if pluginID == "" {
		return fmt.Errorf("plugin_id is required")
	}

	// Look up the plugin's schema for validation.
	provider, ok := toolregistry.GetPluginProvider(pluginID)
	if !ok {
		return fmt.Errorf("unknown plugin: %s", pluginID)
	}

	schema := provider.SettingsSchema()
	allowedKeys := make(map[string]toolregistry.SettingsField, len(schema))
	for _, f := range schema {
		allowedKeys[f.Key] = f
	}

	// Load existing settings once (needed to preserve untouched fields).
	existing, _ := s.repo.GetByUserPlugin(ctx, userID, pluginID)

	// Filter to only known keys and validate required fields.
	clean := make(map[string]string, len(allowedKeys))
	for _, f := range schema {
		v, provided := values[f.Key]

		if !provided || v == "" {
			// Value not supplied — try to preserve the existing stored value.
			if existing != nil && existing.Settings[f.Key] != "" {
				clean[f.Key] = existing.Settings[f.Key]
				continue
			}
			if f.Required {
				return fmt.Errorf("required setting %q is missing", f.Key)
			}
			continue
		}

		clean[f.Key] = v
	}

	return s.repo.Upsert(ctx, &UserPluginSettings{
		UserID:   userID,
		PluginID: pluginID,
		Settings: clean,
	})
}

// Delete removes plugin settings for a user + plugin pair.
func (s *Service) Delete(ctx context.Context, userID, pluginID string) error {
	return s.repo.Delete(ctx, userID, pluginID)
}

// SensitiveKeysForPlugin returns the set of setting keys that are marked as
// password/sensitive for the given plugin. Used for masking in GET responses.
func SensitiveKeysForPlugin(pluginID string) map[string]bool {
	provider, ok := toolregistry.GetPluginProvider(pluginID)
	if !ok {
		return nil
	}
	out := make(map[string]bool)
	for _, f := range provider.SettingsSchema() {
		if f.Type == toolregistry.FieldTypePassword {
			out[f.Key] = true
		}
	}
	return out
}
