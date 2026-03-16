package chatcontracts

import "context"

// ToolPermissionService abstracts the tool requirements module that the chat
// module requires for tool-set and individual-tool toggles.
//
// The concrete *toolreqs.Service satisfies this interface implicitly.
// This is the single authority for all tool access decisions.
type ToolPermissionService interface {
	// GetEnabledToolSets returns enabled tool-set IDs for the given user/agent.
	GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error)

	// HasPermission checks whether a specific tool is enabled.
	HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool

	// SetToolSetPermission enables or disables a tool set for an agent.
	SetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string, enabled bool) error

	// SetPermission enables or disables an individual tool within a tool set.
	SetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string, enabled bool) error
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
