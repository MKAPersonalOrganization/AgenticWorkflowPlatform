package chatcontracts

import "context"

// ToolRequirementConfigService abstracts the tool requirements module that the
// chat module requires for per-user, per-agent, per-tool requirement
// configuration (allow/block/require_confirmation, etc.).
//
// The concrete *toolreqs.Service satisfies this interface implicitly.
// This is the single authority for all tool access decisions.
type ToolRequirementConfigService interface {
	// GetToolRequirementConfigJSON returns the requirement config JSON for a tool.
	// Returns "{}" when no explicit config exists.
	GetToolRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID, toolID string) (string, error)

	// SetToolRequirementConfigJSON saves the requirement config JSON for a tool.
	SetToolRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID, toolID string, configJSON string) error

	// GetToolSetRequirementConfigJSON returns the requirement config JSON for a tool set.
	GetToolSetRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID string) (string, error)

	// SetToolSetRequirementConfigJSON saves the requirement config JSON for a tool set.
	SetToolSetRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID string, configJSON string) error

	// ListToolRequirementConfigsJSON returns all tool requirement configs for
	// a tool set as a map of toolID → config JSON string.
	ListToolRequirementConfigsJSON(ctx context.Context, userID, agentID, toolSetID string) (map[string]string, error)
}

// UserProfile represents the extended profile of a user.
// This is a local DTO mirroring identity.UserProfile so that the chat
// module does not import modules/userprofile/identity directly.
type UserProfile struct {
	UserID        string   `json:"user_id"`
	LearningStyle string   `json:"learning_style"`
	Struggles     []string `json:"struggles"`
	Strengths     []string `json:"strengths"`
	Goals         string   `json:"goals"`
}

// UserProfileRepository reads user profiles.
// The concrete identity.UserProfileRepository satisfies this interface
// with a thin adapter that maps identity.UserProfile → chatcontracts.UserProfile.
type UserProfileRepository interface {
	GetByUserID(ctx context.Context, userID string) (*UserProfile, error)
}
